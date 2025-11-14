package requestgen

import (
	"context"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestprocessing"
	"github.com/crossplane-contrib/provider-http/internal/utils"

	"golang.org/x/exp/maps"
)

type RequestDetails struct {
	Url     string
	Body    httpClient.Data
	Headers httpClient.Data
}

// GenerateRequestDetails generates request details.
func GenerateRequestDetails(ctx context.Context, localKube client.Client, methodMapping interfaces.HTTPMapping, forProvider interfaces.MappedHTTPRequestSpec, response interfaces.HTTPResponse, logger logging.Logger) (RequestDetails, error, bool) {
	patchedResponse, err := datapatcher.PatchSecretsIntoResponse(ctx, localKube, response, logger)
	if err != nil {
		return RequestDetails{}, err, false
	}

	jqObject := GenerateRequestContext(forProvider, patchedResponse)
	url, err := generateURL(methodMapping.GetURL(), jqObject)
	if err != nil {
		return RequestDetails{}, err, false
	}

	if !utils.IsUrlValid(url) {
		return RequestDetails{}, errors.Errorf(utils.ErrInvalidURL, url), false
	}

	body, err := generateBody(ctx, localKube, methodMapping.GetBody(), jqObject, logger)
	if err != nil {
		return RequestDetails{}, err, false
	}

	headersData, err := generateHeaders(ctx, localKube, coalesceHeaders(methodMapping, forProvider), jqObject, logger)
	if err != nil {
		return RequestDetails{}, err, false
	}

	return RequestDetails{Body: body, Url: url, Headers: headersData}, nil, true
}

// GenerateRequestContext creates a JSON-compatible map from the specified Request's ForProvider and Response fields.
// It merges the two maps, converts JSON strings to nested maps, and returns the resulting map.
func GenerateRequestContext(forProvider interfaces.MappedHTTPRequestSpec, patchedResponse interfaces.HTTPResponse) map[string]interface{} {
	baseMap, _ := json_util.StructToMap(forProvider)
	statusMap, _ := json_util.StructToMap(map[string]interface{}{
		"response": patchedResponse,
	})

	maps.Copy(baseMap, statusMap)
	json_util.ConvertJSONStringsToMaps(&baseMap)

	if responseMap, ok := baseMap["response"].(map[string]interface{}); ok {
		if _, exists := responseMap["headers"]; !exists {
			responseMap["headers"] = nil
		}
	}

	return baseMap
}

// GenerateValidRequestDetails generates valid request details based on the given Request resource and Mapping configuration.
// It first attempts to generate request details using the HTTP response stored in the Request's status. If the generated
// details are valid, the function returns them. If not, it falls back to using the cached response in the Request's status
// and attempts to generate request details again. The function returns the generated request details or an error if the
// generation process fails.
func GenerateValidRequestDetails(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, mapping interfaces.HTTPMapping, response interfaces.HTTPResponse, cachedResponse interfaces.HTTPResponse, localKube client.Client, logger logging.Logger) (RequestDetails, error) {
	requestDetails, _, ok := GenerateRequestDetails(ctx, localKube, mapping, spec, response, logger)
	if IsRequestValid(requestDetails) && ok {
		return requestDetails, nil
	}

	requestDetails, err, _ := GenerateRequestDetails(ctx, localKube, mapping, spec, cachedResponse, logger)
	if err != nil {
		return RequestDetails{}, err
	}

	return requestDetails, nil
}

// IsRequestValid checks if the request details are valid.
func IsRequestValid(requestDetails RequestDetails) bool {
	return (!strings.Contains(fmt.Sprint(requestDetails), "null")) && (requestDetails.Url != "")
}

// coalesceHeaders returns the non-nil headers, or the default headers if both are nil.
func coalesceHeaders(mapping interfaces.HTTPMapping, spec interfaces.HTTPRequestSpec) map[string][]string {
	if headers := mapping.GetHeaders(); headers != nil {
		return headers
	}
	return spec.GetHeaders()
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
func generateBody(ctx context.Context, localKube client.Client, mappingBody string, jqObject map[string]interface{}, logger logging.Logger) (httpClient.Data, error) {
	if mappingBody == "" {
		return httpClient.Data{
			Encrypted: "",
			Decrypted: "",
		}, nil
	}

	jqQuery := utils.NormalizeWhitespace(mappingBody)
	body, err := requestprocessing.ApplyJQOnStr(jqQuery, jqObject)
	if err != nil {
		return httpClient.Data{}, err
	}

	sensitiveBody, err := datapatcher.PatchSecretsIntoString(ctx, localKube, body, logger)
	if err != nil {
		return httpClient.Data{}, err
	}

	return httpClient.Data{
		Encrypted: body,
		Decrypted: sensitiveBody,
	}, nil
}

// generateHeaders applies JQ queries to generate headers.
func generateHeaders(ctx context.Context, localKube client.Client, headers map[string][]string, jqObject map[string]interface{}, logger logging.Logger) (httpClient.Data, error) {
	generatedHeaders, err := requestprocessing.ApplyJQOnMapStrings(headers, jqObject)
	if err != nil {
		return httpClient.Data{}, err
	}

	sensitiveHeaders, err := datapatcher.PatchSecretsIntoHeaders(ctx, localKube, generatedHeaders, logger)
	if err != nil {
		return httpClient.Data{}, err
	}

	return httpClient.Data{
		Encrypted: generatedHeaders,
		Decrypted: sensitiveHeaders,
	}, nil
}
