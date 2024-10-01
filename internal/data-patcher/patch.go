package datapatcher

import (
	"context"

	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	kubehandler "github.com/crossplane-contrib/provider-http/internal/kube-handler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errPatchToReferencedSecret = "cannot patch to referenced secret"
)

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

// PatchResponseToSecret patches response data into a Kubernetes secret.
func PatchResponseToSecret(ctx context.Context, localKube client.Client, logger logging.Logger, data *httpClient.HttpResponse, path, secretKey, secretName, secretNamespace string, owner metav1.Object) error {
	secret, err := kubehandler.GetOrCreateSecret(ctx, localKube, secretName, secretNamespace, owner)
	if err != nil {
		return err
	}

	err = patchValueToSecret(ctx, localKube, logger, data, secret, secretKey, path)
	if err != nil {
		return errors.Wrap(err, errPatchToReferencedSecret)
	}

	return nil
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
