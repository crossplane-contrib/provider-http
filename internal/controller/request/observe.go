package request

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/pkg/errors"
)

const (
	errObjectNotFound = "object wasn't found"
	errNotValidJSON   = "%s is not a valid JSON string: %s"
)

type ObserveRequestDetails struct {
	Details       httpClient.HttpDetails
	ResponseError error
	Synced        bool
}

// NewObserveRequestDetails is a constructor function that initializes
// an instance of ObserveRequestDetails with default values.
func NewObserve(details httpClient.HttpDetails, resErr error, synced bool) ObserveRequestDetails {
	return ObserveRequestDetails{
		Synced:        synced,
		Details:       details,
		ResponseError: resErr,
	}
}

// NewObserveRequestDetails is a constructor function that initializes
// an instance of ObserveRequestDetails with default values.
func FailedObserve() ObserveRequestDetails {
	return ObserveRequestDetails{
		Synced: false,
	}
}

// isUpToDate checks whether desired spec up to date with the observed state for a given request
func (c *external) isUpToDate(ctx context.Context, cr *v1alpha2.Request) (ObserveRequestDetails, error) {
	if !c.isObjectValidForObservation(cr) {
		return FailedObserve(), errors.New(errObjectNotFound)
	}

	requestDetails, err := c.requestDetails(ctx, cr, http.MethodGet)
	if err != nil {
		return FailedObserve(), err
	}

	details, responseErr := c.http.SendRequest(ctx, http.MethodGet, requestDetails.Url, requestDetails.Body, requestDetails.Headers, cr.Spec.ForProvider.InsecureSkipTLSVerify)
	if details.HttpResponse.StatusCode == http.StatusNotFound {
		return FailedObserve(), errors.New(errObjectNotFound)
	}

	c.patchResponseToSecret(ctx, cr, &details.HttpResponse)
	desiredState, err := c.desiredState(ctx, cr)
	if err != nil {
		if isErrorMappingNotFound(err) {
			// Since there is no PUT mapping, we skip the check for its presence in the GET response.
			return NewObserve(details, responseErr, true), nil
		}

		// For any other error, we return a failed observation.
		return FailedObserve(), err
	}

	return c.compareResponseAndDesiredState(details, responseErr, desiredState)
}

func (c *external) isObjectValidForObservation(cr *v1alpha2.Request) bool {
	return cr.Status.Response.Body != "" &&
		!(cr.Status.RequestDetails.Method == http.MethodPost && utils.IsHTTPError(cr.Status.Response.StatusCode))
}

func (c *external) compareResponseAndDesiredState(details httpClient.HttpDetails, err error, desiredState string) (ObserveRequestDetails, error) {
	observeRequestDetails := NewObserve(details, err, false)

	if json.IsJSONString(details.HttpResponse.Body) && json.IsJSONString(desiredState) {
		responseBodyMap := json.JsonStringToMap(details.HttpResponse.Body)
		desiredStateMap := json.JsonStringToMap(desiredState)
		observeRequestDetails.Synced = json.Contains(responseBodyMap, desiredStateMap) && utils.IsHTTPSuccess(details.HttpResponse.StatusCode)
		return observeRequestDetails, nil
	}

	if !json.IsJSONString(details.HttpResponse.Body) && json.IsJSONString(desiredState) {
		return FailedObserve(), errors.Errorf(errNotValidJSON, "response body", details.HttpResponse.Body)
	}

	if json.IsJSONString(details.HttpResponse.Body) && !json.IsJSONString(desiredState) {
		return FailedObserve(), errors.Errorf(errNotValidJSON, "PUT mapping result", desiredState)
	}

	observeRequestDetails.Synced = strings.Contains(details.HttpResponse.Body, desiredState) && utils.IsHTTPSuccess(details.HttpResponse.StatusCode)
	return observeRequestDetails, nil
}

func (c *external) desiredState(ctx context.Context, cr *v1alpha2.Request) (string, error) {
	requestDetails, err := c.requestDetails(ctx, cr, http.MethodPut)
	if err != nil {
		return "", err
	}

	return requestDetails.Body.Encrypted.(string), nil
}

func (c *external) requestDetails(ctx context.Context, cr *v1alpha2.Request, method string) (requestgen.RequestDetails, error) {
	mapping, ok := getMappingByMethod(&cr.Spec.ForProvider, method)
	if !ok {
		return requestgen.RequestDetails{}, errors.Errorf(errMappingNotFound, method)
	}

	return c.generateValidRequestDetails(ctx, cr, mapping)
}

// isErrorMappingNotFound checks if the provided error indicates that the
// mapping for an HTTP PUT request is not found.
func isErrorMappingNotFound(err error) bool {
	return errors.Cause(err).Error() == fmt.Sprintf(errMappingNotFound, http.MethodPut)
}
