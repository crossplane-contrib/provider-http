/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
)

// Ensure RequestParameters implements MappedHTTPRequestSpec
var _ interfaces.MappedHTTPRequestSpec = (*RequestParameters)(nil)

// Ensure RequestParameters implements ResponseCheckAware
var _ interfaces.ResponseCheckAware = (*RequestParameters)(nil)

// GetWaitTimeout returns the maximum time duration for waiting.
func (r *RequestParameters) GetWaitTimeout() *metav1.Duration {
	return r.WaitTimeout
}

// GetInsecureSkipTLSVerify returns whether to skip TLS certificate verification.
func (r *RequestParameters) GetInsecureSkipTLSVerify() bool {
	return r.InsecureSkipTLSVerify
}

// GetSecretInjectionConfigs returns the secret injection configurations.
func (r *RequestParameters) GetSecretInjectionConfigs() []common.SecretInjectionConfig {
	return r.SecretInjectionConfigs
}

// GetHeaders returns the default headers for the request.
func (r *RequestParameters) GetHeaders() map[string][]string {
	return r.Headers
}

// GetMappings returns the HTTP mappings for different methods/actions.
func (r *RequestParameters) GetMappings() []interfaces.HTTPMapping {
	result := make([]interfaces.HTTPMapping, len(r.Mappings))
	for i := range r.Mappings {
		result[i] = &r.Mappings[i]
	}
	return result
}

// GetPayload returns the payload configuration.
func (r *RequestParameters) GetPayload() interfaces.HTTPPayload {
	return &r.Payload
}

// GetExpectedResponseCheck returns the expected response check configuration.
func (r *RequestParameters) GetExpectedResponseCheck() interfaces.ResponseCheck {
	return &r.ExpectedResponseCheck
}

// GetIsRemovedCheck returns the is-removed check configuration.
func (r *RequestParameters) GetIsRemovedCheck() interfaces.ResponseCheck {
	return &r.IsRemovedCheck
}

// Ensure Mapping implements HTTPMapping
var _ interfaces.HTTPMapping = (*Mapping)(nil)

// GetMethod returns the HTTP method.
func (m *Mapping) GetMethod() string {
	return m.Method
}

// SetMethod sets the HTTP method.
func (m *Mapping) SetMethod(method string) {
	m.Method = method
}

// GetAction returns the action type.
func (m *Mapping) GetAction() string {
	return m.Action
}

// GetBody returns the body template for this mapping.
func (m *Mapping) GetBody() string {
	return m.Body
}

// GetURL returns the URL template for this mapping.
func (m *Mapping) GetURL() string {
	return m.URL
}

// GetHeaders returns the headers for this mapping.
func (m *Mapping) GetHeaders() map[string][]string {
	return m.Headers
}

// Ensure Payload implements HTTPPayload
var _ interfaces.HTTPPayload = (*Payload)(nil)

// GetBaseURL returns the base URL.
func (p *Payload) GetBaseURL() string {
	return p.BaseUrl
}

// GetBody returns the payload body.
func (p *Payload) GetBody() string {
	return p.Body
}

// Ensure ExpectedResponseCheck implements ResponseCheck
var _ interfaces.ResponseCheck = (*ExpectedResponseCheck)(nil)

// GetType returns the check type.
func (e *ExpectedResponseCheck) GetType() string {
	return e.Type
}

// GetLogic returns the custom logic for the check.
func (e *ExpectedResponseCheck) GetLogic() string {
	return e.Logic
}

// Ensure Response implements HTTPResponse
var _ interfaces.HTTPResponse = (*Response)(nil)

// GetStatusCode returns the HTTP status code.
func (r *Response) GetStatusCode() int {
	return r.StatusCode
}

// GetBody returns the response body.
func (r *Response) GetBody() string {
	return r.Body
}

// GetHeaders returns the response headers.
func (r *Response) GetHeaders() map[string][]string {
	return r.Headers
}

// Ensure Request implements CachedResponse
var _ interfaces.CachedResponse = (*Request)(nil)

// GetCachedResponse returns the cached response from the status.
func (r *Request) GetCachedResponse() interfaces.HTTPResponse {
	if r.Status.Response.StatusCode == 0 {
		return nil
	}
	return &r.Status.Response
}

// Ensure Request implements RequestStatusReader
var _ interfaces.RequestStatusReader = (*Request)(nil)

// GetResponse returns the HTTP response from status.
func (r *Request) GetResponse() interfaces.HTTPResponse {
	return &r.Status.Response
}

// GetFailed returns the failure count.
func (r *Request) GetFailed() int32 {
	return r.Status.Failed
}

// GetRequestDetails returns the request details mapping.
func (r *Request) GetRequestDetails() interfaces.HTTPMapping {
	return &r.Status.RequestDetails
}

// Ensure Request implements RequestResource
var _ interfaces.RequestResource = (*Request)(nil)

// GetSpec returns the request specification.
func (r *Request) GetSpec() interfaces.MappedHTTPRequestSpec {
	return &r.Spec.ForProvider
}

// Ensure Request implements RequestStatus (read + write + cached)
var _ interfaces.RequestStatus = (*Request)(nil)
