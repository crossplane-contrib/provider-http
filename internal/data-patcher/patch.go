package datapatcher

import (
	"context"

	kubehandler "github.com/crossplane-contrib/provider-http/internal/kube-handler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errPatchToReferencedSecret = "cannot patch to referenced secret"
)

// PatchSecretsIntoBody patches secrets into the provided string body.
func PatchSecretsIntoBody(ctx context.Context, localKube client.Client, body string) (string, error) {
	return patchSecretsToValue(ctx, localKube, body)
}

// PatchSecretsIntoHeaders takes a map of headers and applies security measures to
// sensitive values within the headers. It creates a copy of the input map
// to avoid modifying the original map and iterates over the copied map
// to process each list of headers. It then applies the necessary modifications
// to each header using patchSecretsToValue function.
func PatchSecretsIntoHeaders(ctx context.Context, localKube client.Client, headers map[string][]string) (map[string][]string, error) {
	headersCopy := copyHeaders(headers)

	for _, headersList := range headersCopy {
		for i, header := range headersList {
			newHeader, err := patchSecretsToValue(ctx, localKube, header)
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
func PatchResponseToSecret(ctx context.Context, localKube client.Client, logger logging.Logger, data interface{}, path, secretKey, secretName, secretNamespace string) error {
	secret, err := kubehandler.GetOrCreateSecret(ctx, localKube, secretName, secretNamespace)
	if err != nil {
		return err
	}

	err = patchValueToSecret(ctx, localKube, logger, data, secret, secretKey, path)
	if err != nil {
		return errors.Wrap(err, errPatchToReferencedSecret)
	}

	return nil
}
