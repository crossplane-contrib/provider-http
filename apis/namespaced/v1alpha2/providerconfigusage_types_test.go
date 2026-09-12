/*
Copyright 2021 The Crossplane Authors.

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

package v1alpha2_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/v1alpha2"
)

func TestSchemeRegistersNamespacedProviderConfigUsageOnly(t *testing.T) {
	s := runtime.NewScheme()
	if err := v1alpha2.SchemeBuilder.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	cases := []struct {
		name string
		gvk  schema.GroupVersionKind
		want bool
	}{
		{
			name: "ProviderConfigUsage",
			gvk:  v1alpha2.ProviderConfigUsageGroupVersionKind,
			want: true,
		},
		{
			name: "ProviderConfigUsageList",
			gvk:  v1alpha2.ProviderConfigUsageListGroupVersionKind,
			want: true,
		},
		{
			name: "ClusterProviderConfigUsage",
			gvk:  schema.GroupVersion{Group: v1alpha2.Group, Version: v1alpha2.Version}.WithKind("ClusterProviderConfigUsage"),
			want: false,
		},
		{
			name: "ClusterProviderConfigUsageList",
			gvk:  schema.GroupVersion{Group: v1alpha2.Group, Version: v1alpha2.Version}.WithKind("ClusterProviderConfigUsageList"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.Recognizes(tc.gvk)
			if got != tc.want {
				t.Errorf("scheme.Recognizes(%v) = %v, want %v", tc.gvk, got, tc.want)
			}
		})
	}
}
