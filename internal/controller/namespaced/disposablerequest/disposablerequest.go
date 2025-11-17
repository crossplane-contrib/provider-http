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
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/disposablerequest/v1alpha2"
	apisv1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/utils"
)

const (
	errNotNamespacedDisposableRequest      = "managed resource is not a namespaced DisposableRequest custom resource"
	errTrackPCUsage                        = "cannot track ProviderConfig usage"
	errNewHttpClient                       = "cannot create new Http client"
	errProviderNotRetrieved                = "provider could not be retrieved"
	errFailedToSendHttpDisposableRequest   = "failed to send http request"
	errFailedUpdateStatusConditions        = "failed updating status conditions"
	ErrExpectedFormat                      = "JQ filter should return a boolean, but returned error: %s"
	errPatchFromReferencedSecret           = "cannot patch from referenced secret"
	errGetReferencedSecret                 = "cannot get referenced secret"
	errCreateReferencedSecret              = "cannot create referenced secret"
	errPatchDataToSecret                   = "Warning, couldn't patch data from request to secret %s:%s:%s, error: %s"
	errConvertResToMap                     = "failed to convert response to map"
	errGetLatestVersion                    = "failed to get the latest version of the resource"
	errResponseFormat                      = "Response does not match the expected format, retries limit "
	errExtractCredentials                  = "cannot extract credentials"
	errCheckExpectedResponse               = "failed to check if response is as expected"
	errResponseDoesntMatchExpectedCriteria = "response does not match expected criteria"

	errGetPC    = "cannot get ProviderConfig"
	errGetCPC   = "cannot get ClusterProviderConfig"
	errGetCreds = "cannot get credentials"
)

// Setup adds a controller that reconciles namespaced DisposableRequest managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha2.DisposableRequestGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.DisposableRequestGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			logger:          o.Logger,
			kube:            mgr.GetClient(),
			newHttpClientFn: httpClient.NewClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		WithCustomPollIntervalHook(),
		managed.WithTimeout(timeout),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

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
	newHttpClientFn func(log logging.Logger, timeout time.Duration, creds string) (httpClient.Client, error)
}

// Connect returns a new ExternalClient.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return nil, errors.New(errNotNamespacedDisposableRequest)
	}

	l := c.logger.WithValues("namespacedDisposableRequest", cr.Name)

	// Set default providerConfigRef if not specified
	if cr.GetProviderConfigReference() == nil {
		cr.SetProviderConfigReference(&xpv1.ProviderConfigReference{
			Name: "default",
			Kind: "ClusterProviderConfig",
		})
		l.Debug("No providerConfigRef specified, defaulting to 'default'")
	}

	var cd apisv1alpha2.ProviderCredentials

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
	case "ClusterProviderConfig":
		cpc := &apisv1alpha2.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, errors.Wrap(err, errGetCPC)
		}
		cd = cpc.Spec.Credentials
	default:
		return nil, errors.Errorf("unsupported provider config kind: %s", ref.Kind)
	}

	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errExtractCredentials)
	}

	h, err := c.newHttpClientFn(l, utils.WaitTimeout(cr.Spec.ForProvider.WaitTimeout), string(data))
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
		return managed.ExternalObservation{}, errors.New(errNotNamespacedDisposableRequest)
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

	isExpected, storedResponse, err := c.validateStoredResponse(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if !isExpected {
		return managed.ExternalObservation{}, errors.New(errResponseDoesntMatchExpectedCriteria)
	}

	isUpToDate = c.calculateUpToDateStatus(cr, isUpToDate)

	if isAvailable {
		if err := c.updateResourceStatus(ctx, cr); err != nil {
			return managed.ExternalObservation{}, err
		}
	}

	if len(cr.Spec.ForProvider.SecretInjectionConfigs) > 0 && cr.Status.Response.StatusCode != 0 {
		c.applySecretInjectionsFromStoredResponse(ctx, cr, storedResponse, isExpected)
	}

	return managed.ExternalObservation{
		ResourceExists:    isAvailable,
		ResourceUpToDate:  isUpToDate,
		ConnectionDetails: nil,
	}, nil
}

// validateStoredResponse validates the stored response against expected criteria
func (c *external) validateStoredResponse(ctx context.Context, cr *v1alpha2.DisposableRequest) (bool, httpClient.HttpResponse, error) {
	sensitiveBody, err := datapatcher.PatchSecretsIntoString(ctx, c.localKube, cr.Status.Response.Body, c.logger)
	if err != nil {
		return false, httpClient.HttpResponse{}, errors.Wrap(err, errPatchFromReferencedSecret)
	}

	storedResponse := httpClient.HttpResponse{
		StatusCode: cr.Status.Response.StatusCode,
		Headers:    cr.Status.Response.Headers,
		Body:       sensitiveBody,
	}

	isExpected, err := c.isResponseAsExpected(cr, storedResponse)
	if err != nil {
		c.logger.Debug("Setting error condition due to validation error", "error", err)
		return false, httpClient.HttpResponse{}, errors.Wrap(err, errCheckExpectedResponse)
	}
	if !isExpected {
		c.logger.Debug("Response does not match expected criteria")
		return false, httpClient.HttpResponse{}, nil
	}

	return true, storedResponse, nil
}

// calculateUpToDateStatus determines if the resource should be considered up-to-date
func (c *external) calculateUpToDateStatus(cr *v1alpha2.DisposableRequest, currentStatus bool) bool {
	// If shouldLoopInfinitely is true, the resource should never be considered up-to-date
	if cr.Spec.ForProvider.ShouldLoopInfinitely {
		if cr.Spec.ForProvider.RollbackRetriesLimit == nil {
			return false
		}
	}
	return currentStatus
}

// updateResourceStatus updates the resource status to Available
func (c *external) updateResourceStatus(ctx context.Context, cr *v1alpha2.DisposableRequest) error {
	if err := c.localKube.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
		return errors.Wrap(err, errGetLatestVersion)
	}

	cr.Status.SetConditions(xpv1.Available())
	if err := c.localKube.Status().Update(ctx, cr); err != nil {
		return errors.New(errFailedUpdateStatusConditions)
	}
	return nil
}

// deployAction sends the HTTP request defined in the DisposableRequest resource and updates its status based on the response.
func (c *external) deployAction(ctx context.Context, cr *v1alpha2.DisposableRequest) error {
	if cr.Status.Synced {
		c.logger.Debug("Resource is already synced, skipping deployment action")
		return nil
	}

	// Check if retries limit has been reached
	if utils.RollBackEnabled(cr.Spec.ForProvider.RollbackRetriesLimit) && utils.RetriesLimitReached(cr.Status.Failed, cr.Spec.ForProvider.RollbackRetriesLimit) {
		c.logger.Debug("Retries limit reached, not retrying anymore")
		return nil
	}

	details, httpRequestErr := c.sendHttpRequest(ctx, cr)

	resource, err := c.prepareRequestResource(ctx, cr, details)
	if err != nil {
		return err
	}

	// Handle HTTP request errors first
	if httpRequestErr != nil {
		return c.handleHttpRequestError(ctx, cr, resource, httpRequestErr)
	}

	return c.handleHttpResponse(ctx, cr, details.HttpResponse, resource)
}

// applySecretInjectionsFromStoredResponse applies secret injection configurations using the stored response
// This is used when the resource is already synced but secret injection configs may have been updated
func (c *external) applySecretInjectionsFromStoredResponse(ctx context.Context, cr *v1alpha2.DisposableRequest, storedResponse httpClient.HttpResponse, isExpectedResponse bool) {
	if isExpectedResponse {
		c.logger.Debug("Applying secret injections from stored response")
		datapatcher.ApplyResponseDataToSecrets(ctx, c.localKube, c.logger, &storedResponse, cr.Spec.ForProvider.SecretInjectionConfigs, cr)
		return
	}

	c.logger.Debug("Skipping secret injections as response does not match expected criteria")
}

// sendHttpRequest sends the HTTP request with sensitive data patched
func (c *external) sendHttpRequest(ctx context.Context, cr *v1alpha2.DisposableRequest) (httpClient.HttpDetails, error) {
	sensitiveBody, err := datapatcher.PatchSecretsIntoString(ctx, c.localKube, cr.Spec.ForProvider.Body, c.logger)
	if err != nil {
		return httpClient.HttpDetails{}, err
	}

	sensitiveHeaders, err := datapatcher.PatchSecretsIntoHeaders(ctx, c.localKube, cr.Spec.ForProvider.Headers, c.logger)
	if err != nil {
		return httpClient.HttpDetails{}, err
	}

	bodyData := httpClient.Data{Encrypted: cr.Spec.ForProvider.Body, Decrypted: sensitiveBody}
	headersData := httpClient.Data{Encrypted: cr.Spec.ForProvider.Headers, Decrypted: sensitiveHeaders}
	details, err := c.http.SendRequest(ctx, cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL, bodyData, headersData, cr.Spec.ForProvider.InsecureSkipTLSVerify)

	return details, err
}

// prepareRequestResource creates and initializes the RequestResource
func (c *external) prepareRequestResource(ctx context.Context, cr *v1alpha2.DisposableRequest, details httpClient.HttpDetails) (*utils.RequestResource, error) {
	// Get the latest version of the resource before updating
	if err := c.localKube.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
		return nil, errors.Wrap(err, errGetLatestVersion)
	}

	resource := &utils.RequestResource{
		StatusWriter:   cr, // DisposableRequest implements interfaces.BaseStatusWriter
		Resource:       cr,
		RequestContext: ctx,
		HttpResponse:   details.HttpResponse,
		LocalClient:    c.localKube,
		HttpRequest:    details.HttpRequest,
	}

	return resource, nil
}

// handleHttpResponse processes the HTTP response and updates resource status accordingly
func (c *external) handleHttpResponse(ctx context.Context, cr *v1alpha2.DisposableRequest, sensitiveResponse httpClient.HttpResponse, resource *utils.RequestResource) error {
	// Handle HTTP error status codes
	if utils.IsHTTPError(resource.HttpResponse.StatusCode) {
		return c.handleHttpErrorStatus(ctx, cr, resource)
	}

	// Handle response validation
	return c.handleResponseValidation(ctx, cr, sensitiveResponse, resource)
}

// handleHttpRequestError handles cases where the HTTP request itself failed
func (c *external) handleHttpRequestError(ctx context.Context, cr *v1alpha2.DisposableRequest, resource *utils.RequestResource, httpRequestErr error) error {
	setErr := resource.SetError(httpRequestErr)
	c.applySecretInjectionsFromStoredResponse(ctx, cr, resource.HttpResponse, false)
	if settingError := utils.SetRequestResourceStatus(*resource, setErr, resource.SetLastReconcileTime(), resource.SetRequestDetails()); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}
	return httpRequestErr
}

// handleHttpErrorStatus handles HTTP error status codes
func (c *external) handleHttpErrorStatus(ctx context.Context, cr *v1alpha2.DisposableRequest, resource *utils.RequestResource) error {
	c.applySecretInjectionsFromStoredResponse(ctx, cr, resource.HttpResponse, false)
	if settingError := utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(), resource.SetRequestDetails(), resource.SetError(nil)); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return errors.Errorf(utils.ErrStatusCode, cr.Spec.ForProvider.Method, strconv.Itoa(resource.HttpResponse.StatusCode))
}

// handleResponseValidation validates the response and updates status accordingly
func (c *external) handleResponseValidation(ctx context.Context, cr *v1alpha2.DisposableRequest, sensitiveResponse httpClient.HttpResponse, resource *utils.RequestResource) error {
	isExpectedResponse, err := c.isResponseAsExpected(cr, sensitiveResponse)
	if err != nil {
		return err
	}

	if isExpectedResponse {
		c.applySecretInjectionsFromStoredResponse(ctx, cr, resource.HttpResponse, true)
		return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(), resource.SetSynced(), resource.SetRequestDetails())
	}

	limit := utils.GetRollbackRetriesLimit(cr.Spec.ForProvider.RollbackRetriesLimit)
	return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(),
		resource.SetError(errors.New(errResponseFormat+fmt.Sprint(limit))), resource.SetRequestDetails())
}

func (c *external) isResponseAsExpected(cr *v1alpha2.DisposableRequest, res httpClient.HttpResponse) (bool, error) {
	// If no expected response is defined, consider it as expected.
	if cr.Spec.ForProvider.ExpectedResponse == "" {
		return true, nil
	}

	if res.StatusCode == 0 {
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
		return managed.ExternalCreation{}, errors.New(errNotNamespacedDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{}, errors.Wrap(c.deployAction(ctx, cr), errFailedToSendHttpDisposableRequest)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha2.DisposableRequest)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotNamespacedDisposableRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, errors.Wrap(c.deployAction(ctx, cr), errFailedToSendHttpDisposableRequest)
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
