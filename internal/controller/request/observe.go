package request

import (
	"context"
	"net/http"
	"strings"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
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

	urlJQFilter := methodGetMapping.URL
	GetURL, err := generateURL(urlJQFilter, cr)
	if err != nil {
		return false, err
	}

	// TODO (REL): handle headers from payload
	res, err := c.http.SendRequest(ctx, http.MethodGet, GetURL, "", cr.Spec.ForProvider.Headers)
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

func generateURL(urlJQFilter string, cr *v1alpha1.Request) (string, error) {
	// TODO (REl): implement
	return "", nil
}

func generateBody(mappingBody string, cr *v1alpha1.Request) (string, error) {
	if mappingBody == "" {
		return "", nil
	}

	jqQuery, err := CreateJQQuery(mappingBody)
	if err != nil {
		return "", err
	}

	body, err := applyGoJQ(jqQuery, cr)
	if err != nil {
		return "", err
	}

	return body, nil
}

func desiredState(cr *v1alpha1.Request) (string, error) {
	methodPutMapping, ok := getMappingByMethod(&cr.Spec.ForProvider, http.MethodPut)
	if !ok {
		return "", errors.New(errNoGetMapping)
	}

	return generateBody(methodPutMapping.Body, cr)
}

func isHTTPSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
