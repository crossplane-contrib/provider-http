package request

import (
	"context"
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/pkg/errors"
)

const (
	errObjectNotFound            = "object wasn't found"
	errNotValidJSON              = "%s is not a valid JSON string: %s"
	errConvertResToMap           = "failed to convert response to map"
	ErrExpectedFormat            = "expectedResponseCheck.Logic JQ filter should return a boolean, but returned error: %s"
	errExpectedResponseCheckType = "expectedResponseCheck.Type should be either DEFAULT, CUSTOM or empty"
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

	mapping, err := requestmapping.GetMapping(&cr.Spec.ForProvider, v1alpha2.ActionObserve, c.logger)
	if err != nil {
		return FailedObserve(), err
	}

	requestDetails, err := c.generateValidRequestDetails(ctx, cr, mapping)
	if err != nil {
		return FailedObserve(), err
	}

	details, responseErr := c.http.SendRequest(ctx, mapping.Method, requestDetails.Url, requestDetails.Body, requestDetails.Headers, cr.Spec.ForProvider.InsecureSkipTLSVerify)
	if details.HttpResponse.StatusCode == http.StatusNotFound {
		return FailedObserve(), errors.New(errObjectNotFound)
	}

	c.patchResponseToSecret(ctx, cr, &details.HttpResponse)
	return c.determineResponseCheck(ctx, cr, details, responseErr)
}

// determineResponseCheck determines the response check based on the expectedResponseCheck.Type
func (c *external) determineResponseCheck(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (ObserveRequestDetails, error) {
	responseChecker := c.getResponseCheck(cr)
	if responseChecker == nil {
		return FailedObserve(), errors.New(errExpectedResponseCheckType)
	}

	return responseChecker.Check(ctx, cr, details, responseErr)
}

// isObjectValidForObservation checks if the object is valid for observation
func (c *external) isObjectValidForObservation(cr *v1alpha2.Request) bool {
	return cr.Status.Response.StatusCode != 0 &&
		!(cr.Status.RequestDetails.Method == http.MethodPost && utils.IsHTTPError(cr.Status.Response.StatusCode))
}

// requestDetails generates the request details for a given method or action.
func (c *external) requestDetails(ctx context.Context, cr *v1alpha2.Request, action string) (requestgen.RequestDetails, error) {
	mapping, err := requestmapping.GetMapping(&cr.Spec.ForProvider, action, c.logger)
	if err != nil {
		return requestgen.RequestDetails{}, err
	}

	return c.generateValidRequestDetails(ctx, cr, mapping)
}
