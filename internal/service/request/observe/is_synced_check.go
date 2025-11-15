package observe

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/pkg/errors"
)

var (
	errExpectedFormat = "%s.Logic JQ filter should return a boolean, but returned error: %s"
	errNotValidJSON   = "%s is not a valid JSON string: %s"
)

// defaultIsUpToDateResponseCheck performs a default comparison between the response and desired state.
type defaultIsUpToDateResponseCheck struct{}

// Check performs a default comparison between the response and desired state.
func (d *defaultIsUpToDateResponseCheck) Check(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) (bool, error) {
	desiredState, err := d.desiredState(svcCtx, spec, statusReader, cachedReader)
	if err != nil {
		if isErrorMappingNotFound(err) {
			return true, nil
		}
		return false, err
	}

	return d.compareResponseAndDesiredState(svcCtx, details, desiredState)
}

// compareResponseAndDesiredState compares the response and desired state to determine if they are in sync.
func (d *defaultIsUpToDateResponseCheck) compareResponseAndDesiredState(svcCtx *service.ServiceContext, details httpClient.HttpDetails, desiredState string) (bool, error) {
	sensitiveBody, err := d.patchAndValidate(svcCtx, details.HttpResponse.Body)
	if err != nil {
		return false, err
	}

	sensitiveDesiredState, err := d.patchAndValidate(svcCtx, desiredState)
	if err != nil {
		return false, err
	}

	synced, err := d.comparePatchedResults(sensitiveBody, sensitiveDesiredState, details.HttpResponse.StatusCode)
	if err != nil {
		return false, err
	}

	return synced, nil
}

// patchAndValidate patches secrets into a string and validates the result.
func (d *defaultIsUpToDateResponseCheck) patchAndValidate(svcCtx *service.ServiceContext, content string) (string, error) {
	patched, err := datapatcher.PatchSecretsIntoString(svcCtx.Ctx, svcCtx.LocalKube, content, svcCtx.Logger)
	if err != nil {
		return "", err
	}

	return patched, nil
}

// comparePatchedResults compares the patched response and desired state to determine if they are in sync.
func (d *defaultIsUpToDateResponseCheck) comparePatchedResults(body, desiredState string, statusCode int) (bool, error) {
	// Both are JSON strings
	if json.IsJSONString(body) && json.IsJSONString(desiredState) {
		return d.compareJSON(body, desiredState, statusCode), nil
	}

	// Body is not JSON but desired state is JSON
	if !json.IsJSONString(body) && json.IsJSONString(desiredState) {
		return false, errors.Errorf(errNotValidJSON, "response body", body)
	}

	// Body is JSON but desired state is not JSON
	if json.IsJSONString(body) && !json.IsJSONString(desiredState) {
		return false, errors.Errorf(errNotValidJSON, "PUT mapping result", desiredState)
	}

	// Compare as strings if neither are JSON
	return strings.Contains(body, desiredState) && utils.IsHTTPSuccess(statusCode), nil
}

// compareJSON compares two JSON strings to determine if they are in sync.
func (d *defaultIsUpToDateResponseCheck) compareJSON(body, desiredState string, statusCode int) bool {
	responseBodyMap := json.JsonStringToMap(body)
	desiredStateMap := json.JsonStringToMap(desiredState)
	return json.Contains(responseBodyMap, desiredStateMap) && utils.IsHTTPSuccess(statusCode)
}

// desiredState returns the desired state for a given request
func (d *defaultIsUpToDateResponseCheck) desiredState(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse) (string, error) {
	requestDetails, err := d.requestDetails(svcCtx, spec, statusReader, cachedReader, common.ActionUpdate)
	if err != nil {
		return "", err
	}

	return requestDetails.Body.Encrypted.(string), nil
}

// customIsUpToDateResponseCheck performs a custom response check using JQ logic.
type customIsUpToDateResponseCheck struct{}

// Check performs a custom response check using JQ logic.
func (c *customIsUpToDateResponseCheck) Check(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) (bool, error) {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return false, errors.New("spec does not support custom response checks")
	}

	logic := responseCheckAware.GetExpectedResponseCheck().GetLogic()
	customCheck := &customCheck{}

	isUpToDate, err := customCheck.check(svcCtx, spec, details, logic)
	if err != nil {
		return false, errors.Errorf(errExpectedFormat, "expectedResponseCheck", err.Error())
	}

	return isUpToDate, nil
}

// isErrorMappingNotFound checks if the provided error indicates that the
// mapping for an HTTP PUT request is not found.
func isErrorMappingNotFound(err error) bool {
	return errors.Cause(err).Error() == fmt.Sprintf(requestmapping.ErrMappingNotFound, common.ActionUpdate, http.MethodPut)
}

// requestDetails generates the request details for a given method or action.
func (d *defaultIsUpToDateResponseCheck) requestDetails(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, action string) (requestgen.RequestDetails, error) {
	mapping, err := requestmapping.GetMapping(spec, action, svcCtx.Logger)
	if err != nil {
		return requestgen.RequestDetails{}, err
	}

	return requestgen.GenerateValidRequestDetails(svcCtx, spec, mapping, statusReader.GetResponse(), cachedReader.GetCachedResponse())
}

// isUpToDateChecksFactoryMap is a map that associates each check type with its corresponding factory function.
var isUpToDateChecksFactoryMap = map[string]func() responseCheck{
	common.ExpectedResponseCheckTypeDefault: func() responseCheck {
		return &defaultIsUpToDateResponseCheck{}
	},
	common.ExpectedResponseCheckTypeCustom: func() responseCheck {
		return &customIsUpToDateResponseCheck{}
	},
}

// GetIsUpToDateResponseCheck uses a map to select and return the appropriate ResponseCheck.
func GetIsUpToDateResponseCheck(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec) responseCheck {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return isUpToDateChecksFactoryMap[common.ExpectedResponseCheckTypeDefault]()
	}

	if factory, ok := isUpToDateChecksFactoryMap[responseCheckAware.GetExpectedResponseCheck().GetType()]; ok {
		return factory()
	}
	return isUpToDateChecksFactoryMap[common.ExpectedResponseCheckTypeDefault]()
}
