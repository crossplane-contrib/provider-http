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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
)

// Ensure RequestParameters implements MappedHTTPRequestSpec
var _ interfaces.MappedHTTPRequestSpec = (*RequestParameters)(nil)

// GetWaitTimeout returns the maximum time duration for waiting.
func (r *RequestParameters) GetWaitTimeout() *metav1.Duration {
	return r.WaitTimeout
}

// GetInsecureSkipTLSVerify returns whether to skip TLS certificate verification.
func (r *RequestParameters) GetInsecureSkipTLSVerify() bool {
	return r.InsecureSkipTLSVerify
}

// GetSecretInjectionConfigs returns the secret injection configurations.
// v1alpha1 does not support secret injection, so this returns nil.
func (r *RequestParameters) GetSecretInjectionConfigs() []common.SecretInjectionConfig {
	return nil
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
// v1alpha1 does not support actions, so this returns an empty string.
func (m *Mapping) GetAction() string {
	return ""
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
