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

	apicommon "github.com/crossplane-contrib/provider-http/apis/common"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Re-export common constants for backward compatibility
const (
	ExpectedResponseCheckTypeDefault = apicommon.ExpectedResponseCheckTypeDefault
	ExpectedResponseCheckTypeCustom  = apicommon.ExpectedResponseCheckTypeCustom
)

const (
	ActionCreate  = apicommon.ActionCreate
	ActionObserve = apicommon.ActionObserve
	ActionUpdate  = apicommon.ActionUpdate
	ActionRemove  = apicommon.ActionRemove
)

// RequestParameters are the configurable fields of a Request.
// +kubebuilder:validation:XValidation:rule="!(self.insecureSkipTLSVerify == true && has(self.tlsConfig))",message="insecureSkipTLSVerify and tlsConfig are mutually exclusive"
type RequestParameters struct {
	// Mappings defines the HTTP mappings for different methods.
	// Either Method or Action must be specified. If both are omitted, the mapping will not be used.
	// +kubebuilder:validation:MinItems=1
	Mappings []Mapping `json:"mappings"`

	// Payload defines the payload for the request.
	Payload Payload `json:"payload"`

	// Headers defines default headers for each request.
	Headers map[string][]string `json:"headers,omitempty"`

	// WaitTimeout specifies the maximum time duration for waiting.
	WaitTimeout *metav1.Duration `json:"waitTimeout,omitempty"`

	// InsecureSkipTLSVerify, when set to true, skips TLS certificate checks for the HTTP request.
	// This field is mutually exclusive with TLSConfig.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// TLSConfig allows overriding the TLS configuration from ProviderConfig for this specific request.
	// This field is mutually exclusive with InsecureSkipTLSVerify.
	// +optional
	TLSConfig *common.TLSConfig `json:"tlsConfig,omitempty"`

	// SecretInjectionConfig specifies the secrets receiving patches for response data.
	SecretInjectionConfigs []apicommon.SecretInjectionConfig `json:"secretInjectionConfigs,omitempty"`

	// ExpectedResponseCheck specifies the mechanism to validate the OBSERVE response against expected value.
	ExpectedResponseCheck ExpectedResponseCheck `json:"expectedResponseCheck,omitempty"`

	// IsRemovedCheck specifies the mechanism to validate the OBSERVE response after removal against expected value.
	IsRemovedCheck ExpectedResponseCheck `json:"isRemovedCheck,omitempty"`
}

type Mapping struct {
	// +kubebuilder:validation:Enum=POST;GET;PUT;DELETE;PATCH;HEAD;OPTIONS
	// Method specifies the HTTP method for the request.
	Method string `json:"method,omitempty"`

	// +kubebuilder:validation:Enum=CREATE;OBSERVE;UPDATE;REMOVE
	// Action specifies the intended action for the request.
	Action string `json:"action,omitempty"`

	// Body specifies the body of the request.
	Body string `json:"body,omitempty"`

	// URL specifies the URL for the request.
	URL string `json:"url"`

	// Headers specifies the headers for the request.
	Headers map[string][]string `json:"headers,omitempty"`
}

type ExpectedResponseCheck struct {
	// Type specifies the type of the expected response check.
	// +kubebuilder:validation:Enum=DEFAULT;CUSTOM
	Type string `json:"type,omitempty"`

	// Logic specifies the custom logic for the expected response check.
	Logic string `json:"logic,omitempty"`
}

type Payload struct {
	// BaseUrl specifies the base URL for the request.
	BaseUrl string `json:"baseUrl,omitempty"`

	// Body specifies data to be used in the request body.
	Body string `json:"body,omitempty"`
}

// A RequestSpec defines the desired state of a Request.
type RequestSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       RequestParameters `json:"forProvider"`
}

// RequestObservation are the observable fields of a Request.
type Response struct {
	StatusCode int                 `json:"statusCode,omitempty"`
	Body       string              `json:"body,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
}

// A RequestStatus represents the observed state of a Request.
type RequestStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	Response            Response `json:"response,omitempty"`
	Cache               Cache    `json:"cache,omitempty"`
	Failed              int32    `json:"failed,omitempty"`
	Error               string   `json:"error,omitempty"`
	RequestDetails      Mapping  `json:"requestDetails,omitempty"`
}

type Cache struct {
	LastUpdated string   `json:"lastUpdated,omitempty"`
	Response    Response `json:"response,omitempty"`
}

// +kubebuilder:object:root=true

// A Request is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,http}
// +kubebuilder:storageversion
type Request struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RequestSpec   `json:"spec"`
	Status RequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RequestList contains a list of Request
type RequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Request `json:"items"`
}

// Request type metadata.
var (
	RequestKind             = reflect.TypeOf(Request{}).Name()
	RequestGroupKind        = schema.GroupKind{Group: Group, Kind: RequestKind}.String()
	RequestKindAPIVersion   = RequestKind + "." + SchemeGroupVersion.String()
	RequestGroupVersionKind = SchemeGroupVersion.WithKind(RequestKind)
)

func init() {
	SchemeBuilder.Register(&Request{}, &RequestList{})
}
