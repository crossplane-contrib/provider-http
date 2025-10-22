package request

import (
	"context"
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/request/observe"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/controller/typeconv"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/pkg/errors"
)

const (
	errNotValidJSON              = "%s is not a valid JSON string: %s"
	errConvertResToMap           = "failed to convert response to map"
	errExpectedResponseCheckType = "%s.Type should be either DEFAULT, CUSTOM or empty"
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
	// Convert namespaced request to cluster request for processing
	clusterCR, err := typeconv.ToClusterRequest(cr)
	if err != nil {
		return FailedObserve(), err
	}

	mapping, err := requestmapping.GetMapping(&clusterCR.Spec.ForProvider, typeconv.ToClusterRequestActionObserve(), c.logger)
	if err != nil {
		return FailedObserve(), err
	}

	objectNotCreated := !c.isObjectValidForObservation(cr)

	// Evaluate the HTTP request template. If successfully templated, attempt to
	// observe the resource.
	requestDetails, err := requestgen.GenerateValidRequestDetails(ctx, clusterCR, mapping, c.localKube, c.logger)
	if err != nil {
		if objectNotCreated {
			// The initial request was not successfully templated. Cannot
			// confirm existence of the resource, jumping to the default
			// behavior of creating before observing.
			err = errors.New(observe.ErrObjectNotFound)
		}
		return FailedObserve(), err
	}

	details, responseErr := c.http.SendRequest(ctx, mapping.Method, requestDetails.Url, requestDetails.Body, requestDetails.Headers, cr.Spec.ForProvider.InsecureSkipTLSVerify)
	// The initial observation of an object requires a successful HTTP response
	// to be considered existing.
	if !utils.IsHTTPSuccess(details.HttpResponse.StatusCode) && objectNotCreated {
		// Cannot confirm existence of the resource, jumping to the default
		// behavior of creating before observing.
		return FailedObserve(), errors.New(observe.ErrObjectNotFound)
	}
	if err := c.determineIfRemoved(ctx, cr, details, responseErr); err != nil {
		return FailedObserve(), err
	}

	datapatcher.ApplyResponseDataToSecrets(ctx, c.localKube, c.logger, &details.HttpResponse, cr.Spec.ForProvider.SecretInjectionConfigs, cr)
	return c.determineIfUpToDate(ctx, cr, details, responseErr)
}

// determineIfUpToDate determines if the object is up to date based on the response check.
func (c *external) determineIfUpToDate(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (ObserveRequestDetails, error) {
	// Convert namespaced request to cluster request for processing
	clusterCR, err := typeconv.ToClusterRequest(cr)
	if err != nil {
		return FailedObserve(), err
	}

	responseChecker := observe.GetIsUpToDateResponseCheck(clusterCR, c.localKube, c.logger, c.http)
	if responseChecker == nil {
		return FailedObserve(), errors.Errorf(errExpectedResponseCheckType, "expectedResponseCheck")
	}

	result, err := responseChecker.Check(ctx, clusterCR, details, responseErr)
	if err != nil {
		return FailedObserve(), err
	}

	return NewObserve(details, responseErr, result), nil
}

// determineIfRemoved determines if the object is removed based on the response check.
func (c *external) determineIfRemoved(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) error {
	// Convert namespaced request to cluster request for processing
	clusterCR, err := typeconv.ToClusterRequest(cr)
	if err != nil {
		return err
	}

	responseChecker := observe.GetIsRemovedResponseCheck(clusterCR, c.localKube, c.logger, c.http)
	if responseChecker == nil {
		return errors.Errorf(errExpectedResponseCheckType, "isRemovedCheck")
	}

	return responseChecker.Check(ctx, clusterCR, details, responseErr)
}

// isObjectValidForObservation checks if the object is valid for observation
func (c *external) isObjectValidForObservation(cr *v1alpha2.Request) bool {
	return cr.Status.Response.StatusCode != 0 &&
		!(cr.Status.RequestDetails.Method == http.MethodPost && utils.IsHTTPError(cr.Status.Response.StatusCode))
}
