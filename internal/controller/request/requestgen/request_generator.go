package requestgen

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha1"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestprocessing"
	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/crossplane-contrib/provider-http/internal/utils"

	"golang.org/x/exp/maps"
)

type RequestDetails struct {
	Url     string
	Body    string
	Headers map[string][]string
}

// GenerateRequestDetails generates request details.
func GenerateRequestDetails(methodMapping v1alpha1.Mapping, forProvider v1alpha1.RequestParameters, response v1alpha1.Response) (RequestDetails, error, bool) {
	jqObject := generateRequestObject(forProvider, response)
	url, err := generateURL(methodMapping.URL, jqObject)
	if err != nil {
		return RequestDetails{}, err, false
	}

	if !utils.IsUrlValid(url) {
		return RequestDetails{}, errors.Errorf(utils.ErrInvalidURL, url), false
	}

	body, err := generateBody(methodMapping.Body, jqObject)
	if err != nil {
		return RequestDetails{}, err, false
	}

	headers, err := generateHeaders(coalesceHeaders(methodMapping.Headers, forProvider.Headers), jqObject)
	if err != nil {
		return RequestDetails{}, err, false
	}

	return RequestDetails{Body: body, Url: url, Headers: headers}, nil, true
}

// generateRequestObject creates a JSON-compatible map from the specified Request's ForProvider and Response fields.
// It merges the two maps, converts JSON strings to nested maps, and returns the resulting map.
func generateRequestObject(forProvider v1alpha1.RequestParameters, response v1alpha1.Response) map[string]interface{} {
	baseMap, _ := json_util.StructToMap(forProvider)
	statusMap, _ := json_util.StructToMap(map[string]interface{}{
		"response": response,
	})

	maps.Copy(baseMap, statusMap)
	json_util.ConvertJSONStringsToMaps(&baseMap)

	return baseMap
}

func IsRequestValid(requestDetails RequestDetails) bool {
	return (!strings.Contains(fmt.Sprint(requestDetails), "null")) && (requestDetails.Url != "")
}

// coalesceHeaders returns the non-nil headers, or the default headers if both are nil.
func coalesceHeaders(mappingHeaders, defaultHeaders map[string][]string) map[string][]string {
	if mappingHeaders != nil {
		return mappingHeaders
	}
	return defaultHeaders
}

// generateURL applies a JQ filter to generate a URL.
func generateURL(urlJQFilter string, jqObject map[string]interface{}) (string, error) {
	getURL, err := requestprocessing.ApplyJQOnStr(urlJQFilter, jqObject)
	if err != nil {
		return "", err
	}

	return getURL, nil
}

// generateBody applies a mapping body to generate the request body.
func generateBody(mappingBody string, jqObject map[string]interface{}) (string, error) {
	if mappingBody == "" {
		return "", nil
	}

	jqQuery := requestprocessing.ConvertStringToJQQuery(mappingBody)
	body, err := requestprocessing.ApplyJQOnStr(jqQuery, jqObject)
	if err != nil {
		return "", err
	}

	return body, nil
}

// generateHeaders applies JQ queries to generate headers.
func generateHeaders(headers map[string][]string, jqObject map[string]interface{}) (map[string][]string, error) {
	generatedHeaders, err := requestprocessing.ApplyJQOnMapStrings(headers, jqObject)
	if err != nil {
		return nil, err
	}

	return generatedHeaders, nil
}
