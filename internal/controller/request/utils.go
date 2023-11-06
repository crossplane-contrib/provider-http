package request

import (
	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
)

func getMappingByMethod(requestParams *v1alpha1.RequestParameters, method string) (*v1alpha1.Mapping, bool) {
	for _, mapping := range requestParams.Mappings {
		if mapping.Method == method {
			return &mapping, true
		}
	}
	return nil, false // Method not found, TODO (REL): implement this case
}
