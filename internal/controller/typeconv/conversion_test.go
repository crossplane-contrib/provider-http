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

package typeconv

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterdisposablev1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha2"
	clusterv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
	"github.com/crossplane-contrib/provider-http/apis/common"
	namespaceddisposablev1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/disposablerequest/v1alpha2"
	namespacedv1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"

	commonxp "github.com/crossplane/crossplane-runtime/v2/apis/common"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

func TestConvertNamespacedToClusterRequestParameters(t *testing.T) {
	timeout := &metav1.Duration{Duration: 5 * time.Minute}

	tests := []struct {
		name string
		src  *namespacedv1alpha2.RequestParameters
		want *clusterv1alpha2.RequestParameters
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "basic conversion",
			src: &namespacedv1alpha2.RequestParameters{
				Mappings: []namespacedv1alpha2.Mapping{
					{
						Method: "POST",
						Action: "CREATE",
						URL:    "http://example.com",
						Body:   "test body",
					},
				},
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				WaitTimeout:           timeout,
				InsecureSkipTLSVerify: true,
			},
			want: &clusterv1alpha2.RequestParameters{
				Mappings: []clusterv1alpha2.Mapping{
					{
						Method: "POST",
						Action: "CREATE",
						URL:    "http://example.com",
						Body:   "test body",
					},
				},
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				WaitTimeout:           timeout,
				InsecureSkipTLSVerify: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterRequestParameters(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterRequestParameters() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterMappings(t *testing.T) {
	tests := []struct {
		name string
		src  []namespacedv1alpha2.Mapping
		want []clusterv1alpha2.Mapping
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "empty slice",
			src:  []namespacedv1alpha2.Mapping{},
			want: []clusterv1alpha2.Mapping{},
		},
		{
			name: "single mapping",
			src: []namespacedv1alpha2.Mapping{
				{
					Method: "GET",
					URL:    "http://test.com",
					Body:   "request body",
					Headers: map[string][]string{
						"Authorization": {"Bearer token"},
					},
				},
			},
			want: []clusterv1alpha2.Mapping{
				{
					Method: "GET",
					URL:    "http://test.com",
					Body:   "request body",
					Headers: map[string][]string{
						"Authorization": {"Bearer token"},
					},
				},
			},
		},
		{
			name: "mapping with action field",
			src: []namespacedv1alpha2.Mapping{
				{
					Method: "POST",
					Action: "CREATE",
					URL:    "http://api.example.com/users",
					Body:   `{"name": "test"}`,
					Headers: map[string][]string{
						"Content-Type": {"application/json"},
					},
				},
				{
					Action: "OBSERVE",
					URL:    "http://api.example.com/users/123",
				},
			},
			want: []clusterv1alpha2.Mapping{
				{
					Method: "POST",
					Action: "CREATE",
					URL:    "http://api.example.com/users",
					Body:   `{"name": "test"}`,
					Headers: map[string][]string{
						"Content-Type": {"application/json"},
					},
				},
				{
					Action: "OBSERVE",
					URL:    "http://api.example.com/users/123",
				},
			},
		},
		{
			name: "multiple mappings with different actions",
			src: []namespacedv1alpha2.Mapping{
				{
					Action: "CREATE",
					Method: "POST",
					URL:    "http://api.example.com/resources",
					Body:   `{"data": "create"}`,
				},
				{
					Action: "OBSERVE",
					URL:    "http://api.example.com/resources/{{.id}}",
				},
				{
					Action: "UPDATE",
					Method: "PUT",
					URL:    "http://api.example.com/resources/{{.id}}",
					Body:   `{"data": "update"}`,
				},
				{
					Action: "REMOVE",
					Method: "DELETE",
					URL:    "http://api.example.com/resources/{{.id}}",
				},
			},
			want: []clusterv1alpha2.Mapping{
				{
					Action: "CREATE",
					Method: "POST",
					URL:    "http://api.example.com/resources",
					Body:   `{"data": "create"}`,
				},
				{
					Action: "OBSERVE",
					URL:    "http://api.example.com/resources/{{.id}}",
				},
				{
					Action: "UPDATE",
					Method: "PUT",
					URL:    "http://api.example.com/resources/{{.id}}",
					Body:   `{"data": "update"}`,
				},
				{
					Action: "REMOVE",
					Method: "DELETE",
					URL:    "http://api.example.com/resources/{{.id}}",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterMappings(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterMappings() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterDisposableRequestParameters(t *testing.T) {
	timeout := &metav1.Duration{Duration: 10 * time.Minute}
	nextReconcile := &metav1.Duration{Duration: 30 * time.Second}

	tests := []struct {
		name string
		src  *namespaceddisposablev1alpha2.DisposableRequestParameters
		want *clusterdisposablev1alpha2.DisposableRequestParameters
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "basic conversion",
			src: &namespaceddisposablev1alpha2.DisposableRequestParameters{
				URL:                  "http://example.com/webhook",
				Method:               "POST",
				Body:                 `{"message": "test"}`,
				WaitTimeout:          timeout,
				NextReconcile:        nextReconcile,
				ShouldLoopInfinitely: false,
				SecretInjectionConfigs: []common.SecretInjectionConfig{
					{
						SecretRef: common.SecretRef{
							Name:      "test-secret",
							Namespace: "default",
						},
					},
				},
			},
			want: &clusterdisposablev1alpha2.DisposableRequestParameters{
				URL:                  "http://example.com/webhook",
				Method:               "POST",
				Body:                 `{"message": "test"}`,
				WaitTimeout:          timeout,
				NextReconcile:        nextReconcile,
				ShouldLoopInfinitely: false,
				SecretInjectionConfigs: []common.SecretInjectionConfig{
					{
						SecretRef: common.SecretRef{
							Name:      "test-secret",
							Namespace: "default",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterDisposableRequestParameters(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterDisposableRequestParameters() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterPayload(t *testing.T) {
	tests := []struct {
		name string
		src  namespacedv1alpha2.Payload
		want clusterv1alpha2.Payload
	}{
		{
			name: "empty payload",
			src:  namespacedv1alpha2.Payload{},
			want: clusterv1alpha2.Payload{},
		},
		{
			name: "payload with baseUrl and body",
			src: namespacedv1alpha2.Payload{
				BaseUrl: "https://api.example.com",
				Body:    `{"key": "value"}`,
			},
			want: clusterv1alpha2.Payload{
				BaseUrl: "https://api.example.com",
				Body:    `{"key": "value"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterPayload(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterPayload() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterExpectedResponseCheck(t *testing.T) {
	tests := []struct {
		name string
		src  namespacedv1alpha2.ExpectedResponseCheck
		want clusterv1alpha2.ExpectedResponseCheck
	}{
		{
			name: "empty check",
			src:  namespacedv1alpha2.ExpectedResponseCheck{},
			want: clusterv1alpha2.ExpectedResponseCheck{},
		},
		{
			name: "check with type and logic",
			src: namespacedv1alpha2.ExpectedResponseCheck{
				Type:  "jq",
				Logic: ".status == \"success\"",
			},
			want: clusterv1alpha2.ExpectedResponseCheck{
				Type:  "jq",
				Logic: ".status == \"success\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterExpectedResponseCheck(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterExpectedResponseCheck() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterResponse(t *testing.T) {
	tests := []struct {
		name string
		src  namespacedv1alpha2.Response
		want clusterv1alpha2.Response
	}{
		{
			name: "empty response",
			src:  namespacedv1alpha2.Response{},
			want: clusterv1alpha2.Response{},
		},
		{
			name: "complete response",
			src: namespacedv1alpha2.Response{
				StatusCode: 200,
				Body:       `{"result": "success"}`,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			want: clusterv1alpha2.Response{
				StatusCode: 200,
				Body:       `{"result": "success"}`,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterResponse(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterResponse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterMappingStatus(t *testing.T) {
	tests := []struct {
		name string
		src  namespacedv1alpha2.Mapping
		want clusterv1alpha2.Mapping
	}{
		{
			name: "empty mapping",
			src:  namespacedv1alpha2.Mapping{},
			want: clusterv1alpha2.Mapping{},
		},
		{
			name: "complete mapping",
			src: namespacedv1alpha2.Mapping{
				Method: "PUT",
				Action: "UPDATE",
				URL:    "https://api.test.com/update",
				Body:   `{"update": true}`,
				Headers: map[string][]string{
					"Authorization": {"Bearer xyz"},
				},
			},
			want: clusterv1alpha2.Mapping{
				Method: "PUT",
				Action: "UPDATE",
				URL:    "https://api.test.com/update",
				Body:   `{"update": true}`,
				Headers: map[string][]string{
					"Authorization": {"Bearer xyz"},
				},
			},
		},
		{
			name: "mapping with action only",
			src: namespacedv1alpha2.Mapping{
				Action: "OBSERVE",
				URL:    "https://api.test.com/resource/{{.id}}",
			},
			want: clusterv1alpha2.Mapping{
				Action: "OBSERVE",
				URL:    "https://api.test.com/resource/{{.id}}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterMappingStatus(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterMappingStatus() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterCache(t *testing.T) {
	tests := []struct {
		name string
		src  namespacedv1alpha2.Cache
		want clusterv1alpha2.Cache
	}{
		{
			name: "empty cache",
			src:  namespacedv1alpha2.Cache{},
			want: clusterv1alpha2.Cache{},
		},
		{
			name: "cache with response and timestamp",
			src: namespacedv1alpha2.Cache{
				Response: namespacedv1alpha2.Response{
					StatusCode: 200,
					Body:       `{"cached": true}`,
				},
				LastUpdated: "2023-01-01T12:00:00Z",
			},
			want: clusterv1alpha2.Cache{
				Response: clusterv1alpha2.Response{
					StatusCode: 200,
					Body:       `{"cached": true}`,
				},
				LastUpdated: "2023-01-01T12:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterCache(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterCache() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertClusterToNamespacedResponse(t *testing.T) {
	tests := []struct {
		name string
		src  clusterv1alpha2.Response
		want namespacedv1alpha2.Response
	}{
		{
			name: "empty response",
			src:  clusterv1alpha2.Response{},
			want: namespacedv1alpha2.Response{},
		},
		{
			name: "complete response",
			src: clusterv1alpha2.Response{
				StatusCode: 201,
				Body:       `{"created": true}`,
				Headers: map[string][]string{
					"Location": {"https://api.example.com/resource/123"},
				},
			},
			want: namespacedv1alpha2.Response{
				StatusCode: 201,
				Body:       `{"created": true}`,
				Headers: map[string][]string{
					"Location": {"https://api.example.com/resource/123"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertClusterToNamespacedResponse(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertClusterToNamespacedResponse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterDisposableResponse(t *testing.T) {
	tests := []struct {
		name string
		src  namespaceddisposablev1alpha2.Response
		want clusterdisposablev1alpha2.Response
	}{
		{
			name: "empty response",
			src:  namespaceddisposablev1alpha2.Response{},
			want: clusterdisposablev1alpha2.Response{},
		},
		{
			name: "complete response",
			src: namespaceddisposablev1alpha2.Response{
				StatusCode: 202,
				Body:       `{"accepted": true}`,
				Headers: map[string][]string{
					"X-Request-ID": {"req-123"},
				},
			},
			want: clusterdisposablev1alpha2.Response{
				StatusCode: 202,
				Body:       `{"accepted": true}`,
				Headers: map[string][]string{
					"X-Request-ID": {"req-123"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterDisposableResponse(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterDisposableResponse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterDisposableMapping(t *testing.T) {
	tests := []struct {
		name string
		src  namespaceddisposablev1alpha2.Mapping
		want clusterdisposablev1alpha2.Mapping
	}{
		{
			name: "empty mapping",
			src:  namespaceddisposablev1alpha2.Mapping{},
			want: clusterdisposablev1alpha2.Mapping{},
		},
		{
			name: "complete mapping",
			src: namespaceddisposablev1alpha2.Mapping{
				Method: "DELETE",
				URL:    "https://api.test.com/resource/456",
				Body:   "",
				Headers: map[string][]string{
					"X-API-Key": {"secret-key"},
				},
			},
			want: clusterdisposablev1alpha2.Mapping{
				Method: "DELETE",
				URL:    "https://api.test.com/resource/456",
				Body:   "",
				Headers: map[string][]string{
					"X-API-Key": {"secret-key"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterDisposableMapping(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterDisposableMapping() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertClusterToNamespacedDisposableResponse(t *testing.T) {
	tests := []struct {
		name string
		src  clusterdisposablev1alpha2.Response
		want namespaceddisposablev1alpha2.Response
	}{
		{
			name: "empty response",
			src:  clusterdisposablev1alpha2.Response{},
			want: namespaceddisposablev1alpha2.Response{},
		},
		{
			name: "complete response",
			src: clusterdisposablev1alpha2.Response{
				StatusCode: 204,
				Body:       "",
				Headers: map[string][]string{
					"X-Rate-Limit": {"100"},
				},
			},
			want: namespaceddisposablev1alpha2.Response{
				StatusCode: 204,
				Body:       "",
				Headers: map[string][]string{
					"X-Rate-Limit": {"100"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertClusterToNamespacedDisposableResponse(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertClusterToNamespacedDisposableResponse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterRequest(t *testing.T) {
	tests := []struct {
		name string
		src  *namespacedv1alpha2.Request
		want *clusterv1alpha2.Request
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "basic request conversion",
			src: &namespacedv1alpha2.Request{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "http.m.crossplane.io/v1alpha2",
					Kind:       "Request",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "default",
				},
				Spec: namespacedv1alpha2.RequestSpec{
					ForProvider: namespacedv1alpha2.RequestParameters{
						Mappings: []namespacedv1alpha2.Mapping{
							{
								Method: "POST",
								URL:    "https://example.com/api",
								Body:   `{"test": true}`,
							},
						},
					},
				},
				Status: namespacedv1alpha2.RequestStatus{
					Failed: 0,
				},
			},
			want: &clusterv1alpha2.Request{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "http.m.crossplane.io/v1alpha2",
					Kind:       "Request",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "default",
				},
				Spec: clusterv1alpha2.RequestSpec{
					ForProvider: clusterv1alpha2.RequestParameters{
						Mappings: []clusterv1alpha2.Mapping{
							{
								Method: "POST",
								URL:    "https://example.com/api",
								Body:   `{"test": true}`,
							},
						},
					},
				},
				Status: clusterv1alpha2.RequestStatus{
					Failed: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterRequest(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertNamespacedToClusterDisposableRequest(t *testing.T) {
	tests := []struct {
		name string
		src  *namespaceddisposablev1alpha2.DisposableRequest
		want *clusterdisposablev1alpha2.DisposableRequest
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "basic disposable request conversion",
			src: &namespaceddisposablev1alpha2.DisposableRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "http.m.crossplane.io/v1alpha2",
					Kind:       "DisposableRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-disposable",
					Namespace: "test-ns",
				},
				Spec: namespaceddisposablev1alpha2.DisposableRequestSpec{
					ForProvider: namespaceddisposablev1alpha2.DisposableRequestParameters{
						URL:    "https://webhook.example.com",
						Method: "POST",
						Body:   `{"event": "test"}`,
					},
				},
				Status: namespaceddisposablev1alpha2.DisposableRequestStatus{
					Synced: true,
				},
			},
			want: &clusterdisposablev1alpha2.DisposableRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "http.m.crossplane.io/v1alpha2",
					Kind:       "DisposableRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-disposable",
					Namespace: "test-ns",
				},
				Spec: clusterdisposablev1alpha2.DisposableRequestSpec{
					ForProvider: clusterdisposablev1alpha2.DisposableRequestParameters{
						URL:    "https://webhook.example.com",
						Method: "POST",
						Body:   `{"event": "test"}`,
					},
				},
				Status: clusterdisposablev1alpha2.DisposableRequestStatus{
					Synced: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertNamespacedToClusterDisposableRequest(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertNamespacedToClusterDisposableRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertManagedResourceSpecToResourceSpec(t *testing.T) {
	tests := []struct {
		name string
		src  xpv2.ManagedResourceSpec
		want xpv1.ResourceSpec
	}{
		{
			name: "empty spec",
			src:  xpv2.ManagedResourceSpec{},
			want: xpv1.ResourceSpec{},
		},
		{
			name: "spec with basic fields",
			src: xpv2.ManagedResourceSpec{
				WriteConnectionSecretToReference: &commonxp.LocalSecretReference{
					Name: "my-secret",
				},
				ProviderConfigReference: &commonxp.ProviderConfigReference{
					Name: "my-provider-config",
					Kind: "ProviderConfig",
				},
				ManagementPolicies: []commonxp.ManagementAction{commonxp.ManagementActionAll},
			},
			want: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name: "my-secret",
				},
				ProviderConfigReference: &xpv1.Reference{
					Name: "my-provider-config",
				},
				ManagementPolicies: []commonxp.ManagementAction{commonxp.ManagementActionAll},
			},
		},
		{
			name: "spec with nil references",
			src: xpv2.ManagedResourceSpec{
				ManagementPolicies: []commonxp.ManagementAction{commonxp.ManagementActionObserve, commonxp.ManagementActionCreate},
			},
			want: xpv1.ResourceSpec{
				ManagementPolicies: []commonxp.ManagementAction{commonxp.ManagementActionObserve, commonxp.ManagementActionCreate},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertManagedResourceSpecToResourceSpec(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertManagedResourceSpecToResourceSpec() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertLocalSecretToSecretReference(t *testing.T) {
	tests := []struct {
		name string
		src  *commonxp.LocalSecretReference
		want *xpv1.SecretReference
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "basic conversion",
			src: &commonxp.LocalSecretReference{
				Name: "my-connection-secret",
			},
			want: &xpv1.SecretReference{
				Name: "my-connection-secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertLocalSecretToSecretReference(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertLocalSecretToSecretReference() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConvertProviderConfigReferenceToReference(t *testing.T) {
	tests := []struct {
		name string
		src  *commonxp.ProviderConfigReference
		want *xpv1.Reference
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "basic conversion",
			src: &commonxp.ProviderConfigReference{
				Name: "my-provider-config",
				Kind: "ProviderConfig",
			},
			want: &xpv1.Reference{
				Name: "my-provider-config",
			},
		},
		{
			name: "cluster provider config",
			src: &commonxp.ProviderConfigReference{
				Name: "my-cluster-config",
				Kind: "ClusterProviderConfig",
			},
			want: &xpv1.Reference{
				Name: "my-cluster-config",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertProviderConfigReferenceToReference(tt.src)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ConvertProviderConfigReferenceToReference() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
