/*
Copyright 2024 The Crossplane Authors.

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

package request

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	apisv1alpha1 "github.com/crossplane-contrib/provider-http/apis/v1alpha1"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/statushandler"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	errNotRequest                   = "managed resource is not a Request custom resource"
	errTrackPCUsage                 = "cannot track ProviderConfig usage"
	errNewHttpClient                = "cannot create new Http client"
	errProviderNotRetrieved         = "provider could not be retrieved"
	errFailedToSendHttpRequest      = "something went wrong"
	errFailedToCheckIfUpToDate      = "failed to check if request is up to date"
	errFailedToUpdateStatusFailures = "failed to reset status failures counter"
	errFailedUpdateStatusConditions = "failed updating status conditions"
	errPatchDataToSecret            = "Warning, couldn't patch data from request to secret %s:%s:%s, error: %s"
	errGetLatestVersion             = "failed to get the latest version of the resource"
	errExtractCredentials           = "cannot extract credentials"
)

// Setup adds a controller that reconciles Request managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha2.RequestGroupKind)
	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.RequestGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			logger:          o.Logger,
			kube:            mgr.GetClient(),
			usage:           resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newHttpClientFn: httpClient.NewClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithTimeout(timeout),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.Request{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	logger          logging.Logger
	kube            client.Client
	usage           resource.Tracker
	newHttpClientFn func(log logging.Logger, timeout time.Duration, creds string) (httpClient.Client, error)
}

// Connect creates a new external client using the provider config.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha2.Request)
	if !ok {
		return nil, errors.New(errNotRequest)
	}

	l := c.logger.WithValues("request", cr.Name)

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	n := types.NamespacedName{Name: cr.GetProviderConfigReference().Name}
	if err := c.kube.Get(ctx, n, pc); err != nil {
		return nil, errors.Wrap(err, errProviderNotRetrieved)
	}

	var creds string = ""
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

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	localKube client.Client
	logger    logging.Logger
	http      httpClient.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha2.Request)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRequest)
	}

	observeRequestDetails, err := c.isUpToDate(ctx, cr)
	if err != nil && err.Error() == errObjectNotFound {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errFailedToCheckIfUpToDate)
	}

	// Get the latest version of the resource before updating
	if err := c.localKube.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetLatestVersion)
	}

	statusHandler, err := statushandler.NewStatusHandler(ctx, cr, observeRequestDetails.Details, observeRequestDetails.ResponseError, c.localKube, c.logger)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	synced := observeRequestDetails.Synced
	if synced {
		statusHandler.ResetFailures()
	}

	cr.Status.SetConditions(xpv1.Available())
	err = statusHandler.SetRequestStatus()
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, " failed updating status")
	}

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  synced,
		ConnectionDetails: nil,
	}, nil
}

// deployAction executes the action based on the given Request resource and Mapping configuration.
func (c *external) deployAction(ctx context.Context, cr *v1alpha2.Request, action string) error {
	mapping, err := requestmapping.GetMapping(&cr.Spec.ForProvider, action, c.logger)
	if err != nil {
		c.logger.Info(err.Error())
		return nil
	}

	requestDetails, err := c.generateValidRequestDetails(ctx, cr, mapping)
	if err != nil {
		return err
	}

	details, err := c.http.SendRequest(ctx, mapping.Method, requestDetails.Url, requestDetails.Body, requestDetails.Headers, cr.Spec.ForProvider.InsecureSkipTLSVerify)
	c.patchResponseToSecret(ctx, cr, &details.HttpResponse)

	statusHandler, err := statushandler.NewStatusHandler(ctx, cr, details, err, c.localKube, c.logger)
	if err != nil {
		return err
	}

	return statusHandler.SetRequestStatus()
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha2.Request)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRequest)
	}

	return managed.ExternalCreation{}, errors.Wrap(c.deployAction(ctx, cr, v1alpha2.ActionCreate), errFailedToSendHttpRequest)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha2.Request)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRequest)
	}

	return managed.ExternalUpdate{}, errors.Wrap(c.deployAction(ctx, cr, v1alpha2.ActionUpdate), errFailedToSendHttpRequest)
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha2.Request)
	if !ok {
		return errors.New(errNotRequest)
	}

	return errors.Wrap(c.deployAction(ctx, cr, v1alpha2.ActionRemove), errFailedToSendHttpRequest)
}

// patchResponseToSecret patches the response data to the secret based on the given Request resource and Mapping configuration.
func (c *external) patchResponseToSecret(ctx context.Context, cr *v1alpha2.Request, response *httpClient.HttpResponse) {
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

// generateValidRequestDetails generates valid request details based on the given Request resource and Mapping configuration.
// It first attempts to generate request details using the HTTP response stored in the Request's status. If the generated
// details are valid, the function returns them. If not, it falls back to using the cached response in the Request's status
// and attempts to generate request details again. The function returns the generated request details or an error if the
// generation process fails.
func (c *external) generateValidRequestDetails(ctx context.Context, cr *v1alpha2.Request, mapping *v1alpha2.Mapping) (requestgen.RequestDetails, error) {
	requestDetails, _, ok := requestgen.GenerateRequestDetails(ctx, c.localKube, *mapping, cr.Spec.ForProvider, cr.Status.Response, c.logger)
	if requestgen.IsRequestValid(requestDetails) && ok {
		return requestDetails, nil
	}

	requestDetails, err, _ := requestgen.GenerateRequestDetails(ctx, c.localKube, *mapping, cr.Spec.ForProvider, cr.Status.Cache.Response, c.logger)
	if err != nil {
		return requestgen.RequestDetails{}, err
	}

	return requestDetails, nil
}
