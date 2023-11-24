package statushandler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"github.com/arielsepton/provider-http/internal/controller/request/requestgen"
	"github.com/arielsepton/provider-http/internal/controller/request/responseconverter"
	"github.com/arielsepton/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RequestStatusHandler is the interface to interact with status setting for v1alpha1.Request
type RequestStatusHandler interface {
	SetRequestStatus(mappings []v1alpha1.Mapping, forProvider v1alpha1.RequestParameters) error
	ResetFailures()
}

// requestStatusHandler sets the request status.
// it checks wether to set cache, and failures count.
type requestStatusHandler struct {
	logger       logging.Logger
	resource     *utils.RequestResource
	httpError    error
	basicSetters *[]utils.SetRequestStatusFunc
}

// SetRequestStatus updates the current Request's status to reflect the details of the last HTTP request that occurred.
// It takes the context, the Request resource, the HTTP response, the mapping configuration, and any error that occurred
// during the HTTP request. The function sets the status fields such as StatusCode, Headers, Body, Method, and Cache,
// based on the outcome of the HTTP request and the presence of an error.
func (r *requestStatusHandler) SetRequestStatus(mappings []v1alpha1.Mapping, forProvider v1alpha1.RequestParameters) error {
	if r.httpError != nil {
		return r.setErrorAndReturn(r.resource, r.httpError)
	}

	if utils.IsHTTPError(r.resource.HttpResponse.StatusCode) {
		return r.incrementFailuresAndReturn(r.resource, *r.basicSetters)
	}

	if utils.IsHTTPSuccess(r.resource.HttpResponse.StatusCode) {
		r.appendExtraSetters(mappings, forProvider)
	}

	return utils.SetRequestResourceStatus(*r.resource, *r.basicSetters...)
}

func (r *requestStatusHandler) setErrorAndReturn(resource *utils.RequestResource, err error) error {
	utils.SetRequestResourceStatus(*resource, resource.SetError(err))
	return err
}

func (r *requestStatusHandler) incrementFailuresAndReturn(resource *utils.RequestResource, basicSetters []utils.SetRequestStatusFunc) error {
	basicSetters = append(basicSetters, resource.SetError(nil)) // should increment failures counter

	utils.SetRequestResourceStatus(*resource, basicSetters...)
	return errors.Errorf(utils.ErrStatusCode, resource.HttpResponse.Method, strconv.Itoa(resource.HttpResponse.StatusCode))
}

func (r *requestStatusHandler) appendExtraSetters(mappings []v1alpha1.Mapping, forProvider v1alpha1.RequestParameters) {
	if r.resource.HttpResponse.Method != http.MethodGet {
		*r.basicSetters = append(*r.basicSetters, r.resource.ResetFailures())
	}

	if r.shouldSetCache(mappings, forProvider) {
		*r.basicSetters = append(*r.basicSetters, r.resource.SetCache())
	}
}

// shouldSetCache determines whether the cache should be updated based on the provided mapping, HTTP response,
// and RequestParameters. It generates request details according to the given mapping and response. If the request
// details are not valid, it means that instead of using the response, the cache should be used.
func (r *requestStatusHandler) shouldSetCache(mappings []v1alpha1.Mapping, forProvider v1alpha1.RequestParameters) bool {
	for _, mapping := range mappings {
		response := responseconverter.HttpResponseToV1alpha1Response(r.resource.HttpResponse)
		requestDetails, _, ok := requestgen.GenerateRequestDetails(mapping, forProvider, response, r.logger)
		if !(requestgen.IsRequestValid(requestDetails) && ok) {
			return false
		}
	}

	return true
}

func (r *requestStatusHandler) ResetFailures() {
	utils.SetRequestResourceStatus(*r.resource, r.resource.ResetFailures())
}

// NewClient returns a new Request statusHandler
func NewStatusHandler(ctx context.Context, cr *v1alpha1.Request, client client.Client, res httpClient.HttpResponse, err error, logger logging.Logger) RequestStatusHandler {
	requestStatusHandler := &requestStatusHandler{
		logger: logger,
		resource: &utils.RequestResource{
			Resource:       cr,
			HttpResponse:   res,
			RequestContext: ctx,
			LocalClient:    client,
		},
		basicSetters: &[]utils.SetRequestStatusFunc{},
	}

	*requestStatusHandler.basicSetters = []utils.SetRequestStatusFunc{
		requestStatusHandler.resource.SetStatusCode(),
		requestStatusHandler.resource.SetHeaders(),
		requestStatusHandler.resource.SetBody(),
		requestStatusHandler.resource.SetMethod(),
	}

	return requestStatusHandler

}
