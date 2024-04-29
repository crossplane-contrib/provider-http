package request

import (
	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
)

func getMappingByMethod(requestParams *v1alpha2.RequestParameters, method string) (*v1alpha2.Mapping, bool) {
	for _, mapping := range requestParams.Mappings {
		if mapping.Method == method {
			return &mapping, true
		}
	}
	return nil, false
}
