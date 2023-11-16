package requestgen

import (
	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/controller/request/requestprocessing"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

type RequestDetails struct {
	Url     string
	Body    string
	Headers map[string][]string
}

// GenerateValidRequestDetails generates valid request details.
func GenerateValidRequestDetails(methodMapping v1alpha1.Mapping, cr *v1alpha1.Request, logger logging.Logger) (RequestDetails, error) {
	url, err := generateURL(methodMapping.URL, cr, logger)
	if err != nil {
		return RequestDetails{}, err
	}

	body, err := generateBody(methodMapping.Body, cr, logger)
	if err != nil {
		return RequestDetails{}, err
	}

	headers, err := generateHeaders(coalesceHeaders(methodMapping.Headers, cr.Spec.ForProvider.Headers), cr, logger)
	if err != nil {
		return RequestDetails{}, err
	}

	return RequestDetails{Body: body, Url: url, Headers: headers}, nil
}

// coalesceHeaders returns the non-nil headers, or the default headers if both are nil.
func coalesceHeaders(mappingHeaders, defaultHeaders map[string][]string) map[string][]string {
	if mappingHeaders != nil {
		return mappingHeaders
	}
	return defaultHeaders
}

// generateURL applies a JQ filter to generate a URL.
func generateURL(urlJQFilter string, cr *v1alpha1.Request, logger logging.Logger) (string, error) {
	getURL, err := requestprocessing.ApplyJQOnStr(urlJQFilter, cr, logger)
	if err != nil {
		return "", err
	}

	return getURL, nil
}

// generateBody applies a mapping body to generate the request body.
func generateBody(mappingBody string, cr *v1alpha1.Request, logger logging.Logger) (string, error) {
	jqQuery := requestprocessing.ConvertStringToJQQuery(mappingBody)
	body, err := requestprocessing.ApplyJQOnStr(jqQuery, cr, logger)
	if err != nil {
		return "", err
	}

	return body, nil
}

// generateHeaders applies JQ queries to generate headers.
func generateHeaders(headers map[string][]string, cr *v1alpha1.Request, logger logging.Logger) (map[string][]string, error) {
	generatedHeaders, err := requestprocessing.ApplyJQOnMapStrings(headers, cr, logger)
	if err != nil {
		return nil, err
	}

	return generatedHeaders, nil
}
