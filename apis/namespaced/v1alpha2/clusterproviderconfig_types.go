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

var _ resource.ProviderConfig = &ClusterProviderConfig{}

// +kubebuilder:object:root=true

// A ClusterProviderConfig configures a Http provider at the cluster level for cross-namespace access.
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

// ClusterProviderConfig type metadata.
var (
	ClusterProviderConfigKind             = reflect.TypeOf(ClusterProviderConfig{}).Name()
	ClusterProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterProviderConfigKind}.String()
	ClusterProviderConfigKindAPIVersion   = ClusterProviderConfigKind + "." + SchemeGroupVersion.String()
	ClusterProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ClusterProviderConfigKind)
)

// GetCondition returns the condition for the given ConditionType if exists,
// otherwise returns nil
func (pc *ClusterProviderConfig) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return pc.Status.GetCondition(ct)
}

// SetConditions sets the conditions on the resource status
func (pc *ClusterProviderConfig) SetConditions(c ...xpv1.Condition) {
	pc.Status.SetConditions(c...)
}

// GetUsers returns the number of users of this ClusterProviderConfig.
func (pc *ClusterProviderConfig) GetUsers() int64 {
	return pc.Status.Users
}

// SetUsers sets the number of users of this ClusterProviderConfig.
func (pc *ClusterProviderConfig) SetUsers(i int64) {
	pc.Status.Users = i
}

func init() {
	SchemeBuilder.Register(&ClusterProviderConfig{}, &ClusterProviderConfigList{})
}
