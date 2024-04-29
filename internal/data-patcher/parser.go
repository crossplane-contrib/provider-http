package datapatcher

import (
	"fmt"
	"regexp"
	"strings"

	"strconv"

	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	kubehandler "github.com/crossplane-contrib/provider-http/internal/kube-handler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
)

const (
	errEmptyKey    = "Warning, value at field %s is empty, skipping secret update for: %s"
	errConvertData = "failed to convert data to map"
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
func patchSecretsToValue(ctx context.Context, localKube client.Client, valueToHandle string) (string, error) {
	placeholders := removeDuplicates(findPlaceholders(valueToHandle))
	for _, placeholder := range placeholders {

		name, namespace, key, ok := parsePlaceholder(placeholder)
		if !ok {
			return valueToHandle, nil
		}
		secret, err := kubehandler.GetSecret(ctx, localKube, name, namespace)
		if err != nil {
			return "", err
		}

		valueToHandle = replacePlaceholderWithSecretValue(valueToHandle, placeholder, secret, key)
	}

	return valueToHandle, nil

}

// patchValueToSecret patches a value to a secret.
func patchValueToSecret(ctx context.Context, kubeClient client.Client, logger logging.Logger, data *httpClient.HttpResponse, secret *corev1.Secret, secretKey string, requestFieldPath string) error {
	dataMap, err := json_util.StructToMap(data)
	if err != nil {
		return errors.Wrap(err, errConvertData)
	}

	json_util.ConvertJSONStringsToMaps(&dataMap)

	valueToPatch, err := jq.ParseString(requestFieldPath, dataMap)
	if err != nil {
		boolResult, _ := jq.ParseBool(requestFieldPath, dataMap)
		valueToPatch = strconv.FormatBool(boolResult)
	}

	if valueToPatch == "" {
		logger.Info(fmt.Sprintf(errEmptyKey, requestFieldPath, fmt.Sprint(data)))
		return nil
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	secret.Data[secretKey] = []byte(valueToPatch)

	// patch the {{name:namespace:key}} of secret instead of the sensitive value
	placeholder := fmt.Sprintf("{{%s:%s:%s}}", secret.Name, secret.Namespace, secretKey)
	data.Body = strings.ReplaceAll(data.Body, valueToPatch, placeholder)
	for _, headersList := range data.Headers {
		for i, header := range headersList {
			newHeader := strings.ReplaceAll(header, valueToPatch, placeholder)
			headersList[i] = newHeader
		}
	}

	return kubehandler.UpdateSecret(ctx, kubeClient, secret)
}
