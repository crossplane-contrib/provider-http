/*
Copyright 2023 The Crossplane Authors.
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

// Package common contains shared types that are used in multiple CRDs.
// +kubebuilder:object:generate=true
package common

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// TLSConfig contains TLS configuration for HTTPS requests.
type TLSConfig struct {
	// CABundle is a PEM encoded CA bundle which will be used to validate the server certificate.
	// If empty, system root CAs will be used.
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// CACertSecretRef is a reference to a secret containing the CA certificate(s).
	// The secret must contain a key specified in the SecretKeySelector.
	// +optional
	CACertSecretRef *xpv1.SecretKeySelector `json:"caCertSecretRef,omitempty"`

	// ClientCertSecretRef is a reference to a secret containing the client certificate.
	// The secret must contain a key specified in the SecretKeySelector.
	// +optional
	ClientCertSecretRef *xpv1.SecretKeySelector `json:"clientCertSecretRef,omitempty"`

	// ClientKeySecretRef is a reference to a secret containing the client private key.
	// The secret must contain a key specified in the SecretKeySelector.
	// +optional
	ClientKeySecretRef *xpv1.SecretKeySelector `json:"clientKeySecretRef,omitempty"`

	// InsecureSkipVerify controls whether the client verifies the server's certificate chain and host name.
	// If true, any certificate presented by the server and any host name in that certificate is accepted.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}
