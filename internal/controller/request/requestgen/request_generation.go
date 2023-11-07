package requestgen

import (
	"net/http"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/controller/request/jsonmapping"
	httpRequestsUtils "github.com/arielsepton/provider-http/internal/utils/http_request_utils"
)

type RequestDetails struct {
	Url  string
	Body string
}

func GenerateURL(urlJQFilter string, cr *v1alpha1.Request) (string, error) {
	// TODO (REl): implement
	return "", nil
}

func GenerateBody(mappingBody string, cr *v1alpha1.Request) (string, error) {
	if mappingBody == "" {
		return "", nil
	}

	jqQuery, err := jsonmapping.CreateJQQuery(mappingBody)
	if err != nil {
		return "", err
	}

	body, err := jsonmapping.ApplyGoJQ(jqQuery, cr)
	if err != nil {
		return "", err
	}

	return body, nil
}

func GenerateValidRequestDetails(methodMapping v1alpha1.Mapping, cr *v1alpha1.Request) (RequestDetails, error) {
	url, err := GenerateURL(methodMapping.URL, cr)
	if err != nil {
		return RequestDetails{}, err
	}

	if err := httpRequestsUtils.IsRequestValid(http.MethodPost, url); err != nil {
		return RequestDetails{}, err
	}

	body, err := GenerateBody(methodMapping.Body, cr)
	if err != nil {
		return RequestDetails{}, err
	}

	return RequestDetails{
		Body: body,
		Url:  url,
	}, nil
}
