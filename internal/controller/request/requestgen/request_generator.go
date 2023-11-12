package requestgen

import (
	"errors"
	"fmt"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/controller/request/requestmapping"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	errEmptyAndAbort = " is empty! aborting"
	urlForMapping    = "url for mapping "
)

type RequestDetails struct {
	Url     string
	Body    string
	Headers map[string][]string
}

func generateURL(urlJQFilter string, cr *v1alpha1.Request, logger logging.Logger) (string, error) {
	getURL, err := requestmapping.ApplyJQOnStr(urlJQFilter, cr, logger)
	if err != nil {
		return "", err
	}

	return getURL, nil
}

func generateBody(mappingBody string, cr *v1alpha1.Request, logger logging.Logger) (string, error) {
	jqQuery := requestmapping.ConvertStringToJQQuery(mappingBody)
	body, err := requestmapping.ApplyJQOnStr(jqQuery, cr, logger)
	if err != nil {
		return "", err
	}

	return body, nil
}

func generateHeaders(headers map[string][]string, cr *v1alpha1.Request, logger logging.Logger) (map[string][]string, error) {
	generatedHeaders, err := requestmapping.ApplyJQOnMap(headers, cr, logger)
	if err != nil {
		return nil, err
	}
	return generatedHeaders, nil
}

func GenerateValidRequestDetails(methodMapping v1alpha1.Mapping, cr *v1alpha1.Request, logger logging.Logger) (RequestDetails, error) {
	url, err := generateURL(methodMapping.URL, cr, logger)
	if err != nil {
		return RequestDetails{}, err
	}

	if url == "" {
		return RequestDetails{}, errors.New(fmt.Sprint(urlForMapping, methodMapping.Method, errEmptyAndAbort))
	}

	body, err := generateBody(methodMapping.Body, cr, logger)
	if err != nil {
		return RequestDetails{}, err
	}

	// If mapping contains headers, the func will use them. if not, the func will use forProvider headers by default.
	requestHeaders := methodMapping.Headers
	if requestHeaders == nil {
		requestHeaders = cr.Spec.ForProvider.Headers
	}

	headers, err := generateHeaders(requestHeaders, cr, logger)
	if err != nil {
		return RequestDetails{}, err
	}

	return RequestDetails{
		Body:    body,
		Url:     url,
		Headers: headers,
	}, nil
}
