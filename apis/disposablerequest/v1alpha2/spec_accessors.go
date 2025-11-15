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

// Ensure DisposableRequestParameters implements SimpleHTTPRequestSpec
var _ interfaces.SimpleHTTPRequestSpec = (*DisposableRequestParameters)(nil)

// Ensure DisposableRequestParameters implements ReconciliationPolicyAware
var _ interfaces.ReconciliationPolicyAware = (*DisposableRequestParameters)(nil)

// Ensure DisposableRequestParameters implements RollbackAware
var _ interfaces.RollbackAware = (*DisposableRequestParameters)(nil)

// GetWaitTimeout returns the maximum time duration for waiting.
func (d *DisposableRequestParameters) GetWaitTimeout() *metav1.Duration {
	return d.WaitTimeout
}

// GetInsecureSkipTLSVerify returns whether to skip TLS certificate verification.
func (d *DisposableRequestParameters) GetInsecureSkipTLSVerify() bool {
	return d.InsecureSkipTLSVerify
}

// GetSecretInjectionConfigs returns the secret injection configurations.
func (d *DisposableRequestParameters) GetSecretInjectionConfigs() []common.SecretInjectionConfig {
	return d.SecretInjectionConfigs
}

// GetHeaders returns the default headers for the request.
func (d *DisposableRequestParameters) GetHeaders() map[string][]string {
	return d.Headers
}

// GetURL returns the URL for the request.
func (d *DisposableRequestParameters) GetURL() string {
	return d.URL
}

// GetMethod returns the HTTP method for the request.
func (d *DisposableRequestParameters) GetMethod() string {
	return d.Method
}

// GetBody returns the body of the request.
func (d *DisposableRequestParameters) GetBody() string {
	return d.Body
}

// GetExpectedResponse returns the jq filter expression for validating the response.
func (d *DisposableRequestParameters) GetExpectedResponse() string {
	return d.ExpectedResponse
}

// GetNextReconcile returns the duration after which the next reconcile should occur.
func (d *DisposableRequestParameters) GetNextReconcile() *metav1.Duration {
	return d.NextReconcile
}

// GetShouldLoopInfinitely returns whether reconciliation should loop indefinitely.
func (d *DisposableRequestParameters) GetShouldLoopInfinitely() bool {
	return d.ShouldLoopInfinitely
}

// GetRollbackRetriesLimit returns the maximum number of rollback retry attempts.
func (d *DisposableRequestParameters) GetRollbackRetriesLimit() *int32 {
	return d.RollbackRetriesLimit
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

// Ensure DisposableRequest implements CachedResponse
var _ interfaces.CachedResponse = (*DisposableRequest)(nil)

// GetCachedResponse returns the cached response from the status.
func (d *DisposableRequest) GetCachedResponse() interfaces.HTTPResponse {
	if d.Status.Response.StatusCode == 0 {
		return nil
	}
	return &d.Status.Response
}

// GetSynced returns whether the resource is synced.
func (d *DisposableRequest) GetSynced() bool {
	return d.Status.Synced
}

// GetFailed returns the failure count.
func (d *DisposableRequest) GetFailed() int32 {
	return d.Status.Failed
}

// GetResponse returns the HTTP response from status.
func (d *DisposableRequest) GetResponse() interfaces.HTTPResponse {
	return &d.Status.Response
}

// SetFailed sets the failure count.
func (d *DisposableRequest) SetFailed(failed int32) {
	d.Status.Failed = failed
}

// Ensure DisposableRequest implements DisposableRequestStatus
var _ interfaces.DisposableRequestStatus = (*DisposableRequest)(nil)

// Ensure DisposableRequest implements DisposableRequestResource
var _ interfaces.DisposableRequestResource = (*DisposableRequest)(nil)

// GetSpec returns the request specification (ForProvider parameters).
func (d *DisposableRequest) GetSpec() interfaces.SimpleHTTPRequestSpec {
	return &d.Spec.ForProvider
}
