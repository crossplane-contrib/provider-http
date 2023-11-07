/*
Copyright 2022 The Crossplane Authors.

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
	"net/http"
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

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	apisv1alpha1 "github.com/arielsepton/provider-http/apis/v1alpha1"
	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	requestsUtils "github.com/arielsepton/provider-http/internal/utils/requests"
)

const (
	defaultWaitTimeout = 5 * time.Minute
)

const (
	errNotRequest              = "managed resource is not a Request custom resource"
	errTrackPCUsage            = "cannot track ProviderConfig usage"
	errNewHttpClient           = "cannot create new Http client"
	errProviderNotRetrieved    = "provider could not be retrieved"
	errFailedToSendHttpRequest = "failed to send http request"
	errEmptyMethod             = "no method is specified"
	errEmptyURL                = "no url is specified"
	errFailedToCheckIfUpToDate = "failed to check if request is up to date"
	errFailedUpdateCR          = "failed updating CR"
)

// Setup adds a controller that reconciles Request managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha1.RequestGroupKind)
	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.RequestGroupVersionKind),
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
		For(&v1alpha1.Request{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	logger          logging.Logger
	kube            client.Client
	usage           resource.Tracker
	newHttpClientFn func(log logging.Logger, timeout time.Duration) (httpClient.Client, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Request)
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

	h, err := c.newHttpClientFn(l, waitTimeout(cr))
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
	cr, ok := mg.(*v1alpha1.Request)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRequest)
	}

	synced, err := c.isUpToDate(ctx, cr)
	if err != nil && err.Error() == errObjectNotFound {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errFailedToCheckIfUpToDate)
	}

	cr.Status.SetConditions(xpv1.Available())
	if err := c.localKube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalObservation{}, errors.New(errFailedUpdateCR)
	}

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  synced && !(shouldRetry(cr) && !retriesLimitReached(cr)),
		ConnectionDetails: nil,
	}, nil
}

func (c *external) deployAction(ctx context.Context, cr *v1alpha1.Request, method string, url string, body string, headers map[string][]string) error {
	res, err := c.http.SendRequest(ctx, method, url, body, headers)

	resource := &requestsUtils.RequestResource{
		Resource:       cr,
		RequestContext: ctx,
		HttpResponse:   res,
		LocalClient:    c.localKube,
	}

	if err != nil {
		setErr := resource.SetError(err)
		return requestsUtils.SetRequestResourceStatus(*resource, setErr)
	}

	setStatusCode := resource.SetStatusCode()
	setHeaders := resource.SetHeaders()
	setBody := resource.SetBody()

	return requestsUtils.SetRequestResourceStatus(*resource, setStatusCode, setHeaders, setBody)
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	// TODO (REl): implement generation of body and url

	cr, ok := mg.(*v1alpha1.Request)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRequest)
	}

	if err := isRequestValid(http.MethodPost, PostURL); err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{}, errors.Wrap(c.deployAction(ctx, cr, http.MethodPost,
		PostURL, PostBody, cr.Spec.ForProvider.Headers), errFailedToSendHttpRequest)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	// TODO (REl): implement generation of body and url

	cr, ok := mg.(*v1alpha1.Request)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRequest)
	}

	if err := isRequestValid(http.MethodPut, PutURL); err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, errors.Wrap(c.deployAction(ctx, cr, http.MethodPut,
		PutURL, PutBody, cr.Spec.ForProvider.Headers), errFailedToSendHttpRequest)
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	// TODO (REl): implement generation of body and url

	cr, ok := mg.(*v1alpha1.Request)
	if !ok {
		return errors.New(errNotRequest)
	}

	if err := isRequestValid(http.MethodDelete, DeleteURL); err != nil {
		return err
	}

	return errors.Wrap(c.deployAction(ctx, cr, http.MethodDelete,
		DeleteURL, DeleteBody, cr.Spec.ForProvider.Headers), errFailedToSendHttpRequest)
}

// TODO (REl): duplicated code
func shouldRetry(cr *v1alpha1.Request) bool {
	return rollBackEnabled(cr) && cr.Status.Failed != 0
}

// TODO (REl): duplicated code
func rollBackEnabled(cr *v1alpha1.Request) bool {
	return cr.Spec.ForProvider.RollbackRetriesLimit != nil
}

// TODO (REl): duplicated code
func retriesLimitReached(cr *v1alpha1.Request) bool {
	return cr.Status.Failed >= *cr.Spec.ForProvider.RollbackRetriesLimit
}

// TODO (REl): duplicated code
func isRequestValid(method string, url string) error {
	if method == "" {
		return errors.New(errEmptyMethod)
	}

	if url == "" {
		return errors.New(errEmptyURL)
	}

	return nil
}

// TODO (REl): duplicated code
func waitTimeout(cr *v1alpha1.Request) time.Duration {
	if cr.Spec.ForProvider.WaitTimeout != nil {
		return cr.Spec.ForProvider.WaitTimeout.Duration
	}
	return defaultWaitTimeout
}
