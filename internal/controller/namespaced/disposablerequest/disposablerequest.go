/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package disposablerequest

import (
	"context"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/namespaced/disposablerequest/v1alpha2"
	apisv1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/disposablerequest"
	"github.com/crossplane-contrib/provider-http/internal/utils"
)

const (
	errNotNamespacedDisposableRequest      = "managed resource is not a namespaced DisposableRequest custom resource"
	errTrackPCUsage                        = "cannot track ProviderConfig usage"
	errNewHttpClient                       = "cannot create new Http client"
	errFailedToSendHttpDisposableRequest   = "failed to send http request"
	errExtractCredentials                  = "cannot extract credentials"
	errResponseDoesntMatchExpectedCriteria = "response does not match expected criteria"

	errGetPC  = "cannot get ProviderConfig"
	errGetCPC = "cannot get ClusterProviderConfig"
)

// errorConditionReconciler wraps a reconciler to add ErrorObserved condition after reconciliation
type errorConditionReconciler struct {
	reconciler reconcile.Reconciler
	kube       client.Client
}

// Reconcile implements reconcile.Reconciler
func (r *errorConditionReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// Call the managed reconciler
	result, err := r.reconciler.Reconcile(ctx, req)

	// After reconciliation, check if we need to add ErrorObserved condition
	cr := &v1alpha2.DisposableRequest{}
	if getErr := r.kube.Get(ctx, req.NamespacedName, cr); getErr != nil {
		// If we can't get the resource, just return the original result
		return result, err
	}

	if cr.Status.Error != "" && utils.RetriesLimitReached(cr.Status.Failed, cr.Spec.ForProvider.RollbackRetriesLimit) {
		// Ensure the ErrorObserved condition is present and current
		if ensureErr := r.ensureErrorObserved(ctx, cr); ensureErr != nil {
			// Log but don't fail the reconciliation
			return result, err
		}
	}

	return result, err
}

// ensureErrorObserved sets or updates the ErrorObserved condition to the current error message
func (r *errorConditionReconciler) ensureErrorObserved(ctx context.Context, cr *v1alpha2.DisposableRequest) error {
	// Check if ErrorObserved condition already exists and is current
	hasCurrentErrorCondition := false
	for _, c := range cr.Status.Conditions {
		if c.Type == "ErrorObserved" && c.Status == corev1.ConditionTrue && c.Message == cr.Status.Error {
			hasCurrentErrorCondition = true
			break
		}
	}

	if hasCurrentErrorCondition {
		return nil
	}

	// Update conditions
	conditions := cr.Status.Conditions
	foundIndex := -1
	for i, c := range conditions {
		if c.Type == "ErrorObserved" {
			foundIndex = i
			break
		}
	}

	errorCondition := xpv1.Condition{
		Type:               "ErrorObserved",
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "RetriesExhausted",
		Message:            cr.Status.Error,
	}

	if foundIndex >= 0 {
		conditions[foundIndex] = errorCondition
	} else {
		conditions = append(conditions, errorCondition)
	}

	cr.SetConditions(conditions...)

	// Update the resource status
	if updateErr := r.kube.Status().Update(ctx, cr); updateErr != nil {
		return updateErr
	}

	return nil
}

// Setup adds a controller that reconciles namespaced DisposableRequest managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha2.DisposableRequestGroupKind)

	reconcilerOptions := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{
			logger:          o.Logger,
			kube:            mgr.GetClient(),
			usage:           resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha2.ProviderConfigUsage{}),
			newHttpClientFn: httpClient.NewClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		WithCustomPollIntervalHook(),
		managed.WithTimeout(timeout),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		reconcilerOptions = append(reconcilerOptions, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.DisposableRequestGroupVersionKind),
		reconcilerOptions...,
	)

	// Wrap the reconciler to add ErrorObserved condition after managed reconciler completes
	wrappedReconciler := &errorConditionReconciler{
		reconciler: ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter),
		kube:       mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.DisposableRequest{}).
		Complete(wrappedReconciler)
}

type connector struct {
	logger          logging.Logger
	kube            client.Client
	usage           *resource.ProviderConfigUsageTracker
	newHttpClientFn func(log logging.Logger, timeout time.Duration, creds string) (httpClient.Client, error)
}

// Connect returns a new ExternalClient.
//
//gocyclo:ignore
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return nil, errors.New(errNotNamespacedDisposableRequest)
	}

	l := c.logger.WithValues("namespacedDisposableRequest", cr.Name)

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	// Set default providerConfigRef if not specified
	if cr.GetProviderConfigReference() == nil {
		cr.SetProviderConfigReference(&xpv1.ProviderConfigReference{
			Name: "default",
			Kind: "ClusterProviderConfig",
		})
		l.Debug("No providerConfigRef specified, defaulting to 'default'")
	}

	var cd apisv1alpha2.ProviderCredentials
	var providerTLS *common.TLSConfig

	// Switch to ModernManaged resource to get ProviderConfigRef
	m := mg.(resource.ModernManaged)
	ref := m.GetProviderConfigReference()

	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha2.ProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: m.GetNamespace()}, pc); err != nil {
			return nil, errors.Wrap(err, errGetPC)
		}
		cd = pc.Spec.Credentials
		providerTLS = pc.Spec.TLS
	case "ClusterProviderConfig":
		cpc := &apisv1alpha2.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, errors.Wrap(err, errGetCPC)
		}
		cd = cpc.Spec.Credentials
		providerTLS = cpc.Spec.TLS
	default:
		return nil, errors.Errorf("unsupported provider config kind: %s", ref.Kind)
	}

	creds := ""
	if cd.Source == xpv1.CredentialsSourceSecret {
		data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, errExtractCredentials)
		}
		creds = string(data)
	}

	h, err := c.newHttpClientFn(l, utils.WaitTimeout(cr.Spec.ForProvider.WaitTimeout), creds)
	if err != nil {
		return nil, errors.Wrap(err, errNewHttpClient)
	}

	// Merge TLS configs: resource-level overrides provider-level
	mergedTLSConfig := httpClient.MergeTLSConfigs(cr.Spec.ForProvider.TLSConfig, providerTLS)

	// Apply InsecureSkipTLSVerify from DisposableRequest spec if set
	if cr.Spec.ForProvider.InsecureSkipTLSVerify {
		if mergedTLSConfig == nil {
			mergedTLSConfig = &common.TLSConfig{}
		}
		mergedTLSConfig.InsecureSkipVerify = true
	}

	// Load TLS configuration from secrets
	tlsConfigData, err := httpClient.LoadTLSConfig(ctx, c.kube, mergedTLSConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load TLS configuration")
	}

	return &external{
		localKube:     c.kube,
		logger:        l,
		http:          h,
		tlsConfigData: tlsConfigData,
	}, nil
}

type external struct {
	localKube     client.Client
	logger        logging.Logger
	http          httpClient.Client
	tlsConfigData *httpClient.TLSConfigData
}

// Observe checks the state of the DisposableRequest resource and updates its status accordingly.
//
//gocyclo:ignore
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNamespacedDisposableRequest)
	}

	if meta.WasDeleted(mg) {
		c.logger.Debug("DisposableRequest is being deleted, skipping observation and secret injection")
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Check if retries are needed (error occurred and haven't exhausted retries)
	needsRetry := utils.ShouldRetry(cr.Spec.ForProvider.RollbackRetriesLimit, cr.Status.Failed) && !utils.RetriesLimitReached(cr.Status.Failed, cr.Spec.ForProvider.RollbackRetriesLimit)

	// For retries, respect nextReconcile timing if configured
	isUpToDate := true
	if needsRetry {
		if cr.Spec.ForProvider.NextReconcile != nil {
			// Only retry if enough time has passed since last reconcile
			nextReconcileDuration := cr.Spec.ForProvider.NextReconcile.Duration
			last := cr.Status.LastReconcileTime.Time
			if last.IsZero() {
				last = time.Now()
			}
			if !time.Now().Before(last.Add(nextReconcileDuration)) {
				c.logger.Debug("NextReconcile time reached and retry needed, marking resource as not up-to-date")
				isUpToDate = false
			} else {
				c.logger.Debug("Retry needed but nextReconcile time not yet reached, keeping resource up-to-date")
			}
		} else {
			// No nextReconcile configured, allow immediate retry
			isUpToDate = false
		}
	}

	isAvailable := isUpToDate

	// If the resource is not yet marked as synced, we would normally trigger
	// a Create (or Update) which causes an immediate deployment. However,
	// when a retry is pending (an error occurred and rollback retries are
	// enabled) and the configured NextReconcile time has not yet been reached
	// we should avoid triggering an immediate deployment and instead treat
	// the resource as existing and up-to-date until NextReconcile elapses.
	if !cr.Status.Synced {
		if needsRetry && isUpToDate {
			c.logger.Debug("Retry pending and nextReconcile not reached; suppressing Create to respect NextReconcile timing")
			return managed.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: true,
			}, nil
		}

		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	svcCtx := service.NewServiceContext(ctx, c.localKube, c.logger, c.http, c.tlsConfigData)
	crCtx := service.NewDisposableRequestCRContext(cr)
	isExpected, storedResponse, err := disposablerequest.ValidateStoredResponse(svcCtx, crCtx)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if !isExpected {
		c.logger.Debug("Response does not match expected criteria")
		// Respect nextReconcile timing even for validation failures
		if cr.Spec.ForProvider.NextReconcile != nil {
			nextReconcileDuration := cr.Spec.ForProvider.NextReconcile.Duration
			last := cr.Status.LastReconcileTime.Time
			if last.IsZero() {
				last = time.Now()
			}
			if time.Now().Before(last.Add(nextReconcileDuration)) {
				c.logger.Debug("Validation failed but nextReconcile time not yet reached, keeping resource up-to-date")
				return managed.ExternalObservation{
					ResourceExists:   isAvailable,
					ResourceUpToDate: true,
				}, nil
			}
		}
		c.logger.Debug("Validation failed and nextReconcile time reached (or not configured), marking resource as not up-to-date")
		return managed.ExternalObservation{
			ResourceExists:   isAvailable,
			ResourceUpToDate: false,
		}, nil
	}

	isUpToDate = disposablerequest.CalculateUpToDateStatus(crCtx, isUpToDate)

	// If nextReconcile is configured and no retry is pending, check if regular reconcile time has passed
	if !needsRetry && cr.Spec.ForProvider.NextReconcile != nil {
		nextReconcileDuration := cr.Spec.ForProvider.NextReconcile.Duration
		last := cr.Status.LastReconcileTime.Time
		if last.IsZero() {
			// If last reconcile time isn't set yet, consider now to avoid
			// triggering an immediate extra reconcile.
			last = time.Now()
		}
		if !time.Now().Before(last.Add(nextReconcileDuration)) {
			c.logger.Debug("NextReconcile time reached, marking resource as not up-to-date to force deployment")
			isUpToDate = false
		}
	}

	if isAvailable {
		if err := disposablerequest.UpdateResourceStatus(ctx, cr, c.localKube); err != nil {
			return managed.ExternalObservation{}, err
		}
	}

	if len(cr.Spec.ForProvider.SecretInjectionConfigs) > 0 && cr.Status.Response.StatusCode != 0 {
		disposablerequest.ApplySecretInjectionsFromStoredResponse(svcCtx, crCtx, storedResponse)
	}

	return managed.ExternalObservation{
		ResourceExists:    isAvailable,
		ResourceUpToDate:  isUpToDate,
		ConnectionDetails: nil,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNamespacedDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalCreation{}, err
	}

	svcCtx := service.NewServiceContext(ctx, c.localKube, c.logger, c.http, c.tlsConfigData)
	crCtx := service.NewDisposableRequestCRContext(cr)
	return managed.ExternalCreation{}, errors.Wrap(disposablerequest.DeployAction(svcCtx, crCtx), errFailedToSendHttpDisposableRequest)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotNamespacedDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalUpdate{}, err
	}

	svcCtx := service.NewServiceContext(ctx, c.localKube, c.logger, c.http, c.tlsConfigData)
	crCtx := service.NewDisposableRequestCRContext(cr)
	return managed.ExternalUpdate{}, errors.Wrap(disposablerequest.DeployAction(svcCtx, crCtx), errFailedToSendHttpDisposableRequest)
}

func (c *external) Delete(_ context.Context, _ resource.Managed) (managed.ExternalDelete, error) {
	return managed.ExternalDelete{}, nil
}

// Disconnect does nothing. It never returns an error.
func (c *external) Disconnect(_ context.Context) error {
	return nil
}

// WithCustomPollIntervalHook returns a managed.ReconcilerOption that sets a custom poll interval based on the DisposableRequest spec.
func WithCustomPollIntervalHook() managed.ReconcilerOption {
	return managed.WithPollIntervalHook(customPollIntervalHook)
}

// customPollIntervalHook computes the duration until the next reconcile based on the
// DisposableRequest's spec and status. If LastReconcileTime is zero (not yet observed),
// treat it as now to avoid premature short-interval requeues.
func customPollIntervalHook(mg resource.Managed, _ time.Duration) time.Duration {
	defaultPollInterval := 30 * time.Second

	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return defaultPollInterval
	}

	if cr.Spec.ForProvider.NextReconcile == nil {
		return defaultPollInterval
	}

	// Calculate next reconcile time based on NextReconcile duration
	nextReconcileDuration := cr.Spec.ForProvider.NextReconcile.Duration
	lastReconcileTime := cr.Status.LastReconcileTime.Time
	if lastReconcileTime.IsZero() {
		// Status update may not have propagated yet; consider last reconcile as now.
		lastReconcileTime = time.Now()
	}
	nextReconcileTime := lastReconcileTime.Add(nextReconcileDuration)

	// Determine if the current time is past the next reconcile time
	now := time.Now()
	if now.Before(nextReconcileTime) {
		// If not yet time to reconcile, calculate remaining time
		return nextReconcileTime.Sub(now)
	}

	// Default poll interval if the next reconcile time is in the past
	return defaultPollInterval
}
