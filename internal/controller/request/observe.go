package request

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/json"
	"github.com/arielsepton/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
)

const (
	errObjectNotFound         = "object wasn't created"
	errNoGetMapping           = "forProvider doesn't contain GET mapping"
	errNoPutMapping           = "forProvider doesn't contain PUT mapping"
	errBodyNotValidJSON       = "response body is not a valid JSON string "
	errPutMappingNotValidJSON = "PUT mapping is not a valid JSON string"
)

// isUpToDate checks whether desired spec up to date with the observed state for a given request
func (c *external) isUpToDate(ctx context.Context, cr *v1alpha1.Request) (bool, error) {
	if cr.Status.Response.Body == "" ||
		(cr.Status.Response.Method == http.MethodPost && utils.IsHTTPError(cr.Status.Response.StatusCode)) {
		return false, errors.New(errObjectNotFound)
	}

	methodGetMapping, ok := getMappingByMethod(&cr.Spec.ForProvider, http.MethodGet)
	if !ok {
		return false, errors.New(errNoGetMapping)
	}

	requestDetails, err := generateValidRequestDetails(cr, methodGetMapping, c.logger)
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

	desiredState, err := c.desiredState(cr, c.logger)
	if err != nil {
		return false, err
	}

	err = c.setRequestStatus(ctx, cr, res, err)
	if err != nil {
		return false, err
	}

	if json.IsJSONString(res.Body) && json.IsJSONString(desiredState) {
		responseBodyMap := json.JsonStringToMap(res.Body)
		desiredStateMap := json.JsonStringToMap(desiredState)
		return json.Contains(responseBodyMap, desiredStateMap) && utils.IsHTTPSuccess(res.StatusCode), nil
	}

	if !json.IsJSONString(res.Body) && json.IsJSONString(desiredState) {
		return false, errors.New(fmt.Sprint(errBodyNotValidJSON, res.Body))
	}

	if json.IsJSONString(res.Body) && !json.IsJSONString(desiredState) {
		return false, errors.New(errPutMappingNotValidJSON)
	}

	return strings.Contains(res.Body, desiredState) && utils.IsHTTPSuccess(res.StatusCode), nil
}

func (c *external) desiredState(cr *v1alpha1.Request, logger logging.Logger) (string, error) {
	methodPutMapping, ok := getMappingByMethod(&cr.Spec.ForProvider, http.MethodPut)
	if !ok {
		// TODO (REL): maybe here use POST if PUT is not present.
		return "", errors.New(errNoPutMapping)
	}

	requestDetails, err := generateValidRequestDetails(cr, methodPutMapping, c.logger)
	return requestDetails.Body, err
}
