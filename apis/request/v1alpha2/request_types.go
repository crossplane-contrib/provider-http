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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// RequestParameters are the configurable fields of a Request.
type RequestParameters struct {
	// Mappings defines the HTTP mappings for different methods.
	Mappings []Mapping `json:"mappings"`

	// Payload defines the payload for the request.
	Payload Payload `json:"payload"`

	// Headers defines default headers for each request.
	Headers map[string][]string `json:"headers,omitempty"`

	// WaitTimeout specifies the maximum time duration for waiting.
	WaitTimeout *metav1.Duration `json:"waitTimeout,omitempty"`

	// InsecureSkipTLSVerify, when set to true, skips TLS certificate checks for the HTTP request
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// SecretInjectionConfig specifies the secrets receiving patches for response data.
	SecretInjectionConfigs []SecretInjectionConfig `json:"secretInjectionConfigs,omitempty"`
}

type Mapping struct {
	// +kubebuilder:validation:Enum=POST;GET;PUT;DELETE
	Method  string              `json:"method"`
	Body    string              `json:"body,omitempty"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type Payload struct {
	BaseUrl string `json:"baseUrl,omitempty"`
	Body    string `json:"body,omitempty"`
}

// A RequestSpec defines the desired state of a Request.
type RequestSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       RequestParameters `json:"forProvider"`
}

// SecretInjectionConfig represents the configuration for injecting secret data into a Kubernetes secret.
type SecretInjectionConfig struct {
	// SecretRef contains the name and namespace of the Kubernetes secret where the data will be injected.
	SecretRef SecretRef `json:"secretRef"`

	// SecretKey is the key within the Kubernetes secret where the data will be injected.
	SecretKey string `json:"secretKey"`

	// ResponsePath is is a jq filter expression represents the path in the response where the secret value will be extracted from.
	ResponsePath string `json:"responsePath"`
}

// SecretRef contains the name and namespace of a Kubernetes secret.
type SecretRef struct {
	// Name is the name of the Kubernetes secret.
	Name string `json:"name"`

	// Namespace is the namespace of the Kubernetes secret.
	Namespace string `json:"namespace"`
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
