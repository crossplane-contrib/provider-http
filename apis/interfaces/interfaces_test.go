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

	disposablerequestv1alpha1 "github.com/crossplane-contrib/provider-http/apis/disposablerequest/v1alpha1"
	disposablerequestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/disposablerequest/v1alpha2"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	requestv1alpha1 "github.com/crossplane-contrib/provider-http/apis/request/v1alpha1"
	requestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
)

// TestInterfaceImplementations verifies that all types properly implement the expected interfaces.
func TestInterfaceImplementations(t *testing.T) {
	// Test v1alpha2.RequestParameters implements MappedHTTPRequestSpec
	var _ interfaces.MappedHTTPRequestSpec = (*requestv1alpha2.RequestParameters)(nil)

	// Test v1alpha1.RequestParameters implements MappedHTTPRequestSpec
	var _ interfaces.MappedHTTPRequestSpec = (*requestv1alpha1.RequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements SimpleHTTPRequestSpec
	var _ interfaces.SimpleHTTPRequestSpec = (*disposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha1.DisposableRequestParameters implements SimpleHTTPRequestSpec
	var _ interfaces.SimpleHTTPRequestSpec = (*disposablerequestv1alpha1.DisposableRequestParameters)(nil)

	// Test Response types implement HTTPResponse
	var _ interfaces.HTTPResponse = (*requestv1alpha2.Response)(nil)
	var _ interfaces.HTTPResponse = (*requestv1alpha1.Response)(nil)
	var _ interfaces.HTTPResponse = (*disposablerequestv1alpha2.Response)(nil)
	var _ interfaces.HTTPResponse = (*disposablerequestv1alpha1.Response)(nil)

	// Test Mapping types implement HTTPMapping
	var _ interfaces.HTTPMapping = (*requestv1alpha2.Mapping)(nil)
	var _ interfaces.HTTPMapping = (*requestv1alpha1.Mapping)(nil)

	// Test Payload types implement HTTPPayload
	var _ interfaces.HTTPPayload = (*requestv1alpha2.Payload)(nil)
	var _ interfaces.HTTPPayload = (*requestv1alpha1.Payload)(nil)
}

func TestV1Alpha2SpecificInterfaces(t *testing.T) {
	// Test v1alpha2.RequestParameters implements ResponseCheckAware
	var _ interfaces.ResponseCheckAware = (*requestv1alpha2.RequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements ReconciliationPolicyAware
	var _ interfaces.ReconciliationPolicyAware = (*disposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha2.DisposableRequestParameters implements RollbackAware
	var _ interfaces.RollbackAware = (*disposablerequestv1alpha2.DisposableRequestParameters)(nil)

	// Test v1alpha1.DisposableRequestParameters implements RollbackAware
	var _ interfaces.RollbackAware = (*disposablerequestv1alpha1.DisposableRequestParameters)(nil)

	// Test v1alpha2.Request implements RequestStatus
	var _ interfaces.RequestStatus = (*requestv1alpha2.Request)(nil)

	// Test v1alpha2.DisposableRequest implements DisposableRequestStatus
	var _ interfaces.DisposableRequestStatus = (*disposablerequestv1alpha2.DisposableRequest)(nil)
}

func TestMethodAccess(t *testing.T) {
	// Test that we can call interface methods
	params := &requestv1alpha2.RequestParameters{
		Mappings: []requestv1alpha2.Mapping{
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
