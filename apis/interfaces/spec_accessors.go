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

package interfaces

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-http/apis/common"
)

// HTTPRequestSpec defines the common interface for accessing HTTP request configuration.
// This interface abstracts the differences between Request and DisposableRequest types.
type HTTPRequestSpec interface {
	// GetWaitTimeout returns the maximum time duration for waiting.
	GetWaitTimeout() *metav1.Duration

	// GetInsecureSkipTLSVerify returns whether to skip TLS certificate verification.
	GetInsecureSkipTLSVerify() bool

	// GetSecretInjectionConfigs returns the secret injection configurations.
	GetSecretInjectionConfigs() []common.SecretInjectionConfig

	// GetHeaders returns the default headers for the request.
	GetHeaders() map[string][]string
}

// SimpleHTTPRequestSpec defines the interface for simple HTTP requests (like DisposableRequest).
// These requests have a single URL, method, and body rather than multiple mappings.
type SimpleHTTPRequestSpec interface {
	HTTPRequestSpec

	// GetURL returns the URL for the request.
	GetURL() string

	// GetMethod returns the HTTP method for the request.
	GetMethod() string

	// GetBody returns the body of the request.
	GetBody() string

	// GetExpectedResponse returns the jq filter expression for validating the response.
	GetExpectedResponse() string
}

// MappedHTTPRequestSpec defines the interface for requests with multiple mappings (like Request).
// These requests can have different URLs and methods for different actions.
type MappedHTTPRequestSpec interface {
	HTTPRequestSpec

	// GetMappings returns the HTTP mappings for different methods/actions.
	GetMappings() []HTTPMapping

	// GetPayload returns the payload configuration.
	GetPayload() HTTPPayload
}

// HTTPMapping represents a single HTTP mapping configuration.
type HTTPMapping interface {
	// GetMethod returns the HTTP method (POST, GET, PUT, DELETE, PATCH, HEAD, OPTIONS).
	GetMethod() string

	// SetMethod sets the HTTP method.
	SetMethod(method string)

	// GetAction returns the action type (CREATE, OBSERVE, UPDATE, REMOVE).
	// Returns empty string if not applicable.
	GetAction() string

	// GetBody returns the body template for this mapping.
	GetBody() string

	// GetURL returns the URL template for this mapping.
	GetURL() string

	// GetHeaders returns the headers for this mapping.
	GetHeaders() map[string][]string
}

// HTTPPayload represents the payload configuration.
type HTTPPayload interface {
	// GetBaseURL returns the base URL.
	GetBaseURL() string

	// GetBody returns the payload body.
	GetBody() string
}

// ResponseCheckAware indicates that a spec supports custom response validation.
// This is a v1alpha2-specific feature.
type ResponseCheckAware interface {
	// GetExpectedResponseCheck returns the expected response check configuration.
	GetExpectedResponseCheck() ResponseCheck

	// GetIsRemovedCheck returns the is-removed check configuration.
	GetIsRemovedCheck() ResponseCheck
}

// ResponseCheck represents a response validation check.
type ResponseCheck interface {
	// GetType returns the check type (DEFAULT or CUSTOM).
	GetType() string

	// GetLogic returns the custom logic for the check (jq expression).
	GetLogic() string
}

// ReconciliationPolicyAware indicates that a spec supports custom reconciliation policies.
// This is a v1alpha2 DisposableRequest-specific feature.
type ReconciliationPolicyAware interface {
	// GetNextReconcile returns the duration after which the next reconcile should occur.
	GetNextReconcile() *metav1.Duration

	// GetShouldLoopInfinitely returns whether reconciliation should loop indefinitely.
	GetShouldLoopInfinitely() bool
}

// RollbackAware indicates that a spec supports rollback retries.
// This is a v1alpha2 DisposableRequest-specific feature.
type RollbackAware interface {
	// GetRollbackRetriesLimit returns the maximum number of rollback retry attempts.
	GetRollbackRetriesLimit() *int32
}

// HTTPResponse represents the common interface for HTTP response data.
type HTTPResponse interface {
	// GetStatusCode returns the HTTP status code.
	GetStatusCode() int

	// GetBody returns the response body.
	GetBody() string

	// GetHeaders returns the response headers.
	GetHeaders() map[string][]string
}

// CachedResponse represents a response that can be retrieved from cache.
type CachedResponse interface {
	// GetCachedResponse returns the cached response.
	GetCachedResponse() HTTPResponse
}

// DisposableRequestStatusReader provides read-only access to DisposableRequest status fields.
type DisposableRequestStatusReader interface {
	// GetSynced returns whether the resource is synced.
	GetSynced() bool

	// GetFailed returns the number of failed attempts.
	GetFailed() int32

	// GetResponse returns the HTTP response.
	GetResponse() HTTPResponse
}

// BaseStatusWriter provides common status modification methods shared by both Request and DisposableRequest.
// This interface defines the core status update operations that all resources support.
type BaseStatusWriter interface {
	// SetStatusCode sets the HTTP status code.
	SetStatusCode(statusCode int)

	// SetHeaders sets the response headers.
	SetHeaders(headers map[string][]string)

	// SetBody sets the response body.
	SetBody(body string)

	// SetError sets the error message.
	SetError(err error)

	// SetRequestDetails sets the request details.
	SetRequestDetails(url, method, body string, headers map[string][]string)
}

// DisposableRequestStatusWriter provides write access to DisposableRequest status fields.
type DisposableRequestStatusWriter interface {
	BaseStatusWriter

	// SetSynced sets the synced status.
	SetSynced(synced bool)

	// SetFailed sets the number of failed attempts.
	SetFailed(failed int32)

	// SetLastReconcileTime sets the last reconcile time.
	SetLastReconcileTime()
}

// DisposableRequestStatus combines read and write access to DisposableRequest status.
type DisposableRequestStatus interface {
	DisposableRequestStatusReader
	DisposableRequestStatusWriter
}

// RequestStatusReader provides read-only access to Request status fields.
type RequestStatusReader interface {
	// GetResponse returns the HTTP response.
	GetResponse() HTTPResponse

	// GetFailed returns the number of failed attempts.
	GetFailed() int32

	// GetRequestDetails returns the request details mapping.
	GetRequestDetails() HTTPMapping
}

// RequestStatusWriter provides write access to Request status fields.
type RequestStatusWriter interface {
	BaseStatusWriter

	// SetCache sets the cached response.
	SetCache(statusCode int, headers map[string][]string, body string)

	// ResetFailures resets the failure count.
	ResetFailures()
}

// RequestStatus combines read and write access to Request status.
type RequestStatus interface {
	RequestStatusReader
	RequestStatusWriter
	CachedResponse
}

// RequestResource represents a complete Request resource with both spec and status.
// This interface is implemented by Request types and provides access to both configuration and state.
type RequestResource interface {
	// Embed client.Object for Kubernetes object access (includes metav1.Object and runtime.Object)
	client.Object

	// GetSpec returns the request specification (ForProvider parameters).
	GetSpec() MappedHTTPRequestSpec

	// RequestStatusReader for accessing response and status information
	RequestStatusReader

	// RequestStatusWriter for modifying response and status information
	RequestStatusWriter

	// CachedResponse for accessing cached response
	CachedResponse
}

// DisposableRequestResource represents a complete DisposableRequest resource with both spec and status.
// This interface is implemented by DisposableRequest types and provides access to both configuration and state.
type DisposableRequestResource interface {
	// Embed client.Object for Kubernetes object access (includes metav1.Object and runtime.Object)
	client.Object

	// GetSpec returns the request specification (ForProvider parameters).
	// For DisposableRequest, the spec also serves as the RollbackAware interface.
	GetSpec() SimpleHTTPRequestSpec

	// DisposableRequestStatusReader for accessing response and status information
	DisposableRequestStatusReader

	// DisposableRequestStatusWriter for modifying response and status information
	DisposableRequestStatusWriter
}
