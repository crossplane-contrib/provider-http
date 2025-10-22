package statushandler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/request/responseconverter"
	"github.com/crossplane-contrib/provider-http/internal/controller/typeconv"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RequestStatusHandler is the interface to interact with status setting for v1alpha2.Request
type RequestStatusHandler interface {
	SetRequestStatus() error
	ResetFailures()
}

// requestStatusHandler sets the request status.
// it checks wether to set cache, and failures count.
type requestStatusHandler struct {
	logger        logging.Logger
	extraSetters  *[]utils.SetRequestStatusFunc
	resource      *utils.RequestResource
	responseError error
	forProvider   v1alpha2.RequestParameters
}

// SetRequestStatus updates the current Request's status to reflect the details of the last HTTP request that occurred.
// It takes the context, the Request resource, the HTTP response, the mapping configuration, and any error that occurred
// during the HTTP request. The function sets the status fields such as StatusCode, Headers, Body, Method, and Cache,
// based on the outcome of the HTTP request and the presence of an error.
func (r *requestStatusHandler) SetRequestStatus() error {
	if r.responseError != nil {
		r.logger.Debug("error occurred during HTTP request", "error", r.responseError)
		return r.setErrorAndReturn(r.responseError)
	}

	basicSetters := []utils.SetRequestStatusFunc{
		r.resource.SetStatusCode(),
		r.resource.SetHeaders(),
		r.resource.SetBody(),
		r.resource.SetRequestDetails(),
	}

	basicSetters = append(basicSetters, *r.extraSetters...)

	if utils.IsHTTPError(r.resource.HttpResponse.StatusCode) {
		return r.incrementFailures(basicSetters)
	}

	if utils.IsHTTPSuccess(r.resource.HttpResponse.StatusCode) {
		r.appendExtraSetters(r.forProvider, &basicSetters)
	}

	if settingError := utils.SetRequestResourceStatus(*r.resource, basicSetters...); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return nil
}

// setErrorAndReturn sets the error message in the status of the Request.
func (r *requestStatusHandler) setErrorAndReturn(err error) error {
	r.logger.Debug("Error occurred during HTTP request", "error", err)
	if settingError := utils.SetRequestResourceStatus(*r.resource, r.resource.SetError(err)); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return err
}

// incrementFailures increments the failures counter and sets the error message in the status of the Request.
func (r *requestStatusHandler) incrementFailures(combinedSetters []utils.SetRequestStatusFunc) error {
	combinedSetters = append(combinedSetters, r.resource.SetError(nil)) // should increment failures counter

	if settingError := utils.SetRequestResourceStatus(*r.resource, combinedSetters...); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	r.logger.Debug(fmt.Sprintf("HTTP %s request failed with status code %s, and response %s", strconv.Itoa(r.resource.HttpResponse.StatusCode), strconv.Itoa(r.resource.HttpResponse.StatusCode), r.resource.HttpResponse.Body))
	return nil
}

func (r *requestStatusHandler) appendExtraSetters(forProvider v1alpha2.RequestParameters, combinedSetters *[]utils.SetRequestStatusFunc) {
	if r.resource.HttpRequest.Method != http.MethodGet {
		*combinedSetters = append(*combinedSetters, r.resource.ResetFailures())
	}

	if r.shouldSetCache(forProvider) {
		*combinedSetters = append(*combinedSetters, r.resource.SetCache())
	}
}

// shouldSetCache determines whether the cache should be updated based on the provided mapping, HTTP response,
// and RequestParameters. It generates request details according to the given mapping and response. If the request
// details are not valid, it means that instead of using the response, the cache should be used.
func (r *requestStatusHandler) shouldSetCache(forProvider v1alpha2.RequestParameters) bool {
	for _, mapping := range forProvider.Mappings {
		response := responseconverter.HttpResponseToV1alpha1Response(r.resource.HttpResponse)
		// Convert namespaced types to cluster types for requestgen
		clusterMapping := typeconv.ConvertNamespacedToClusterMappingStatus(mapping)
		clusterForProvider := typeconv.ConvertNamespacedToClusterRequestParameters(&forProvider)
		requestDetails, _, ok := requestgen.GenerateRequestDetails(r.resource.RequestContext, r.resource.LocalClient, clusterMapping, *clusterForProvider, response, r.logger)
		if !(requestgen.IsRequestValid(requestDetails) && ok) {
			return false
		}
	}

	return true
}

// ResetFailures resets the failures counter in the status of the Request.
func (r *requestStatusHandler) ResetFailures() {
	if r.extraSetters == nil {
		r.extraSetters = &[]utils.SetRequestStatusFunc{}
	}

	*r.extraSetters = append(*r.extraSetters, r.resource.ResetFailures())
}

// NewClient returns a new Request statusHandler
func NewStatusHandler(ctx context.Context, cr *v1alpha2.Request, requestDetails httpClient.HttpDetails, err error, localKube client.Client, logger logging.Logger) (RequestStatusHandler, error) {
	// Get the latest version of the resource before updating
	if err := localKube.Get(ctx, types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}, cr); err != nil {
		return nil, errors.Wrap(err, "failed to get the latest version of the resource")
	}

	requestStatusHandler := &requestStatusHandler{
		logger:       logger,
		extraSetters: &[]utils.SetRequestStatusFunc{},
		resource: &utils.RequestResource{
			Resource:       cr,
			HttpResponse:   requestDetails.HttpResponse,
			HttpRequest:    requestDetails.HttpRequest,
			RequestContext: ctx,
			LocalClient:    localKube,
		},
		responseError: err,
		forProvider:   cr.Spec.ForProvider,
	}

	return requestStatusHandler, nil
}
