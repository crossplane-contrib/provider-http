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

package v1alpha1

import (
	"reflect"
	"context"
	"fmt"
	"strings"
	"encoding/json"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime"
    b64 "encoding/base64"

	corev1 "k8s.io/api/core/v1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
 
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

	WaitTimeout *metav1.Duration `json:"waitTimeout,omitempty"`

	// RollbackRetriesLimit is max number of attempts to retry HTTP request by sending again the request.
	RollbackRetriesLimit *int32 `json:"rollbackRetriesLimit,omitempty"`

	// InsecureSkipTLSVerify, when set to true, skips TLS certificate checks for the HTTP request
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// ExpectedResponse is a jq filter expression used to evaluate the HTTP response and determine if it matches the expected criteria.
	// The expression should return a boolean; if true, the response is considered expected.
	// Example: '.Body.job_status == "success"'
	ExpectedResponse string `json:"expectedResponse,omitempty"`
}

// A DisposableRequestSpec defines the desired state of a DisposableRequest.
type DisposableRequestSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider DisposableRequestParameters `json:"forProvider"`
	
	// RequestSecretDataPatches specifies the secrets providing patches for request data.
	RequestSecretDataPatches []SecretRef `json:"requestSecretDataPatches,omitempty"`

	// ResponseSecretDataPatches specifies the secrets receiving patches for response data.
	ResponseSecretDataPatches []SecretRef `json:"responseSecretDataPatches,omitempty"`
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

type SecretRef struct {
	Name string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Key string `json:"key,omitempty"`
	ToFieldPaths []string `json:"toFieldPaths,omitempty"`
	FromFieldPath string `json:"fromFieldPath,omitempty"`
}

// A DisposableRequestStatus represents the observed state of a DisposableRequest.
type DisposableRequestStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	Response            Response `json:"response,omitempty"`
	Failed              int32    `json:"failed,omitempty"`
	Error               string   `json:"error,omitempty"`
	Synced              bool     `json:"synced,omitempty"`
	RequestDetails      Mapping  `json:"requestDetails,omitempty"`
}

// +kubebuilder:object:root=true

// A DisposableRequest is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,http}
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

// ApplyToFieldPathPatch patches the "to" secret, using a source field
// on the "from" resource.
func (r *SecretRef) ApplyToFieldPathPatch(data string, secret *corev1.Secret, localKube client.Client, ctx context.Context) error {
	encodedValue := b64.StdEncoding.EncodeToString([]byte(data))
	if secret.Data == nil {
        secret.Data = make(map[string][]byte)
    }

    // Set the key to the encoded value
    secret.Data[r.Key] = []byte(encodedValue)

	err := localKube.Update(ctx, secret)
	if err != nil {
		return err
	}

	return nil
}

// ApplyFromFieldPathPatch patches the "to" resource, using a source field
// on the "from" resource.
func (r *SecretRef) ApplyFromFieldPathPatch(from, to runtime.Object) error {
	paved, err := fieldpath.PaveObject(from)
	if err != nil {
		return err
	}

	encodedValue, err := paved.GetValue("data."+r.Key)
	if err != nil {
		return err
	}

	fmt.Println("encodedValue")
	fmt.Println(encodedValue)
	originalValue, err := b64.StdEncoding.DecodeString(encodedValue.(string))
	originalString := string(originalValue)

	return patchFieldValueToObject(r.ToFieldPaths, originalString, to)
}

// patchFieldValueToObject, given a path, value and "to" object, will
// apply the value to the "to" object at the given path, returning
// any errors as they occur.
func patchFieldValueToObject(paths []string, value interface{}, to runtime.Object) error {
	for _, path := range paths {
		paved, err := fieldpath.PaveObject(to)
		if err != nil {
			return err
		}

		subPath := extractSubPath(path)

		// Since "body" is a string, we need to convert it to a JSON object in order to patch into it.
		if subPath != "" {
			fmt.Println("we have body here")
			fmt.Println(paved)

			// Convert the object to a map[string]interface{}
			bodyValue, err := paved.GetValue("spec.forProvider.body")
			fmt.Println("get success")

			if err != nil {
				return err
			}

			fmt.Println("currentValue")
			fmt.Println(bodyValue.(string))

			if IsJSONString(bodyValue.(string)) {
				fmt.Println("it is json string")

				bodyMap := JsonStringToMap(bodyValue.(string))
				// Convert map[string]interface{} to Unstructured
				unstructuredObj := &unstructured.Unstructured{}
				unstructuredObj.SetUnstructuredContent(bodyMap)

				// Convert Unstructured to runtime.Object
				toObject := unstructuredObj.DeepCopyObject()

				pavedBody, err := fieldpath.PaveObject(toObject)
				fmt.Println("after paved")

				fmt.Println("a new subpath: ")
				fmt.Println(subPath)

				fmt.Println("a new value: ")
				fmt.Println(value)

				err = pavedBody.SetValue(subPath, value)


				fmt.Println("after set value")
				if err != nil {
					// TODO: perhaps print warning?
					return err
				}

				fmt.Println("modifiedString")
				fmt.Println(pavedBody)


				// Convert the modified pavedBody back to a JSON string
				modifiedBodyJSON, err := json.Marshal(pavedBody.UnstructuredContent())
				if err != nil {
					return err
				}
				modifiedBodyString := string(modifiedBodyJSON)

				// Set the modified body string back to the original structure
				err = paved.SetValue("spec.forProvider.body", modifiedBodyString)
				if err != nil {
					return err
				}

			}
		} else {
			err = paved.SetValue("spec."+path, value)
			fmt.Println("paved.")
			if err != nil {
				// TODO: perhaps print warning?
				return err
			}
		}

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to) 
		fmt.Println("runtime.")

		if err != nil {
			fmt.Println("runtime err.")

			return err
		}
	}

	return nil
}

func IsJSONString(jsonStr string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(jsonStr), &js) == nil
}

func JsonStringToMap(jsonStr string) map[string]interface{} {
	var jsonData map[string]interface{}
	_ = json.Unmarshal([]byte(jsonStr), &jsonData)
	return jsonData
}

func extractSubPath(fullPath string) string {
    // Find the index of ".body" in the full path
    index := strings.Index(fullPath, ".body")
    if index == -1 {
        // If ".body" is not found, return an empty string
        return ""
    }

    // Extract the substring after ".body"
    subPath := fullPath[index+len(".body")+1:]
    return subPath
}