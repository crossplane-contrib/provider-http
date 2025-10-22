/*
Copyright 2020 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// verify casting done in controller
var _ resource.ProviderConfig = &ClusterProviderConfig{}
var _ resource.ProviderConfigUsage = &ClusterProviderConfigUsage{}
var _ resource.ProviderConfigUsageList = &ClusterProviderConfigUsageList{}

// +kubebuilder:object:root=true

// A ClusterProviderConfig configures a Http provider for cross-namespace access.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentials.secretRef.name",priority=1
// +kubebuilder:resource:scope=Cluster
type ClusterProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterProviderConfigList contains a list of ClusterProviderConfig.
type ClusterProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterProviderConfig `json:"items"`
}

// +kubebuilder:object:root=true

// A ClusterProviderConfigUsage indicates that a resource is using a ClusterProviderConfig.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CONFIG-NAME",type="string",JSONPath=".providerConfigRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".resourceRef.name"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,http}
type ClusterProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv1.ProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true

// ClusterProviderConfigUsageList contains a list of ClusterProviderConfigUsage
type ClusterProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterProviderConfigUsage `json:"items"`
}

// ClusterProviderConfig type metadata.
var (
	ClusterProviderConfigKind             = reflect.TypeOf(ClusterProviderConfig{}).Name()
	ClusterProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterProviderConfigKind}.String()
	ClusterProviderConfigKindAPIVersion   = ClusterProviderConfigKind + "." + SchemeGroupVersion.String()
	ClusterProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ClusterProviderConfigKind)
)

// ClusterProviderConfigUsage type metadata.
var (
	ClusterProviderConfigUsageKind                 = reflect.TypeOf(ClusterProviderConfigUsage{}).Name()
	ClusterProviderConfigUsageGroupKind            = schema.GroupKind{Group: Group, Kind: ClusterProviderConfigUsageKind}.String()
	ClusterProviderConfigUsageKindAPIVersion       = ClusterProviderConfigUsageKind + "." + SchemeGroupVersion.String()
	ClusterProviderConfigUsageGroupVersionKind     = SchemeGroupVersion.WithKind(ClusterProviderConfigUsageKind)
	ClusterProviderConfigUsageListKind             = reflect.TypeOf(ClusterProviderConfigUsageList{}).Name()
	ClusterProviderConfigUsageListGroupVersionKind = SchemeGroupVersion.WithKind(ClusterProviderConfigUsageListKind)
)

// ClusterProviderConfig interface methods

// GetCondition returns the condition for the given ConditionType if exists,
// otherwise returns nil
func (cpc *ClusterProviderConfig) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return cpc.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource status
func (cpc *ClusterProviderConfig) SetConditions(c ...xpv1.Condition) {
	cpc.Status.SetConditions(c...)
}

// GetUsers returns the number of users of this ClusterProviderConfig.
func (cpc *ClusterProviderConfig) GetUsers() int64 {
	return cpc.Status.Users
}

// SetUsers sets the number of users of this ClusterProviderConfig.
func (cpc *ClusterProviderConfig) SetUsers(i int64) {
	cpc.Status.Users = i
}

// ClusterProviderConfigUsage interface methods

// SetResourceReference sets the resource reference.
func (cpcu *ClusterProviderConfigUsage) SetResourceReference(r xpv1.TypedReference) {
	cpcu.ResourceReference = r
}

// GetResourceReference gets the resource reference.
func (cpcu *ClusterProviderConfigUsage) GetResourceReference() xpv1.TypedReference {
	return cpcu.ResourceReference
}

// ClusterProviderConfigUsageList interface methods

// GetItems returns the list of ClusterProviderConfigUsage items.
func (cpcul *ClusterProviderConfigUsageList) GetItems() []resource.ProviderConfigUsage {
	items := make([]resource.ProviderConfigUsage, len(cpcul.Items))
	for i := range cpcul.Items {
		items[i] = &cpcul.Items[i]
	}
	return items
}

func init() {
	SchemeBuilder.Register(&ClusterProviderConfig{}, &ClusterProviderConfigList{})
	SchemeBuilder.Register(&ClusterProviderConfigUsage{}, &ClusterProviderConfigUsageList{})
}
