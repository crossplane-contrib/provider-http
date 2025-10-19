package requestmapping

import (
	"fmt"
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/pkg/errors"
)

const (
	ErrMappingNotFound = "%s or %s mapping doesn't exist in request, skipping operation"
)

var (
	// actionToMathodFactoryMap maps action to the default corresponding HTTP method.
	actionToMathodFactoryMap = map[string]string{
		v1alpha2.ActionCreate:  http.MethodPost,
		v1alpha2.ActionObserve: http.MethodGet,
		v1alpha2.ActionUpdate:  http.MethodPut,
		v1alpha2.ActionRemove:  http.MethodDelete,
	}
)

// getMappingByMethod returns the mapping for the given method from the request parameters.
func getMappingByMethod(requestParams *v1alpha2.RequestParameters, method string) (*v1alpha2.Mapping, bool) {
	for _, mapping := range requestParams.Mappings {
		if mapping.Method == method {
			return &mapping, true
		}
	}
	return nil, false
}

// getMappingByAction returns the mapping for the given action from the request parameters.
func getMappingByAction(requestParams *v1alpha2.RequestParameters, action string) (*v1alpha2.Mapping, bool) {
	for _, mapping := range requestParams.Mappings {
		if mapping.Action == action {
			return &mapping, true
		}
	}
	return nil, false
}

// GetMapping retrieves the mapping based on the provided request parameters, method, and action.
// It first attempts to find the mapping by the specified action. If found, it sets the method if it's not defined.
// If no action is specified or the mapping by action is not found, it falls back to finding the mapping by the default method.
func GetMapping(requestParams *v1alpha2.RequestParameters, action string, logger logging.Logger) (*v1alpha2.Mapping, error) {
	method := getDefaultMethodByAction(action)
	if mapping, found := getMappingByAction(requestParams, action); found {
		if mapping.Method == "" {
			mapping.Method = method
		}
		return mapping, nil
	}

	logger.Debug(fmt.Sprintf("Mapping not found for action %s, trying to find mapping by method %s", action, method))
	if mapping, found := getMappingByMethod(requestParams, method); found {
		return mapping, nil
	}

	return nil, errors.Errorf(ErrMappingNotFound, action, method)
}

// getDefaultMethodByAction returns the default HTTP method for the given action.
func getDefaultMethodByAction(action string) string {
	if defaultAction, ok := actionToMathodFactoryMap[action]; ok {
		return defaultAction
	}

	return http.MethodGet
}
