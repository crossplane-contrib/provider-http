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

// Package typeconv provides type conversion utilities between cluster and namespaced API types
package typeconv

import (
	clusterdisposablev1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha2"
	clusterv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
	namespaceddisposablev1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/disposablerequest/v1alpha2"
	namespacedv1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
)

// ConvertNamespacedToClusterRequestParameters converts namespaced RequestParameters to cluster RequestParameters
func ConvertNamespacedToClusterRequestParameters(src *namespacedv1alpha2.RequestParameters) *clusterv1alpha2.RequestParameters {
	if src == nil {
		return nil
	}

	return &clusterv1alpha2.RequestParameters{
		Mappings:               ConvertNamespacedToClusterMappings(src.Mappings),
		Payload:                ConvertNamespacedToClusterPayload(src.Payload),
		Headers:                src.Headers,
		WaitTimeout:            src.WaitTimeout,
		InsecureSkipTLSVerify:  src.InsecureSkipTLSVerify,
		SecretInjectionConfigs: src.SecretInjectionConfigs,
		ExpectedResponseCheck:  ConvertNamespacedToClusterExpectedResponseCheck(src.ExpectedResponseCheck),
		IsRemovedCheck:         ConvertNamespacedToClusterExpectedResponseCheck(src.IsRemovedCheck),
	}
}

// ConvertNamespacedToClusterMappings converts namespaced Mappings to cluster Mappings
func ConvertNamespacedToClusterMappings(src []namespacedv1alpha2.Mapping) []clusterv1alpha2.Mapping {
	if src == nil {
		return nil
	}

	result := make([]clusterv1alpha2.Mapping, len(src))
	for i, mapping := range src {
		result[i] = clusterv1alpha2.Mapping{
			Method:  mapping.Method,
			Body:    mapping.Body,
			URL:     mapping.URL,
			Headers: mapping.Headers,
		}
	}
	return result
}

// ConvertNamespacedToClusterPayload converts namespaced Payload to cluster Payload
func ConvertNamespacedToClusterPayload(src namespacedv1alpha2.Payload) clusterv1alpha2.Payload {
	return clusterv1alpha2.Payload{
		BaseUrl: src.BaseUrl, // Note: BaseUrl not BaseURL
		Body:    src.Body,
	}
}

// ConvertNamespacedToClusterExpectedResponseCheck converts namespaced ExpectedResponseCheck to cluster ExpectedResponseCheck
func ConvertNamespacedToClusterExpectedResponseCheck(src namespacedv1alpha2.ExpectedResponseCheck) clusterv1alpha2.ExpectedResponseCheck {
	return clusterv1alpha2.ExpectedResponseCheck{
		Type:  src.Type,
		Logic: src.Logic,
	}
}

// ConvertNamespacedToClusterRequest converts namespaced Request to cluster Request
func ConvertNamespacedToClusterRequest(src *namespacedv1alpha2.Request) *clusterv1alpha2.Request {
	if src == nil {
		return nil
	}

	return &clusterv1alpha2.Request{
		TypeMeta:   src.TypeMeta,
		ObjectMeta: src.ObjectMeta,
		Spec: clusterv1alpha2.RequestSpec{
			ResourceSpec: src.Spec.ResourceSpec,
			ForProvider:  *ConvertNamespacedToClusterRequestParameters(&src.Spec.ForProvider),
		},
		Status: ConvertNamespacedToClusterRequestStatus(src.Status),
	}
}

// ConvertNamespacedToClusterRequestStatus converts namespaced RequestStatus to cluster RequestStatus
func ConvertNamespacedToClusterRequestStatus(src namespacedv1alpha2.RequestStatus) clusterv1alpha2.RequestStatus {
	return clusterv1alpha2.RequestStatus{
		ResourceStatus: src.ResourceStatus,
		Response:       ConvertNamespacedToClusterResponse(src.Response),
		Failed:         src.Failed,
		Error:          src.Error,
		RequestDetails: ConvertNamespacedToClusterMappingStatus(src.RequestDetails),
		Cache:          ConvertNamespacedToClusterCache(src.Cache),
	}
}

// ConvertNamespacedToClusterResponse converts namespaced Response to cluster Response
func ConvertNamespacedToClusterResponse(src namespacedv1alpha2.Response) clusterv1alpha2.Response {
	return clusterv1alpha2.Response{
		StatusCode: src.StatusCode,
		Body:       src.Body,
		Headers:    src.Headers,
	}
}

// ConvertNamespacedToClusterMappingStatus converts namespaced Mapping to cluster Mapping for status
func ConvertNamespacedToClusterMappingStatus(src namespacedv1alpha2.Mapping) clusterv1alpha2.Mapping {
	return clusterv1alpha2.Mapping{
		Method:  src.Method,
		Body:    src.Body,
		URL:     src.URL,
		Headers: src.Headers,
	}
}

// ConvertNamespacedToClusterCache converts namespaced Cache to cluster Cache
func ConvertNamespacedToClusterCache(src namespacedv1alpha2.Cache) clusterv1alpha2.Cache {
	return clusterv1alpha2.Cache{
		Response:    ConvertNamespacedToClusterResponse(src.Response),
		LastUpdated: src.LastUpdated,
	}
}

// ConvertClusterToNamespacedResponse converts cluster Response to namespaced Response
func ConvertClusterToNamespacedResponse(src clusterv1alpha2.Response) namespacedv1alpha2.Response {
	return namespacedv1alpha2.Response{
		StatusCode: src.StatusCode,
		Body:       src.Body,
		Headers:    src.Headers,
	}
}

// DisposableRequest conversion functions

// ConvertNamespacedToClusterDisposableRequestParameters converts namespaced DisposableRequestParameters to cluster DisposableRequestParameters
func ConvertNamespacedToClusterDisposableRequestParameters(src *namespaceddisposablev1alpha2.DisposableRequestParameters) *clusterdisposablev1alpha2.DisposableRequestParameters {
	if src == nil {
		return nil
	}

	return &clusterdisposablev1alpha2.DisposableRequestParameters{
		URL:                    src.URL,
		Method:                 src.Method,
		Headers:                src.Headers,
		Body:                   src.Body,
		WaitTimeout:            src.WaitTimeout,
		RollbackRetriesLimit:   src.RollbackRetriesLimit,
		InsecureSkipTLSVerify:  src.InsecureSkipTLSVerify,
		ExpectedResponse:       src.ExpectedResponse,
		NextReconcile:          src.NextReconcile,
		ShouldLoopInfinitely:   src.ShouldLoopInfinitely,
		SecretInjectionConfigs: src.SecretInjectionConfigs,
	}
}

// ConvertNamespacedToClusterDisposableRequest converts namespaced DisposableRequest to cluster DisposableRequest
func ConvertNamespacedToClusterDisposableRequest(src *namespaceddisposablev1alpha2.DisposableRequest) *clusterdisposablev1alpha2.DisposableRequest {
	if src == nil {
		return nil
	}

	return &clusterdisposablev1alpha2.DisposableRequest{
		TypeMeta:   src.TypeMeta,
		ObjectMeta: src.ObjectMeta,
		Spec: clusterdisposablev1alpha2.DisposableRequestSpec{
			ResourceSpec: src.Spec.ResourceSpec,
			ForProvider:  *ConvertNamespacedToClusterDisposableRequestParameters(&src.Spec.ForProvider),
		},
		Status: ConvertNamespacedToClusterDisposableRequestStatus(src.Status),
	}
}

// ConvertNamespacedToClusterDisposableRequestStatus converts namespaced DisposableRequestStatus to cluster DisposableRequestStatus
func ConvertNamespacedToClusterDisposableRequestStatus(src namespaceddisposablev1alpha2.DisposableRequestStatus) clusterdisposablev1alpha2.DisposableRequestStatus {
	return clusterdisposablev1alpha2.DisposableRequestStatus{
		ResourceStatus:    src.ResourceStatus,
		Response:          ConvertNamespacedToClusterDisposableResponse(src.Response),
		Failed:            src.Failed,
		Error:             src.Error,
		Synced:            src.Synced,
		RequestDetails:    ConvertNamespacedToClusterDisposableMapping(src.RequestDetails),
		LastReconcileTime: src.LastReconcileTime,
	}
}

// ConvertNamespacedToClusterDisposableResponse converts namespaced disposable Response to cluster disposable Response
func ConvertNamespacedToClusterDisposableResponse(src namespaceddisposablev1alpha2.Response) clusterdisposablev1alpha2.Response {
	return clusterdisposablev1alpha2.Response{
		StatusCode: src.StatusCode,
		Body:       src.Body,
		Headers:    src.Headers,
	}
}

// ConvertNamespacedToClusterDisposableMapping converts namespaced disposable Mapping to cluster disposable Mapping
func ConvertNamespacedToClusterDisposableMapping(src namespaceddisposablev1alpha2.Mapping) clusterdisposablev1alpha2.Mapping {
	return clusterdisposablev1alpha2.Mapping{
		Method:  src.Method,
		Body:    src.Body,
		URL:     src.URL,
		Headers: src.Headers,
	}
}

// ConvertClusterToNamespacedDisposableResponse converts cluster disposable Response to namespaced disposable Response
func ConvertClusterToNamespacedDisposableResponse(src clusterdisposablev1alpha2.Response) namespaceddisposablev1alpha2.Response {
	return namespaceddisposablev1alpha2.Response{
		StatusCode: src.StatusCode,
		Body:       src.Body,
		Headers:    src.Headers,
	}
}

// Convenience functions with shorter names for easier use

// ToClusterRequest is a convenience wrapper for ConvertNamespacedToClusterRequest
func ToClusterRequest(src *namespacedv1alpha2.Request) (*clusterv1alpha2.Request, error) {
	if src == nil {
		return nil, nil
	}
	return ConvertNamespacedToClusterRequest(src), nil
}

// ToClusterRequestActionObserve returns the cluster action constant for OBSERVE
func ToClusterRequestActionObserve() string {
	return clusterv1alpha2.ActionObserve
}
