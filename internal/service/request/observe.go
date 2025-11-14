package request

import (
	"context"
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/service/request/observe"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// IsUpToDate checks whether desired spec up to date with the observed state for a given request
func IsUpToDate(ctx context.Context, cr interfaces.RequestResource, localKube client.Client, logger logging.Logger, httpClient httpClient.Client) (ObserveRequestDetails, error) {
	spec := cr.GetSpec()
	objectNotCreated := !isObjectValidForObservation(cr)

	mapping, err := requestmapping.GetMapping(spec, common.ActionObserve, logger)
	if err != nil {
		if objectNotCreated {
			// No mapping found and object not created yet, jumping to the default
			// behavior of creating before observing.
			return FailedObserve(), errors.New(observe.ErrObjectNotFound)
		}
		return FailedObserve(), err
	}

	// Evaluate the HTTP request template. If successfully templated, attempt to
	// observe the resource.
	requestDetails, err := requestgen.GenerateValidRequestDetails(ctx, spec, mapping, cr.GetResponse(), cr.GetCachedResponse(), localKube, logger)
	if err != nil {
		if objectNotCreated {
			// The initial request was not successfully templated. Cannot
			// confirm existence of the resource, jumping to the default
			// behavior of creating before observing.
			err = errors.New(observe.ErrObjectNotFound)
		}
		return FailedObserve(), err
	}

	details, responseErr := httpClient.SendRequest(ctx, requestmapping.GetEffectiveMethod(mapping), requestDetails.Url, requestDetails.Body, requestDetails.Headers, spec.GetInsecureSkipTLSVerify())
	// The initial observation of an object requires a successful HTTP response
	// to be considered existing.
	if !utils.IsHTTPSuccess(details.HttpResponse.StatusCode) && objectNotCreated {
		// Cannot confirm existence of the resource, jumping to the default
		// behavior of creating before observing.
		return FailedObserve(), errors.New(observe.ErrObjectNotFound)
	}
	if err := determineIfRemoved(ctx, spec, cr, cr, details, responseErr, localKube, logger, httpClient); err != nil {
		return FailedObserve(), err
	}

	// Get SecretInjectionConfigs from spec
	secretConfigs := spec.GetSecretInjectionConfigs()
	datapatcher.ApplyResponseDataToSecrets(ctx, localKube, logger, &details.HttpResponse, secretConfigs, cr)
	return determineIfUpToDate(ctx, spec, cr, cr, details, responseErr, localKube, logger, httpClient)
}

// determineIfUpToDate determines if the object is up to date based on the response check.
func determineIfUpToDate(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error, localKube client.Client, logger logging.Logger, httpClient httpClient.Client) (ObserveRequestDetails, error) {
	responseChecker := observe.GetIsUpToDateResponseCheck(spec, localKube, logger, httpClient)
	if responseChecker == nil {
		return FailedObserve(), errors.Errorf(errExpectedResponseCheckType, "expectedResponseCheck")
	}

	result, err := responseChecker.Check(ctx, spec, statusReader, cachedReader, details, responseErr)
	if err != nil {
		return FailedObserve(), err
	}

	return NewObserve(details, responseErr, result), nil
}

// determineIfRemoved determines if the object is removed based on the response check.
func determineIfRemoved(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error, localKube client.Client, logger logging.Logger, httpClient httpClient.Client) error {
	responseChecker := observe.GetIsRemovedResponseCheck(spec, localKube, logger, httpClient)
	if responseChecker == nil {
		return errors.Errorf(errExpectedResponseCheckType, "isRemovedCheck")
	}

	return responseChecker.Check(ctx, spec, statusReader, cachedReader, details, responseErr)
}

// isObjectValidForObservation checks if the object is valid for observation
func isObjectValidForObservation(cr interfaces.RequestResource) bool {
	response := cr.GetResponse()
	requestDetails := cr.GetRequestDetails()

	return response.GetStatusCode() != 0 &&
		!(requestDetails.GetMethod() == http.MethodPost && utils.IsHTTPError(response.GetStatusCode()))
}
