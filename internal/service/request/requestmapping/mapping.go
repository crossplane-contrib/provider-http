package requestmapping

import (
	"fmt"
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
)

const (
	ErrMappingNotFound = "%s or %s mapping doesn't exist in request, skipping operation"
)

var (
	// actionToMathodFactoryMap maps action to the default corresponding HTTP method.
	actionToMathodFactoryMap = map[string]string{
		common.ActionCreate:  http.MethodPost,
		common.ActionObserve: http.MethodGet,
		common.ActionUpdate:  http.MethodPut,
		common.ActionRemove:  http.MethodDelete,
	}
)

// getMappingByMethod returns the mapping for the given method from the request parameters.
func getMappingByMethod(requestParams interfaces.MappedHTTPRequestSpec, method string) (interfaces.HTTPMapping, bool) {
	for _, mapping := range requestParams.GetMappings() {
		if mapping.GetMethod() == method {
			return mapping, true
		}
	}
	return nil, false
}

// getMappingByAction returns the mapping for the given action from the request parameters.
func getMappingByAction(requestParams interfaces.MappedHTTPRequestSpec, action string) (interfaces.HTTPMapping, bool) {
	for _, mapping := range requestParams.GetMappings() {
		if mapping.GetAction() == action {
			return mapping, true
		}
	}
	return nil, false
}

// GetMapping retrieves the mapping based on the provided request parameters, method, and action.
// It first attempts to find the mapping by the specified action. If found, it sets the method if it's not defined.
// If no action is specified or the mapping by action is not found, it falls back to finding the mapping by the default method.
func GetMapping(requestParams interfaces.MappedHTTPRequestSpec, action string, logger logging.Logger) (interfaces.HTTPMapping, error) {
	method := getDefaultMethodByAction(action)
	if mapping, found := getMappingByAction(requestParams, action); found {
		if mapping.GetMethod() == "" {
			mapping.SetMethod(method)
		}
		return mapping, nil
	}

	logger.Debug(fmt.Sprintf("Mapping not found for action %s, trying to find mapping by method %s", action, method))
	if mapping, found := getMappingByMethod(requestParams, method); found {
		return mapping, nil
	}

	return nil, errors.Errorf(ErrMappingNotFound, action, method)
}

// GetEffectiveMethod returns the effective HTTP method for a mapping.
// If the mapping has a method defined, it returns that. Otherwise, it derives the method from the action.
func GetEffectiveMethod(mapping interfaces.HTTPMapping) string {
	if method := mapping.GetMethod(); method != "" {
		return method
	}
	return getDefaultMethodByAction(mapping.GetAction())
}

// getDefaultMethodByAction returns the default HTTP method for the given action.
func getDefaultMethodByAction(action string) string {
	if defaultAction, ok := actionToMathodFactoryMap[action]; ok {
		return defaultAction
	}

	return http.MethodGet
}
