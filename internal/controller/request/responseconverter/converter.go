package responseconverter

import (
	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
)

// Convert HttpResponse to Response
func HttpResponseToV1alpha1Response(httpResponse httpClient.HttpResponse) v1alpha2.Response {
	return v1alpha2.Response{
		StatusCode: httpResponse.StatusCode,
		Body:       httpResponse.Body,
		Headers:    httpResponse.Headers,
	}
}
