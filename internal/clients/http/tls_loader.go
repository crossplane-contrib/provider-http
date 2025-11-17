package http

import (
	"context"
	"fmt"

	"github.com/crossplane-contrib/provider-http/apis/common"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kube "sigs.k8s.io/controller-runtime/pkg/client"
)

// LoadTLSConfig loads TLS configuration from secrets and returns TLSConfigData
func LoadTLSConfig(ctx context.Context, kubeClient kube.Client, tlsConfig *common.TLSConfig) (*TLSConfigData, error) {
	if tlsConfig == nil {
		return &TLSConfigData{}, nil
	}

	data := &TLSConfigData{
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
	}

	// Load CA bundle from inline or secret
	if len(tlsConfig.CABundle) > 0 {
		data.CABundle = tlsConfig.CABundle
	} else if tlsConfig.CACertSecretRef != nil {
		caData, err := loadSecretData(ctx, kubeClient, tlsConfig.CACertSecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate from secret: %w", err)
		}
		data.CABundle = caData
	}

	// Load client certificate from secret
	if tlsConfig.ClientCertSecretRef != nil {
		certData, err := loadSecretData(ctx, kubeClient, tlsConfig.ClientCertSecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate from secret: %w", err)
		}
		data.ClientCert = certData
	}

	// Load client key from secret
	if tlsConfig.ClientKeySecretRef != nil {
		keyData, err := loadSecretData(ctx, kubeClient, tlsConfig.ClientKeySecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to load client key from secret: %w", err)
		}
		data.ClientKey = keyData
	}

	return data, nil
}

// loadSecretData loads data from a Kubernetes secret
func loadSecretData(ctx context.Context, kubeClient kube.Client, secretRef *xpv1.SecretKeySelector) ([]byte, error) {
	if secretRef == nil {
		return nil, nil
	}

	secret := &corev1.Secret{}
	nn := types.NamespacedName{
		Name:      secretRef.Name,
		Namespace: secretRef.Namespace,
	}

	if err := kubeClient.Get(ctx, nn, secret); err != nil {
		return nil, fmt.Errorf("cannot get secret %s/%s: %w", secretRef.Namespace, secretRef.Name, err)
	}

	data, ok := secret.Data[secretRef.Key]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s does not contain key %s", secretRef.Namespace, secretRef.Name, secretRef.Key)
	}

	return data, nil
}

// MergeTLSConfigs merges resource-level TLS config with provider-level TLS config
// Resource-level config takes precedence over provider-level config
func MergeTLSConfigs(resourceTLS *common.TLSConfig, providerTLS *common.TLSConfig) *common.TLSConfig {
	if resourceTLS == nil && providerTLS == nil {
		return nil
	}

	if resourceTLS == nil {
		return providerTLS
	}

	if providerTLS == nil {
		return resourceTLS
	}

	// Merge configs with resource taking precedence
	merged := &common.TLSConfig{
		InsecureSkipVerify: resourceTLS.InsecureSkipVerify,
	}

	mergeCABundle(merged, resourceTLS, providerTLS)
	mergeSecretRefs(merged, resourceTLS, providerTLS)

	return merged
}

// mergeCABundle merges CA bundle configuration
func mergeCABundle(merged, resourceTLS, providerTLS *common.TLSConfig) {
	if len(resourceTLS.CABundle) > 0 {
		merged.CABundle = resourceTLS.CABundle
	} else if len(providerTLS.CABundle) > 0 {
		merged.CABundle = providerTLS.CABundle
	}

	if resourceTLS.CACertSecretRef != nil {
		merged.CACertSecretRef = resourceTLS.CACertSecretRef
	} else if providerTLS.CACertSecretRef != nil {
		merged.CACertSecretRef = providerTLS.CACertSecretRef
	}
}

// mergeSecretRefs merges client certificate and key secret references
func mergeSecretRefs(merged, resourceTLS, providerTLS *common.TLSConfig) {
	if resourceTLS.ClientCertSecretRef != nil {
		merged.ClientCertSecretRef = resourceTLS.ClientCertSecretRef
	} else if providerTLS.ClientCertSecretRef != nil {
		merged.ClientCertSecretRef = providerTLS.ClientCertSecretRef
	}

	if resourceTLS.ClientKeySecretRef != nil {
		merged.ClientKeySecretRef = resourceTLS.ClientKeySecretRef
	} else if providerTLS.ClientKeySecretRef != nil {
		merged.ClientKeySecretRef = providerTLS.ClientKeySecretRef
	}
}
