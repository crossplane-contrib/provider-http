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
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
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

	"github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
	apisv1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/namespaced/request/observe"
	"github.com/crossplane-contrib/provider-http/internal/controller/namespaced/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/controller/namespaced/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/controller/namespaced/request/statushandler"
	"github.com/crossplane-contrib/provider-http/internal/controller/typeconv"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/utils"
)

const (
	errNotRequest                   = "managed resource is not a namespaced Request custom resource"
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

	errGetPC    = "cannot get ProviderConfig"
	errGetCPC   = "cannot get ClusterProviderConfig"
	errGetCreds = "cannot get credentials"
)

// Setup adds a controller that reconciles namespaced Request managed resources.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	name := managed.ControllerName(v1alpha2.RequestGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha2.RequestGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			logger:          o.Logger,
			kube:            mgr.GetClient(),
			newHttpClientFn: httpClient.NewClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithTimeout(timeout),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha2.Request{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	logger          logging.Logger
	kube            client.Client
	newHttpClientFn func(log logging.Logger, timeout time.Duration, creds string) (httpClient.Client, error)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha2.Request)
	if !ok {
		return nil, errors.New(errNotRequest)
	}

	l := c.logger.WithValues("namespacedRequest", cr.Name)

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

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha2.Request)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRequest)
	}

	observeRequestDetails, err := c.isUpToDate(ctx, cr)
	if err != nil && err.Error() == observe.ErrObjectNotFound {
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

	// Use namespaced statushandler directly
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

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha2.Request)
	cr.Status.SetConditions(xpv1.Creating())
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

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha2.Request)
	cr.Status.SetConditions(xpv1.Deleting())
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotRequest)
	}

	return managed.ExternalDelete{}, errors.Wrap(c.deployAction(ctx, cr, v1alpha2.ActionRemove), errFailedToSendHttpRequest)
}

// deployAction executes the action based on the given Request resource and Mapping configuration.
func (c *external) deployAction(ctx context.Context, cr *v1alpha2.Request, action string) error {
	// Convert namespaced types for requestmapping and requestgen compatibility
	clusterForProvider := typeconv.ConvertNamespacedToClusterRequestParameters(&cr.Spec.ForProvider)
	mapping, err := requestmapping.GetMapping(clusterForProvider, action, c.logger)
	if err != nil {
		c.logger.Info(err.Error())
		return nil
	}

	// Convert namespaced request to cluster request for requestgen
	clusterCR, err := typeconv.ToClusterRequest(cr)
	if err != nil {
		return err
	}

	requestDetails, err := requestgen.GenerateValidRequestDetails(ctx, clusterCR, mapping, c.localKube, c.logger)
	if err != nil {
		return err
	}

	details, err := c.http.SendRequest(ctx, mapping.Method, requestDetails.Url, requestDetails.Body, requestDetails.Headers, cr.Spec.ForProvider.InsecureSkipTLSVerify)
	datapatcher.ApplyResponseDataToSecrets(ctx, c.localKube, c.logger, &details.HttpResponse, cr.Spec.ForProvider.SecretInjectionConfigs, cr)

	statusHandler, err := statushandler.NewStatusHandler(ctx, cr, details, err, c.localKube, c.logger)
	if err != nil {
		return err
	}

	return statusHandler.SetRequestStatus()
}

func (c *external) Disconnect(_ context.Context) error {
	return nil
}
