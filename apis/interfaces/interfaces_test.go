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

package interfaces_test

import (
	"testing"

	// Cluster-scoped imports
	clusterdisposablerequestv1alpha1 "github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha1"
	clusterdisposablerequestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha2"
	clusterrequestv1alpha1 "github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha1"
	clusterrequestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
	
	// Namespaced imports
	namespaceddisposablerequestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/disposablerequest/v1alpha2"
	namespacedrequestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
	
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
)

// TestClusterScopedInterfaceImplementations verifies that cluster-scoped types properly implement the expected interfaces.
func TestClusterScopedInterfaceImplementations(t *testing.T) {
	// Test v1alpha2.RequestParameters implements MappedHTTPRequestSpec
	var _ interfaces.MappedHTTPRequestSpec = (*clusterrequestv1alpha2.RequestParameters)(nil)

	// Test v1alpha1.RequestParameters implements MappedHTTPRequestSpec
	var _ interfaces.MappedHTTPRequestSpec = (*clusterrequestv1alpha1.RequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements SimpleHTTPRequestSpec
	var _ interfaces.SimpleHTTPRequestSpec = (*clusterdisposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha1.DisposableRequestParameters implements SimpleHTTPRequestSpec
	var _ interfaces.SimpleHTTPRequestSpec = (*clusterdisposablerequestv1alpha1.DisposableRequestParameters)(nil)

	// Test Response types implement HTTPResponse
	var _ interfaces.HTTPResponse = (*clusterrequestv1alpha2.Response)(nil)
	var _ interfaces.HTTPResponse = (*clusterrequestv1alpha1.Response)(nil)
	var _ interfaces.HTTPResponse = (*clusterdisposablerequestv1alpha2.Response)(nil)
	var _ interfaces.HTTPResponse = (*clusterdisposablerequestv1alpha1.Response)(nil)

	// Test Mapping types implement HTTPMapping
	var _ interfaces.HTTPMapping = (*clusterrequestv1alpha2.Mapping)(nil)
	var _ interfaces.HTTPMapping = (*clusterrequestv1alpha1.Mapping)(nil)

	// Test Payload types implement HTTPPayload
	var _ interfaces.HTTPPayload = (*clusterrequestv1alpha2.Payload)(nil)
	var _ interfaces.HTTPPayload = (*clusterrequestv1alpha1.Payload)(nil)
}

// TestNamespacedInterfaceImplementations verifies that namespaced types properly implement the expected interfaces.
func TestNamespacedInterfaceImplementations(t *testing.T) {
	// Test v1alpha2.RequestParameters implements MappedHTTPRequestSpec
	var _ interfaces.MappedHTTPRequestSpec = (*namespacedrequestv1alpha2.RequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements SimpleHTTPRequestSpec
	var _ interfaces.SimpleHTTPRequestSpec = (*namespaceddisposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test Response types implement HTTPResponse
	var _ interfaces.HTTPResponse = (*namespacedrequestv1alpha2.Response)(nil)
	var _ interfaces.HTTPResponse = (*namespaceddisposablerequestv1alpha2.Response)(nil)

	// Test Mapping types implement HTTPMapping
	var _ interfaces.HTTPMapping = (*namespacedrequestv1alpha2.Mapping)(nil)

	// Test Payload types implement HTTPPayload
	var _ interfaces.HTTPPayload = (*namespacedrequestv1alpha2.Payload)(nil)
}

func TestClusterScopedV1Alpha2SpecificInterfaces(t *testing.T) {
	// Test v1alpha2.RequestParameters implements ResponseCheckAware
	var _ interfaces.ResponseCheckAware = (*clusterrequestv1alpha2.RequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements ReconciliationPolicyAware
	var _ interfaces.ReconciliationPolicyAware = (*clusterdisposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements RollbackAware
	var _ interfaces.RollbackAware = (*clusterdisposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha1.DisposableRequestParameters implements RollbackAware
	var _ interfaces.RollbackAware = (*clusterdisposablerequestv1alpha1.DisposableRequestParameters)(nil)

	// Test v1alpha2.Request implements RequestStatus
	var _ interfaces.RequestStatus = (*clusterrequestv1alpha2.Request)(nil)

	// Test v1alpha2.DisposableRequest implements DisposableRequestStatus
	var _ interfaces.DisposableRequestStatus = (*clusterdisposablerequestv1alpha2.DisposableRequest)(nil)
}

func TestNamespacedV1Alpha2SpecificInterfaces(t *testing.T) {
	// Test v1alpha2.RequestParameters implements ResponseCheckAware
	var _ interfaces.ResponseCheckAware = (*namespacedrequestv1alpha2.RequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements ReconciliationPolicyAware
	var _ interfaces.ReconciliationPolicyAware = (*namespaceddisposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements RollbackAware
	var _ interfaces.RollbackAware = (*namespaceddisposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha2.Request implements RequestStatus
	var _ interfaces.RequestStatus = (*namespacedrequestv1alpha2.Request)(nil)

	// Test v1alpha2.DisposableRequest implements DisposableRequestStatus
	var _ interfaces.DisposableRequestStatus = (*namespaceddisposablerequestv1alpha2.DisposableRequest)(nil)
}

func TestClusterScopedMethodAccess(t *testing.T) {
	// Test that we can call interface methods on cluster-scoped resources
	params := &clusterrequestv1alpha2.RequestParameters{
		Mappings: []clusterrequestv1alpha2.Mapping{
			{URL: "https://example.com", Method: "GET"},
		},
	}

	var spec interfaces.MappedHTTPRequestSpec = params

	mappings := spec.GetMappings()
	if len(mappings) != 1 {
		t.Errorf("Expected 1 mapping, got %d", len(mappings))
	}

	if mappings[0].GetURL() != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got '%s'", mappings[0].GetURL())
	}
}

func TestNamespacedMethodAccess(t *testing.T) {
	// Test that we can call interface methods on namespaced resources
	params := &namespacedrequestv1alpha2.RequestParameters{
		Mappings: []namespacedrequestv1alpha2.Mapping{
			{URL: "https://api.example.com", Method: "POST"},
		},
	}

	var spec interfaces.MappedHTTPRequestSpec = params

	mappings := spec.GetMappings()
	if len(mappings) != 1 {
		t.Errorf("Expected 1 mapping, got %d", len(mappings))
	}

	if mappings[0].GetURL() != "https://api.example.com" {
		t.Errorf("Expected URL 'https://api.example.com', got '%s'", mappings[0].GetURL())
	}
}
