package disposablerequest

import (
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/pkg/errors"
)

const (
	ErrExpectedFormat  = "JQ filter should return a boolean, but returned error: %s"
	errConvertResToMap = "failed to convert response to map"
)

// IsResponseAsExpected checks if the response matches the expected criteria defined in the spec
func IsResponseAsExpected(spec interfaces.SimpleHTTPRequestSpec, res httpClient.HttpResponse) (bool, error) {
	// If no expected response is defined, consider it as expected.
	if spec.GetExpectedResponse() == "" {
		return true, nil
	}

	if res.StatusCode == 0 {
		return false, nil
	}

	responseMap, err := json_util.StructToMap(res)
	if err != nil {
		return false, errors.Wrap(err, errConvertResToMap)
	}

	json_util.ConvertJSONStringsToMaps(&responseMap)

	isExpected, err := jq.ParseBool(spec.GetExpectedResponse(), responseMap)
	if err != nil {
		return false, errors.Errorf(ErrExpectedFormat, err.Error())
	}

	return isExpected, nil
}
