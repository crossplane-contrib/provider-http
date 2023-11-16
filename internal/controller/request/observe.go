package request

import (
	"context"
	"fmt"
	"net/http"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/controller/request/requestgen"
	"github.com/arielsepton/provider-http/internal/json"
	"github.com/arielsepton/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
)

const (
	errObjectNotFound = "object wasn't created"
	errNoGetMapping   = "forProvider doesn't contain GET mapping"
	errStatusCode     = "received status code "
)

// isUpToDate checks whether desired spec up to date with the observed state for a given request
func (c *external) isUpToDate(ctx context.Context, cr *v1alpha1.Request) (bool, error) {
	if cr.Status.Response.Body == "" {
		return false, errors.New(errObjectNotFound)
	}

	methodGetMapping, ok := getMappingByMethod(&cr.Spec.ForProvider, http.MethodGet)
	if !ok {
		return false, errors.New(errNoGetMapping)
	}

	requestDetails, err := requestgen.GenerateValidRequestDetails(*methodGetMapping, cr, c.logger)
	if err != nil {
		return false, err
	}

	res, err := c.http.SendRequest(ctx, http.MethodGet, requestDetails.Url, requestDetails.Body, requestDetails.Headers)
	if err != nil {
		return false, err
	}

	if res.StatusCode == http.StatusNotFound {
		return false, errors.New(errObjectNotFound)
	}

	if utils.IsHTTPError(res.StatusCode) {
		return false, errors.New(fmt.Sprint(errStatusCode, res.StatusCode, " indicates an error. aborting"))
	}

	desiredState, err := desiredState(cr, c.logger)
	if err != nil {
		return false, err
	}

	// TODO (REL): check what happens if one of them is not a json.
	responseBodyMap, _ := json.JsonStringToMap(res.ResponseBody)
	desiredStateMap, _ := json.JsonStringToMap(desiredState)
	return json.Contains(responseBodyMap, desiredStateMap) && utils.IsHTTPSuccess(res.StatusCode), nil
}

func desiredState(cr *v1alpha1.Request, logger logging.Logger) (string, error) {
	methodPutMapping, ok := getMappingByMethod(&cr.Spec.ForProvider, http.MethodPut)
	if !ok {
		return "", errors.New(errNoGetMapping)
	}
	requestDetails, err := requestgen.GenerateValidRequestDetails(*methodPutMapping, cr, logger)
	return requestDetails.Body, err
}
