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
	"fmt"
	"strconv"
	"time"

	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane-contrib/provider-http/apis/disposablerequest/v1alpha2"
	apisv1alpha1 "github.com/crossplane-contrib/provider-http/apis/v1alpha1"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	errNotDisposableRequest              = "managed resource is not a DisposableRequest custom resource"
	errTrackPCUsage                      = "cannot track ProviderConfig usage"
	errNewHttpClient                     = "cannot create new Http client"
	errProviderNotRetrieved              = "provider could not be retrieved"
	errFailedToSendHttpDisposableRequest = "failed to send http request"
	errFailedUpdateStatusConditions      = "failed updating status conditions"
	ErrExpectedFormat                    = "JQ filter should return a boolean, but returned error: %s"
	errPatchFromReferencedSecret         = "cannot patch from referenced secret"
	errGetReferencedSecret               = "cannot get referenced secret"
	errCreateReferencedSecret            = "cannot create referenced secret"
	errPatchDataToSecret                 = "Warning, couldn't patch data from request to secret %s:%s:%s, error: %s"
	errConvertResToMap                   = "failed to convert response to map"
	errGetLatestVersion                  = "failed to get the latest version of the resource"
	errResponseFormat                    = "Response does not match the expected format, retries limit "
)

// Setup adds a controller that reconciles DisposableRequest managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha2.DisposableRequestGroupKind)
	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.DisposableRequestGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			logger:          o.Logger,
			kube:            mgr.GetClient(),
			usage:           resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newHttpClientFn: httpClient.NewClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		WithCustomPollIntervalHook(),
		managed.WithTimeout(timeout),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

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
	usage           resource.Tracker
	newHttpClientFn func(log logging.Logger, timeout time.Duration) (httpClient.Client, error)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return nil, errors.New(errNotDisposableRequest)
	}

	l := c.logger.WithValues("disposableRequest", cr.Name)

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	n := types.NamespacedName{Name: cr.GetProviderConfigReference().Name}
	if err := c.kube.Get(ctx, n, pc); err != nil {
		return nil, errors.Wrap(err, errProviderNotRetrieved)
	}

	h, err := c.newHttpClientFn(l, utils.WaitTimeout(cr.Spec.ForProvider.WaitTimeout))
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

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDisposableRequest)
	}

	if !cr.Status.Synced {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Get the latest version of the resource before updating
	if err := c.localKube.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetLatestVersion)
	}

	cr.Status.SetConditions(xpv1.Available())
	if err := c.localKube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalObservation{}, errors.New(errFailedUpdateStatusConditions)
	}

	isUpToDate := !(utils.ShouldRetry(cr.Spec.ForProvider.RollbackRetriesLimit, cr.Status.Failed) && !utils.RetriesLimitReached(cr.Status.Failed, cr.Spec.ForProvider.RollbackRetriesLimit))

	// If shouldLoopInfinitely is true, the resource should never be considered up-to-date
	if cr.Spec.ForProvider.ShouldLoopInfinitely {
		if cr.Spec.ForProvider.RollbackRetriesLimit == nil {
			isUpToDate = false
		}
	}

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  isUpToDate,
		ConnectionDetails: nil,
	}, nil
}

func (c *external) deployAction(ctx context.Context, cr *v1alpha2.DisposableRequest) error {
	sensitiveBody, err := datapatcher.PatchSecretsIntoBody(ctx, c.localKube, cr.Spec.ForProvider.Body, c.logger)
	if err != nil {
		return err
	}

	sensitiveHeaders, err := datapatcher.PatchSecretsIntoHeaders(ctx, c.localKube, cr.Spec.ForProvider.Headers, c.logger)
	if err != nil {
		return err
	}

	bodyData := httpClient.Data{Encrypted: cr.Spec.ForProvider.Body, Decrypted: sensitiveBody}
	headersData := httpClient.Data{Encrypted: cr.Spec.ForProvider.Headers, Decrypted: sensitiveHeaders}
	details, err := c.http.SendRequest(ctx, cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL, bodyData, headersData, cr.Spec.ForProvider.InsecureSkipTLSVerify)

	sensitiveResponse := details.HttpResponse
	resource := &utils.RequestResource{
		Resource:       cr,
		RequestContext: ctx,
		HttpResponse:   details.HttpResponse,
		LocalClient:    c.localKube,
		HttpRequest:    details.HttpRequest,
	}

	c.patchResponseToSecret(ctx, cr, &resource.HttpResponse)

	// Get the latest version of the resource before updating
	if err := c.localKube.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
		return errors.Wrap(err, errGetLatestVersion)
	}

	if err != nil {
		setErr := resource.SetError(err)
		if settingError := utils.SetRequestResourceStatus(*resource, setErr, resource.SetLastReconcileTime(), resource.SetRequestDetails()); settingError != nil {
			return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
		}
		return err
	}

	if utils.IsHTTPError(resource.HttpResponse.StatusCode) {
		if settingError := utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(), resource.SetRequestDetails(), resource.SetError(nil)); settingError != nil {
			return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
		}

		return errors.Errorf(utils.ErrStatusCode, cr.Spec.ForProvider.Method, strconv.Itoa(resource.HttpResponse.StatusCode))
	}

	isExpectedResponse, err := c.isResponseAsExpected(cr, sensitiveResponse)
	if err != nil {
		return err
	}

	if !isExpectedResponse {
		limit := utils.GetRollbackRetriesLimit(cr.Spec.ForProvider.RollbackRetriesLimit)
		return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(),
			resource.SetError(errors.New(errResponseFormat+fmt.Sprint(limit))), resource.SetRequestDetails())
	}

	return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(), resource.SetSynced(), resource.SetRequestDetails())
}

func (c *external) isResponseAsExpected(cr *v1alpha2.DisposableRequest, res httpClient.HttpResponse) (bool, error) {
	// If no expected response is defined, consider it as expected.
	if cr.Spec.ForProvider.ExpectedResponse == "" {
		return true, nil
	}

	if cr.Status.Response.StatusCode == 0 {
		return false, nil
	}

	responseMap, err := json_util.StructToMap(res)
	if err != nil {
		return false, errors.Wrap(err, errConvertResToMap)
	}

	json_util.ConvertJSONStringsToMaps(&responseMap)

	isExpected, err := jq.ParseBool(cr.Spec.ForProvider.ExpectedResponse, responseMap)
	if err != nil {
		return false, errors.Errorf(ErrExpectedFormat, err.Error())
	}

	return isExpected, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{}, errors.Wrap(c.deployAction(ctx, cr), errFailedToSendHttpDisposableRequest)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, errors.Wrap(c.deployAction(ctx, cr), errFailedToSendHttpDisposableRequest)
}

func (c *external) Delete(_ context.Context, _ resource.Managed) error {
	return nil
}

func (c *external) patchResponseToSecret(ctx context.Context, cr *v1alpha2.DisposableRequest, response *httpClient.HttpResponse) {
	for _, ref := range cr.Spec.ForProvider.SecretInjectionConfigs {
		var owner metav1.Object = nil

		if ref.SetOwnerReference {
			owner = cr
		}

		err := datapatcher.PatchResponseToSecret(ctx, c.localKube, c.logger, response, ref.ResponsePath, ref.SecretKey, ref.SecretRef.Name, ref.SecretRef.Namespace, owner)
		if err != nil {
			c.logger.Info(fmt.Sprintf(errPatchDataToSecret, ref.SecretRef.Name, ref.SecretRef.Namespace, ref.SecretKey, err.Error()))
		}
	}
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
