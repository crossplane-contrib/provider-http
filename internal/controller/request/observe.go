package request

import (
	"context"
	"net/http"
	"strings"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/controller/request/requestgen"
	"github.com/pkg/errors"
)

const (
	errObjectNotFound = "object wasn't created"
	errNoGetMapping   = "forProvider doesn't contain GET mapping"
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

	// TODO (REL) : delete this:
	// urlJQFilter := methodGetMapping.URL
	// GetURL, err := requestgen.GenerateURL(urlJQFilter, cr)
	// if err != nil {
	// 	return false, err
	// }

	requestDetails, err := requestgen.GenerateValidRequestDetails(*methodGetMapping, cr)
	if err != nil {
		return false, err
	}

	// TODO (REL): handle headers from payload
	res, err := c.http.SendRequest(ctx, http.MethodGet, requestDetails.Url, requestDetails.Body, cr.Spec.ForProvider.Headers)
	if err != nil {
		return false, err
	}

	if res.StatusCode == http.StatusNotFound {
		return false, errors.New(errObjectNotFound)
	}

	desiredState, err := desiredState(cr)
	if err != nil {
		return false, err
	}

	return strings.Contains(res.ResponseBody, desiredState) && isHTTPSuccess(res.StatusCode), nil
}

func desiredState(cr *v1alpha1.Request) (string, error) {
	methodPutMapping, ok := getMappingByMethod(&cr.Spec.ForProvider, http.MethodPut)
	if !ok {
		return "", errors.New(errNoGetMapping)
	}

	return requestgen.GenerateBody(methodPutMapping.Body, cr)
}

func isHTTPSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
