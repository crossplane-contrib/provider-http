package datapatcher

import (
	"context"
	"fmt"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	kubehandler "github.com/crossplane-contrib/provider-http/internal/kube-handler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errPatchToReferencedSecret = "cannot patch to referenced secret"
	errPatchDataToSecret       = "Warning, couldn't patch data from request to secret %s:%s, error: %s"
)

// PatchSecretsIntoResponse patches secrets into the provided response.
func PatchSecretsIntoResponse(ctx context.Context, localKube client.Client, response interfaces.HTTPResponse, logger logging.Logger) (interfaces.HTTPResponse, error) {
	// If response is nil, return nil (no response to patch)
	if response == nil {
		return nil, nil
	}

	patchedBody, err := PatchSecretsIntoString(ctx, localKube, response.GetBody(), logger)
	if err != nil {
		return nil, err
	}

	patchedHeaders, err := PatchSecretsIntoHeaders(ctx, localKube, response.GetHeaders(), logger)
	if err != nil {
		return nil, err
	}

	return &v1alpha2.Response{
		StatusCode: response.GetStatusCode(),
		Body:       patchedBody,
		Headers:    patchedHeaders,
	}, nil
}

// PatchSecretsIntoString patches secrets into the provided string.
func PatchSecretsIntoString(ctx context.Context, localKube client.Client, str string, logger logging.Logger) (string, error) {
	return patchSecretsToValue(ctx, localKube, str, logger)
}

// PatchSecretsIntoHeaders takes a map of headers and applies security measures to
// sensitive values within the headers. It creates a copy of the input map
// to avoid modifying the original map and iterates over the copied map
// to process each list of headers. It then applies the necessary modifications
// to each header using patchSecretsToValue function.
func PatchSecretsIntoHeaders(ctx context.Context, localKube client.Client, headers map[string][]string, logger logging.Logger) (map[string][]string, error) {
	headersCopy := copyHeaders(headers)

	for _, headersList := range headersCopy {
		for i, header := range headersList {
			newHeader, err := patchSecretsToValue(ctx, localKube, header, logger)
			if err != nil {
				return nil, err
			}

			headersList[i] = newHeader
		}
	}
	return headersCopy, nil
}

// copyHeaders creates a deep copy of the provided headers map.
func copyHeaders(headers map[string][]string) map[string][]string {
	headersCopy := make(map[string][]string, len(headers))
	for key, value := range headers {
		headersCopy[key] = append([]string(nil), value...)
	}

	return headersCopy
}

// patchResponseDataToSecret patches response data into a Kubernetes secret.
func patchResponseDataToSecret(ctx context.Context, localKube client.Client, logger logging.Logger, data, originalData *httpClient.HttpResponse, owner metav1.Object, secretConfig common.SecretInjectionConfig) error {
	secret, err := kubehandler.GetOrCreateSecret(ctx, localKube, secretConfig.SecretRef.Name, secretConfig.SecretRef.Namespace, owner)
	if err != nil {
		return err
	}

	err = applySecretConfig(ctx, localKube, logger, data, originalData, secretConfig, secret)
	if err != nil {
		return err
	}

	return nil
}

// applySecretConfig applies the secret configuration to the secret.
func applySecretConfig(ctx context.Context, localKube client.Client, logger logging.Logger, data *httpClient.HttpResponse, originalData *httpClient.HttpResponse, secretConfig common.SecretInjectionConfig, secret *v1.Secret) error {
	var err error

	if secretConfig.KeyMappings != nil {
		for _, mapping := range secretConfig.KeyMappings {
			err = updateSecretWithPatchedValue(ctx, localKube, logger, data, originalData, secret, mapping)
			if err != nil {
				return errors.Wrap(err, errPatchToReferencedSecret)
			}
		}
	} else {
		// Handle deprecated secretConfig fields
		mapping := common.KeyInjection{
			SecretKey:            secretConfig.SecretKey,
			ResponseJQ:           secretConfig.ResponsePath,
			MissingFieldStrategy: common.DeleteMissingField,
		}

		err = updateSecretWithPatchedValue(ctx, localKube, logger, data, originalData, secret, mapping)
		if err != nil {
			return errors.Wrap(err, errPatchToReferencedSecret)
		}
	}

	err = updateSecretLabelsAndAnnotations(ctx, localKube, logger, data, secret, secretConfig.Metadata.Labels, secretConfig.Metadata.Annotations)
	if err != nil {
		return errors.Wrap(err, errPatchToReferencedSecret)
	}

	return nil
}

// ApplyResponseDataToSecrets applies response data to Kubernetes Secrets as specified in the resource's SecretInjectionConfigs.
// For each SecretInjectionConfig, it extracts a value from the HTTP response and patches it into the referenced Secret.
// Ownership of the Secret is optionally set based on the configuration.
func ApplyResponseDataToSecrets(ctx context.Context, localKube client.Client, logger logging.Logger, response *httpClient.HttpResponse, secretConfigs []common.SecretInjectionConfig, cr metav1.Object) {
	// Create a copy of the original response to use for data extraction (JQ queries)
	// This ensures that each secret injection config extracts from the original response data
	originalResponse := &httpClient.HttpResponse{
		Body:       response.Body,
		Headers:    copyHeaders(response.Headers),
		StatusCode: response.StatusCode,
	}

	for _, ref := range secretConfigs {
		var owner metav1.Object = nil

		if ref.SetOwnerReference {
			owner = cr
		}

		// Use the cumulative response for patching (gets updated with secret placeholders)
		// and originalResponse for data extraction (remains unchanged)
		err := patchResponseDataToSecret(ctx, localKube, logger, response, originalResponse, owner, ref)
		if err != nil {
			logger.Info(fmt.Sprintf(errPatchDataToSecret, ref.SecretRef.Name, ref.SecretRef.Namespace, err.Error()))
		}
	}
}

// PatchSecretsIntoMap takes a map of string to interface{} and patches secrets
// into any string values within the map, including nested maps and slices.
func PatchSecretsIntoMap(ctx context.Context, localKube client.Client, data map[string]interface{}, logger logging.Logger) (map[string]interface{}, error) {
	dataCopy := copyMap(data)

	err := patchSecretsInMap(ctx, localKube, dataCopy, logger)
	if err != nil {
		return nil, err
	}

	return dataCopy, nil
}

// copyMap creates a deep copy of a map[string]interface{}.
func copyMap(original map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{}, len(original))
	for k, v := range original {
		copy[k] = deepCopy(v)
	}
	return copy
}

// deepCopy performs a deep copy of the value, handling maps and slices recursively.
func deepCopy(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return copyMap(v)
	case []interface{}:
		copy := make([]interface{}, len(v))
		for i, item := range v {
			copy[i] = deepCopy(item)
		}
		return copy
	default:
		return v
	}
}

// patchSecretsInSlice traverses a slice and patches secrets into any string values.
func patchSecretsInSlice(ctx context.Context, localKube client.Client, data []interface{}, logger logging.Logger) error {
	for i, item := range data {
		switch v := item.(type) {
		case string:
			// Patch secrets in string values
			patchedValue, err := patchSecretsToValue(ctx, localKube, v, logger)
			if err != nil {
				return err
			}
			data[i] = patchedValue

		case map[string]interface{}:
			// Recursively patch secrets in nested maps
			err := patchSecretsInMap(ctx, localKube, v, logger)
			if err != nil {
				return err
			}

		case []interface{}:
			// Recursively patch secrets in nested slices
			err := patchSecretsInSlice(ctx, localKube, v, logger)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
