package request

import (
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/common"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/request/observe"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestmapping"
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

// IsUpToDate checks whether desired spec up to date with the observed state for a given request
func IsUpToDate(svcCtx *service.ServiceContext, crCtx *service.RequestCRContext) (ObserveRequestDetails, error) {
	spec := crCtx.Spec()
	mapping, err := requestmapping.GetMapping(spec, common.ActionObserve, svcCtx.Logger)
	if err != nil {
		return FailedObserve(), err
	}

	objectNotCreated := !isObjectValidForObservation(crCtx)

	// Evaluate the HTTP request template. If successfully templated, attempt to
	// observe the resource.
	requestDetails, err := requestgen.GenerateValidRequestDetails(svcCtx, crCtx, mapping)
	if err != nil {
		if objectNotCreated {
			// The initial request was not successfully templated. Cannot
			// confirm existence of the resource, jumping to the default
			// behavior of creating before observing.
			err = errors.New(observe.ErrObjectNotFound)
		}
		return FailedObserve(), err
	}

	details, responseErr := svcCtx.HTTP.SendRequest(svcCtx.Ctx, requestmapping.GetEffectiveMethod(mapping), requestDetails.Url, requestDetails.Body, requestDetails.Headers, svcCtx.TLSConfigData)
	// The initial observation of an object requires a successful HTTP response
	// to be considered existing.
	if !utils.IsHTTPSuccess(details.HttpResponse.StatusCode) && objectNotCreated {
		// Cannot confirm existence of the resource, jumping to the default
		// behavior of creating before observing.
		return FailedObserve(), errors.New(observe.ErrObjectNotFound)
	}
	if err := determineIfRemoved(svcCtx, crCtx, details, responseErr); err != nil {
		return FailedObserve(), err
	}

	// Apply response data to secrets and update CR status with response
	secretConfigs := spec.GetSecretInjectionConfigs()
	datapatcher.ApplyResponseDataToSecrets(svcCtx.Ctx, svcCtx.LocalKube, svcCtx.Logger, &details.HttpResponse, secretConfigs, crCtx.GetCR())
	return determineIfUpToDate(svcCtx, crCtx, details, responseErr)
}

// determineIfUpToDate determines if the object is up to date based on the response check.
func determineIfUpToDate(svcCtx *service.ServiceContext, crCtx *service.RequestCRContext, details httpClient.HttpDetails, responseErr error) (ObserveRequestDetails, error) {
	responseChecker := observe.GetIsUpToDateResponseCheck(svcCtx, crCtx.Spec())
	if responseChecker == nil {
		return FailedObserve(), errors.Errorf(errExpectedResponseCheckType, "expectedResponseCheck")
	}

	result, err := responseChecker.Check(svcCtx, crCtx, details, responseErr)
	if err != nil {
		return FailedObserve(), err
	}

	return NewObserve(details, responseErr, result), nil
}

// determineIfRemoved determines if the object is removed based on the response check.
func determineIfRemoved(svcCtx *service.ServiceContext, crCtx *service.RequestCRContext, details httpClient.HttpDetails, responseErr error) error {
	responseChecker := observe.GetIsRemovedResponseCheck(svcCtx, crCtx.Spec())
	if responseChecker == nil {
		return errors.Errorf(errExpectedResponseCheckType, "isRemovedCheck")
	}

	return responseChecker.Check(svcCtx, crCtx, details, responseErr)
}

// isObjectValidForObservation checks if the object is valid for observation
func isObjectValidForObservation(crCtx *service.RequestCRContext) bool {
	response := crCtx.Status().GetResponse()
	requestDetails := crCtx.Status().GetRequestDetails()
	spec := crCtx.Spec()

	return response.GetStatusCode() != 0 &&
		!(requestDetails.GetMethod() == http.MethodPost && utils.IsHTTPError(response.GetStatusCode(), spec.GetAllowedStatusCodes()))
}
