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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha2"
	apisv1alpha1 "github.com/crossplane-contrib/provider-http/apis/cluster/v1alpha1"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/disposablerequest"
	"github.com/crossplane-contrib/provider-http/internal/utils"
)

const (
	errNotDisposableRequest                = "managed resource is not a DisposableRequest custom resource"
	errTrackPCUsage                        = "cannot track ProviderConfig usage"
	errNewHttpClient                       = "cannot create new Http client"
	errProviderNotRetrieved                = "provider could not be retrieved"
	errFailedToSendHttpDisposableRequest   = "failed to send http request"
	errExtractCredentials                  = "cannot extract credentials"
	errResponseDoesntMatchExpectedCriteria = "response does not match expected criteria"
)

// Setup adds a controller that reconciles DisposableRequest managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha2.DisposableRequestGroupKind)

	reconcilerOptions := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{
			logger:          o.Logger,
			kube:            mgr.GetClient(),
			usage:           resource.NewLegacyProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
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

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.DisposableRequest{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	logger          logging.Logger
	kube            client.Client
	usage           *resource.LegacyProviderConfigUsageTracker
	newHttpClientFn func(log logging.Logger, timeout time.Duration, creds string) (httpClient.Client, error)
}

// Connect returns a new ExternalClient.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return nil, errors.New(errNotDisposableRequest)
	}

	l := c.logger.WithValues("disposableRequest", cr.Name)

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	// Set default providerConfigRef if not specified
	if cr.GetProviderConfigReference() == nil {
		cr.SetProviderConfigReference(&xpv1.Reference{
			Name: "default",
		})
		l.Debug("No providerConfigRef specified, defaulting to 'default'")
	}

	pc := &apisv1alpha1.ProviderConfig{}
	n := types.NamespacedName{Name: cr.GetProviderConfigReference().Name}
	if err := c.kube.Get(ctx, n, pc); err != nil {
		return nil, errors.Wrap(err, errProviderNotRetrieved)
	}

	creds := ""
	if pc.Spec.Credentials.Source == xpv1.CredentialsSourceSecret {
		data, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Credentials.Source, c.kube, pc.Spec.Credentials.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, errExtractCredentials)
		}

		creds = string(data)
	}

	h, err := c.newHttpClientFn(l, utils.WaitTimeout(cr.Spec.ForProvider.WaitTimeout), creds)
	if err != nil {
		return nil, errors.Wrap(err, errNewHttpClient)
	}

	return &external{
		localKube: c.kube,
		logger:    l,
		http:      h,
	}, nil
}

type external struct {
	localKube client.Client
	logger    logging.Logger
	http      httpClient.Client
}

// Observe checks the state of the DisposableRequest resource and updates its status accordingly.
//
//gocyclo:ignore
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDisposableRequest)
	}

	if meta.WasDeleted(mg) {
		c.logger.Debug("DisposableRequest is being deleted, skipping observation and secret injection")
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	isUpToDate := !(utils.ShouldRetry(cr.Spec.ForProvider.RollbackRetriesLimit, cr.Status.Failed) && !utils.RetriesLimitReached(cr.Status.Failed, cr.Spec.ForProvider.RollbackRetriesLimit))
	isAvailable := isUpToDate

	if !cr.Status.Synced {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	svcCtx := service.NewServiceContext(ctx, c.localKube, c.logger, c.http)
	crCtx := service.NewDisposableRequestCRContext(cr)
	isExpected, storedResponse, err := disposablerequest.ValidateStoredResponse(svcCtx, crCtx)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if !isExpected {
		return managed.ExternalObservation{}, errors.New(errResponseDoesntMatchExpectedCriteria)
	}

	isUpToDate = disposablerequest.CalculateUpToDateStatus(crCtx, isUpToDate)

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
		return managed.ExternalCreation{}, errors.New(errNotDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalCreation{}, err
	}

	svcCtx := service.NewServiceContext(ctx, c.localKube, c.logger, c.http)
	crCtx := service.NewDisposableRequestCRContext(cr)
	return managed.ExternalCreation{}, errors.Wrap(disposablerequest.DeployAction(svcCtx, crCtx), errFailedToSendHttpDisposableRequest)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalUpdate{}, err
	}

	svcCtx := service.NewServiceContext(ctx, c.localKube, c.logger, c.http)
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
	return managed.WithPollIntervalHook(func(mg resource.Managed, pollInterval time.Duration) time.Duration {
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
		nextReconcileTime := lastReconcileTime.Add(nextReconcileDuration)

		// Determine if the current time is past the next reconcile time
		now := time.Now()
		if now.Before(nextReconcileTime) {
			// If not yet time to reconcile, calculate remaining time
			return nextReconcileTime.Sub(now)
		}

		// Default poll interval if the next reconcile time is in the past
		return defaultPollInterval
	})
}
