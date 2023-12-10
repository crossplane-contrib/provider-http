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
	SetRequestStatus(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error
	ResetFailures(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error
}

// requestStatusHandler sets the request status.
// it checks wether to set cache, and failures count.
type requestStatusHandler struct {
	logger    logging.Logger
	localKube client.Client
}

// SetRequestStatus updates the current Request's status to reflect the details of the last HTTP request that occurred.
// It takes the context, the Request resource, the HTTP response, the mapping configuration, and any error that occurred
// during the HTTP request. The function sets the status fields such as StatusCode, Headers, Body, Method, and Cache,
// based on the outcome of the HTTP request and the presence of an error.
func (r *requestStatusHandler) SetRequestStatus(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error {
	resource := &utils.RequestResource{
		Resource:       cr,
		HttpResponse:   res,
		RequestContext: ctx,
		LocalClient:    r.localKube,
	}

	basicSetters := []utils.SetRequestStatusFunc{
		resource.SetStatusCode(),
		resource.SetHeaders(),
		resource.SetBody(),
		resource.SetMethod(),
	}

	if err != nil {
		return r.setErrorAndReturn(resource, err)
	}

	if utils.IsHTTPError(resource.HttpResponse.StatusCode) {
		return r.incrementFailuresAndReturn(resource, basicSetters)
	}

	if utils.IsHTTPSuccess(resource.HttpResponse.StatusCode) {
		r.appendExtraSetters(cr.Spec.ForProvider, *resource, &basicSetters)
	}

	if settingError := utils.SetRequestResourceStatus(*resource, basicSetters...); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return nil
}

func (r *requestStatusHandler) setErrorAndReturn(resource *utils.RequestResource, err error) error {
	if settingError := utils.SetRequestResourceStatus(*resource, resource.SetError(err)); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return err
}

func (r *requestStatusHandler) incrementFailuresAndReturn(resource *utils.RequestResource, basicSetters []utils.SetRequestStatusFunc) error {
	basicSetters = append(basicSetters, resource.SetError(nil)) // should increment failures counter

	if settingError := utils.SetRequestResourceStatus(*resource, basicSetters...); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return errors.Errorf(utils.ErrStatusCode, resource.HttpResponse.Method, strconv.Itoa(resource.HttpResponse.StatusCode))
}

func (r *requestStatusHandler) appendExtraSetters(forProvider v1alpha1.RequestParameters, resource utils.RequestResource, basicSetters *[]utils.SetRequestStatusFunc) {
	if resource.HttpResponse.Method != http.MethodGet {
		*basicSetters = append(*basicSetters, resource.ResetFailures())
	}

	if r.shouldSetCache(forProvider, resource) {
		*basicSetters = append(*basicSetters, resource.SetCache())
	}
}

// shouldSetCache determines whether the cache should be updated based on the provided mapping, HTTP response,
// and RequestParameters. It generates request details according to the given mapping and response. If the request
// details are not valid, it means that instead of using the response, the cache should be used.
func (r *requestStatusHandler) shouldSetCache(forProvider v1alpha1.RequestParameters, resource utils.RequestResource) bool {
	for _, mapping := range forProvider.Mappings {
		response := responseconverter.HttpResponseToV1alpha1Response(resource.HttpResponse)
		requestDetails, _, ok := requestgen.GenerateRequestDetails(mapping, forProvider, response)
		if !(requestgen.IsRequestValid(requestDetails) && ok) {
			return false
		}
	}

	return true
}

func (r *requestStatusHandler) ResetFailures(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error {
	resource := &utils.RequestResource{
		Resource:       cr,
		HttpResponse:   res,
		RequestContext: ctx,
		LocalClient:    r.localKube,
	}

	if settingError := utils.SetRequestResourceStatus(*resource, resource.ResetFailures()); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return nil
}

// NewClient returns a new Request statusHandler
func NewStatusHandler(client client.Client, logger logging.Logger) RequestStatusHandler {
	requestStatusHandler := &requestStatusHandler{
		logger:    logger,
		localKube: client,
	}

	return requestStatusHandler
}
