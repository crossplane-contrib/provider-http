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

package desposiblerequest

import (
	"context"
	"strconv"
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

	"github.com/arielsepton/provider-http/apis/desposiblerequest/v1alpha1"
	apisv1alpha1 "github.com/arielsepton/provider-http/apis/v1alpha1"
	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"github.com/arielsepton/provider-http/internal/utils"
)

const (
	errNotDesposibleRequest              = "managed resource is not a DesposibleRequest custom resource"
	errTrackPCUsage                      = "cannot track ProviderConfig usage"
	errNewHttpClient                     = "cannot create new Http client"
	errProviderNotRetrieved              = "provider could not be retrieved"
	errFailedToSendHttpDesposibleRequest = "failed to send http request"
	errFailedUpdateStatusConditions      = "failed updating status conditions"
)

// Setup adds a controller that reconciles DesposibleRequest managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha1.DesposibleRequestGroupKind)
	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.DesposibleRequestGroupVersionKind),
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
		For(&v1alpha1.DesposibleRequest{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	logger          logging.Logger
	kube            client.Client
	usage           resource.Tracker
	newHttpClientFn func(log logging.Logger, timeout time.Duration) (httpClient.Client, error)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.DesposibleRequest)
	if !ok {
		return nil, errors.New(errNotDesposibleRequest)
	}

	l := c.logger.WithValues("desposibleRequest", cr.Name)

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
	cr, ok := mg.(*v1alpha1.DesposibleRequest)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDesposibleRequest)
	}

	if !cr.Status.Synced {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Get the latest version of the resource before updating
	if err := c.localKube.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "failed to get the latest version of the resource")
	}

	cr.Status.SetConditions(xpv1.Available())
	if err := c.localKube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalObservation{}, errors.New(errFailedUpdateStatusConditions)
	}

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  !(utils.ShouldRetry(cr.Spec.ForProvider.RollbackRetriesLimit, cr.Status.Failed) && !utils.RetriesLimitReached(cr.Status.Failed, cr.Spec.ForProvider.RollbackRetriesLimit)),
		ConnectionDetails: nil,
	}, nil
}

func (c *external) deployAction(ctx context.Context, cr *v1alpha1.DesposibleRequest) error {
	res, err := c.http.SendRequest(ctx, cr.Spec.ForProvider.Method,
		cr.Spec.ForProvider.URL, cr.Spec.ForProvider.Body, cr.Spec.ForProvider.Headers)

	resource := &utils.RequestResource{
		Resource:       cr,
		RequestContext: ctx,
		HttpResponse:   res,
		LocalClient:    c.localKube,
	}

	if err != nil {
		setErr := resource.SetError(err)
		if settingError := utils.SetRequestResourceStatus(*resource, setErr); settingError != nil {
			return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
		}
		return err
	}

	setStatusCode := resource.SetStatusCode()
	setHeaders := resource.SetHeaders()
	setBody := resource.SetBody()
	setSynced := resource.SetSynced()

	if utils.IsHTTPError(res.StatusCode) {
		if settingError := utils.SetRequestResourceStatus(*resource, setStatusCode, setHeaders, setBody, resource.SetError(nil)); settingError != nil {
			return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
		}

		return errors.Errorf(utils.ErrStatusCode, cr.Spec.ForProvider.Method, strconv.Itoa(res.StatusCode))
	}

	return utils.SetRequestResourceStatus(*resource, setStatusCode, setHeaders, setBody, setSynced)
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.DesposibleRequest)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDesposibleRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{}, errors.Wrap(c.deployAction(ctx, cr), errFailedToSendHttpDesposibleRequest)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.DesposibleRequest)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDesposibleRequest)
	}

	if err := utils.IsRequestValid(cr.Spec.ForProvider.Method, cr.Spec.ForProvider.URL); err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, errors.Wrap(c.deployAction(ctx, cr), errFailedToSendHttpDesposibleRequest)
}

func (c *external) Delete(_ context.Context, _ resource.Managed) error {
	return nil
}
