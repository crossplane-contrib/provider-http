package datapatcher

import (
	"fmt"
	"regexp"
	"strings"

	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubehandler "github.com/crossplane-contrib/provider-http/internal/kube-handler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	errEmptyKey    = "Warning, value at field %s is empty, skipping secret update for: %s"
	errConvertData = "failed to convert data to map"
	errPatchFailed = "failed to patch secret, %s"
)

const (
	secretPattern = `\{\{\s*([^:{}\s]+):([^:{}\s]+):([^:{}\s]+)\s*\}\}`
)

var re = regexp.MustCompile(secretPattern)

// findPlaceholders finds all placeholders in the provided string.
func findPlaceholders(value string) []string {
	return re.FindAllString(value, -1)
}

// removeDuplicates removes duplicate strings from the given slice.
func removeDuplicates(strSlice []string) []string {
	unique := make(map[string]struct{})
	var result []string

	for _, str := range strSlice {
		if _, ok := unique[str]; !ok {
			result = append(result, str)
			unique[str] = struct{}{}
		}
	}

	return result
}

// parsePlaceholder parses a placeholder string and returns its components.
func parsePlaceholder(placeholder string) (name, namespace, key string, ok bool) {
	matches := re.FindStringSubmatch(placeholder)

	if len(matches) != 4 {
		return "", "", "", false
	}

	return matches[1], matches[2], matches[3], true
}

// replacePlaceholderWithSecretValue replaces a placeholder with the value from a secret.
func replacePlaceholderWithSecretValue(originalString, old string, secret *corev1.Secret, key string) string {
	replacementString := string(secret.Data[key])
	return strings.ReplaceAll(originalString, old, replacementString)
}

// patchSecretsToValue patches secrets referenced in the provided value.
func patchSecretsToValue(ctx context.Context, localKube client.Client, valueToHandle string, logger logging.Logger) (string, error) {
	placeholders := removeDuplicates(findPlaceholders(valueToHandle))
	for _, placeholder := range placeholders {

		name, namespace, key, ok := parsePlaceholder(placeholder)
		if !ok {
			return valueToHandle, nil
		}
		secret, err := kubehandler.GetSecret(ctx, localKube, name, namespace)
		if err != nil {
			logger.Info(fmt.Sprintf(errPatchFailed, err.Error()))
			return "", err
		}

		valueToHandle = replacePlaceholderWithSecretValue(valueToHandle, placeholder, secret, key)
	}

	return valueToHandle, nil

}

// patchSecretsInMap traverses a map and patches secrets into any string values.
func patchSecretsInMap(ctx context.Context, localKube client.Client, data map[string]interface{}, logger logging.Logger) error {
	for key, value := range data {
		switch v := value.(type) {
		case string:
			patchedValue, err := patchSecretsToValue(ctx, localKube, v, logger)
			if err != nil {
				return err
			}
			data[key] = patchedValue

		case map[string]interface{}:
			err := patchSecretsInMap(ctx, localKube, v, logger)
			if err != nil {
				return err
			}

		case []interface{}:
			err := patchSecretsInSlice(ctx, localKube, v, logger)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
