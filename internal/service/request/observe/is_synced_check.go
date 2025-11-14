package observe

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errExpectedFormat = "%s.Logic JQ filter should return a boolean, but returned error: %s"
	errNotValidJSON   = "%s is not a valid JSON string: %s"
)

// defaultIsUpToDateResponseCheck performs a default comparison between the response and desired state.
type defaultIsUpToDateResponseCheck struct {
	localKube client.Client
	logger    logging.Logger
	http      httpClient.Client
}

// Check performs a default comparison between the response and desired state.
func (d *defaultIsUpToDateResponseCheck) Check(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) (bool, error) {
	desiredState, err := d.desiredState(ctx, spec, statusReader, cachedReader)
	if err != nil {
		if isErrorMappingNotFound(err) {
			return true, nil
		}
		return false, err
	}

	return d.compareResponseAndDesiredState(ctx, details, desiredState)
}

// compareResponseAndDesiredState compares the response and desired state to determine if they are in sync.
func (d *defaultIsUpToDateResponseCheck) compareResponseAndDesiredState(ctx context.Context, details httpClient.HttpDetails, desiredState string) (bool, error) {
	sensitiveBody, err := d.patchAndValidate(ctx, details.HttpResponse.Body)
	if err != nil {
		return false, err
	}

	sensitiveDesiredState, err := d.patchAndValidate(ctx, desiredState)
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
func (d *defaultIsUpToDateResponseCheck) patchAndValidate(ctx context.Context, content string) (string, error) {
	patched, err := datapatcher.PatchSecretsIntoString(ctx, d.localKube, content, d.logger)
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
func (d *defaultIsUpToDateResponseCheck) desiredState(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse) (string, error) {
	requestDetails, err := d.requestDetails(ctx, spec, statusReader, cachedReader, common.ActionUpdate)
	if err != nil {
		return "", err
	}

	return requestDetails.Body.Encrypted.(string), nil
}

// customIsUpToDateResponseCheck performs a custom response check using JQ logic.
type customIsUpToDateResponseCheck struct {
	localKube client.Client
	logger    logging.Logger
	http      httpClient.Client
}

// Check performs a custom response check using JQ logic.
func (c *customIsUpToDateResponseCheck) Check(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) (bool, error) {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return false, errors.New("spec does not support custom response checks")
	}

	logic := responseCheckAware.GetExpectedResponseCheck().GetLogic()
	customCheck := &customCheck{localKube: c.localKube, logger: c.logger, http: c.http}

	isUpToDate, err := customCheck.check(ctx, spec, details, logic)
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
func (d *defaultIsUpToDateResponseCheck) requestDetails(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, action string) (requestgen.RequestDetails, error) {
	mapping, err := requestmapping.GetMapping(spec, action, d.logger)
	if err != nil {
		return requestgen.RequestDetails{}, err
	}

	return requestgen.GenerateValidRequestDetails(ctx, spec, mapping, statusReader.GetResponse(), cachedReader.GetCachedResponse(), d.localKube, d.logger)
}

// isUpToDateChecksFactoryMap is a map that associates each check type with its corresponding factory function.
var isUpToDateChecksFactoryMap = map[string]func(localKube client.Client, logger logging.Logger, http httpClient.Client) responseCheck{
	common.ExpectedResponseCheckTypeDefault: func(localKube client.Client, logger logging.Logger, http httpClient.Client) responseCheck {
		return &defaultIsUpToDateResponseCheck{localKube: localKube, logger: logger, http: http}
	},
	common.ExpectedResponseCheckTypeCustom: func(localKube client.Client, logger logging.Logger, http httpClient.Client) responseCheck {
		return &customIsUpToDateResponseCheck{localKube: localKube, logger: logger, http: http}
	},
}

// GetIsUpToDateResponseCheck uses a map to select and return the appropriate ResponseCheck.
func GetIsUpToDateResponseCheck(spec interfaces.MappedHTTPRequestSpec, localKube client.Client, logger logging.Logger, http httpClient.Client) responseCheck {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return isUpToDateChecksFactoryMap[common.ExpectedResponseCheckTypeDefault](localKube, logger, http)
	}

	if factory, ok := isUpToDateChecksFactoryMap[responseCheckAware.GetExpectedResponseCheck().GetType()]; ok {
		return factory(localKube, logger, http)
	}
	return isUpToDateChecksFactoryMap[common.ExpectedResponseCheckTypeDefault](localKube, logger, http)
}
