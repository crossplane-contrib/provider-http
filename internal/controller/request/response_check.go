package request

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/responseconverter"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	"github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/pkg/errors"
)

// ResponseCheck is an interface for performing response checks.
type ResponseCheck interface {
	Check(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (ObserveRequestDetails, error)
}

// DefaultResponseCheck performs a default comparison between the response and desired state.
type DefaultResponseCheck struct {
	client *external
}

func (d *DefaultResponseCheck) Check(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (ObserveRequestDetails, error) {
	desiredState, err := d.desiredState(ctx, cr)
	if err != nil {
		if isErrorMappingNotFound(err) {
			return NewObserve(details, responseErr, true), nil
		}
		return FailedObserve(), err
	}

	return d.compareResponseAndDesiredState(ctx, details, responseErr, desiredState)
}

// compareResponseAndDesiredState compares the response and desired state to determine if they are in sync.
func (d *DefaultResponseCheck) compareResponseAndDesiredState(ctx context.Context, details httpClient.HttpDetails, responseErr error, desiredState string) (ObserveRequestDetails, error) {
	observeRequestDetails := NewObserve(details, responseErr, false)

	sensitiveBody, err := d.patchAndValidate(ctx, details.HttpResponse.Body)
	if err != nil {
		return FailedObserve(), err
	}

	sensitiveDesiredState, err := d.patchAndValidate(ctx, desiredState)
	if err != nil {
		return FailedObserve(), err
	}

	synced, err := d.comparePatchedResults(sensitiveBody, sensitiveDesiredState, details.HttpResponse.StatusCode)
	if err != nil {
		return FailedObserve(), err
	}

	observeRequestDetails.Synced = synced
	return observeRequestDetails, nil
}

// patchAndValidate patches secrets into a string and validates the result.
func (d *DefaultResponseCheck) patchAndValidate(ctx context.Context, content string) (string, error) {
	patched, err := datapatcher.PatchSecretsIntoString(ctx, d.client.localKube, content, d.client.logger)
	if err != nil {
		return "", err
	}

	return patched, nil
}

// comparePatchedResults compares the patched response and desired state to determine if they are in sync.
func (d *DefaultResponseCheck) comparePatchedResults(body, desiredState string, statusCode int) (bool, error) {
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
func (d *DefaultResponseCheck) compareJSON(body, desiredState string, statusCode int) bool {
	responseBodyMap := json.JsonStringToMap(body)
	desiredStateMap := json.JsonStringToMap(desiredState)
	return json.Contains(responseBodyMap, desiredStateMap) && utils.IsHTTPSuccess(statusCode)
}

// desiredState returns the desired state for a given request
func (d *DefaultResponseCheck) desiredState(ctx context.Context, cr *v1alpha2.Request) (string, error) {
	requestDetails, err := d.client.requestDetails(ctx, cr, v1alpha2.ActionUpdate)
	if err != nil {
		return "", err
	}

	return requestDetails.Body.Encrypted.(string), nil
}

// CustomResponseCheck performs a custom response check using JQ logic.
type CustomResponseCheck struct {
	client *external
}

func (c *CustomResponseCheck) Check(ctx context.Context, cr *v1alpha2.Request, details httpClient.HttpDetails, responseErr error) (ObserveRequestDetails, error) {
	observeRequestDetails := NewObserve(details, responseErr, false)

	// Convert response to a map and apply JQ logic
	response := responseconverter.HttpResponseToV1alpha1Response(details.HttpResponse)
	responseMap := requestgen.GenerateRequestObject(cr.Spec.ForProvider, response)

	jqQuery := utils.NormalizeWhitespace(cr.Spec.ForProvider.ExpectedResponseCheck.Logic)
	sensitiveJQQuery, err := datapatcher.PatchSecretsIntoString(ctx, c.client.localKube, jqQuery, c.client.logger)
	if err != nil {
		return FailedObserve(), err
	}

	sensitiveResponse, err := datapatcher.PatchSecretsIntoMap(ctx, c.client.localKube, responseMap, c.client.logger)
	if err != nil {
		return FailedObserve(), err
	}

	jsonData, _ := json.ConvertMapToJson(responseMap)
	isExpected, err := jq.ParseBool(sensitiveJQQuery, sensitiveResponse)

	c.client.logger.Debug(fmt.Sprintf("Applying JQ filter %s on data %v, result is %v", jqQuery, jsonData, isExpected))
	if err != nil {
		return FailedObserve(), errors.Errorf(ErrExpectedFormat, err.Error())
	}

	observeRequestDetails.Synced = isExpected
	return observeRequestDetails, nil
}

// responseCheckFactoryMap is a map that associates each check type with its corresponding factory function.
var responseCheckFactoryMap = map[string]func(*external) ResponseCheck{
	v1alpha2.ExpectedResponseCheckTypeDefault: func(c *external) ResponseCheck { return &DefaultResponseCheck{client: c} },
	v1alpha2.ExpectedResponseCheckTypeCustom:  func(c *external) ResponseCheck { return &CustomResponseCheck{client: c} },
}

// getResponseCheck uses a map to select and return the appropriate ResponseCheck.
func (c *external) getResponseCheck(cr *v1alpha2.Request) ResponseCheck {
	if factory, ok := responseCheckFactoryMap[cr.Spec.ForProvider.ExpectedResponseCheck.Type]; ok {
		return factory(c)
	}
	return responseCheckFactoryMap[v1alpha2.ExpectedResponseCheckTypeDefault](c)
}

// isErrorMappingNotFound checks if the provided error indicates that the
// mapping for an HTTP PUT request is not found.
func isErrorMappingNotFound(err error) bool {
	return errors.Cause(err).Error() == fmt.Sprintf(requestmapping.ErrMappingNotFound, v1alpha2.ActionUpdate, http.MethodPut)
}
