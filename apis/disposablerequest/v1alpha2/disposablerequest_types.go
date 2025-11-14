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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane-contrib/provider-http/apis/common"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// DisposableRequestParameters are the configurable fields of a DisposableRequest.
type DisposableRequestParameters struct {
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Field 'forProvider.url' is immutable"
	URL string `json:"url"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Field 'forProvider.method' is immutable"
	Method string `json:"method"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Field 'forProvider.headers' is immutable"
	Headers map[string][]string `json:"headers,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Field 'forProvider.body' is immutable"
	Body string `json:"body,omitempty"`

	// WaitTimeout specifies the maximum time duration for waiting.
	WaitTimeout *metav1.Duration `json:"waitTimeout,omitempty"`

	// RollbackRetriesLimit is max number of attempts to retry HTTP request by sending again the request.
	RollbackRetriesLimit *int32 `json:"rollbackRetriesLimit,omitempty"`

	// InsecureSkipTLSVerify, when set to true, skips TLS certificate checks for the HTTP request
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// TLSConfig allows overriding the TLS configuration from ProviderConfig for this specific request.
	// +optional
	TLSConfig *common.TLSConfig `json:"tlsConfig,omitempty"`

	// ExpectedResponse is a jq filter expression used to evaluate the HTTP response and determine if it matches the expected criteria.
	// The expression should return a boolean; if true, the response is considered expected.
	// Example: '.body.job_status == "success"'
	ExpectedResponse string `json:"expectedResponse,omitempty"`

	// NextReconcile specifies the duration after which the next reconcile should occur.
	NextReconcile *metav1.Duration `json:"nextReconcile,omitempty"`

	// ShouldLoopInfinitely specifies whether the reconciliation should loop indefinitely.
	ShouldLoopInfinitely bool `json:"shouldLoopInfinitely,omitempty"`

	// SecretInjectionConfig specifies the secrets receiving patches from response data.
	SecretInjectionConfigs []common.SecretInjectionConfig `json:"secretInjectionConfigs,omitempty"`
}

// A DisposableRequestSpec defines the desired state of a DisposableRequest.
type DisposableRequestSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       DisposableRequestParameters `json:"forProvider"`
}

type Response struct {
	StatusCode int                 `json:"statusCode,omitempty"`
	Body       string              `json:"body,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
}

type Mapping struct {
	Method  string              `json:"method"`
	Body    string              `json:"body,omitempty"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
}

// A DisposableRequestStatus represents the observed state of a DisposableRequest.
type DisposableRequestStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	Response            Response `json:"response,omitempty"`
	Failed              int32    `json:"failed,omitempty"`
	Error               string   `json:"error,omitempty"`
	Synced              bool     `json:"synced,omitempty"`
	RequestDetails      Mapping  `json:"requestDetails,omitempty"`

	// LastReconcileTime records the last time the resource was reconciled.
	LastReconcileTime metav1.Time `json:"lastReconcileTime,omitempty"`
}

// +kubebuilder:object:root=true

// A DisposableRequest is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,http}
// +kubebuilder:storageversion
type DisposableRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DisposableRequestSpec   `json:"spec"`
	Status DisposableRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DisposableRequestList contains a list of DisposableRequest
type DisposableRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DisposableRequest `json:"items"`
}

// DisposableRequest type metadata.
var (
	DisposableRequestKind             = reflect.TypeOf(DisposableRequest{}).Name()
	DisposableRequestGroupKind        = schema.GroupKind{Group: Group, Kind: DisposableRequestKind}.String()
	DisposableRequestKindAPIVersion   = DisposableRequestKind + "." + SchemeGroupVersion.String()
	DisposableRequestGroupVersionKind = SchemeGroupVersion.WithKind(DisposableRequestKind)
)

func init() {
	SchemeBuilder.Register(&DisposableRequest{}, &DisposableRequestList{})
}
