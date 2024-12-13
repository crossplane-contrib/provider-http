package datapatcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	kubehandler "github.com/crossplane-contrib/provider-http/internal/kube-handler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	logUpdateSecretLabelsAndAnnotations = "Updating labels and annotations for Secret [%s/%s]"
	logNoUpdatesRequired                = "No updates required for labels and annotations of Secret [%s/%s]"
)

// updateSecretLabelsAndAnnotations updates the labels and annotations of a Kubernetes Secret
// based on the provided maps. It ensures the Secret is only updated if there are actual changes.
func updateSecretLabelsAndAnnotations(ctx context.Context, kubeClient client.Client, logger logging.Logger, data *httpClient.HttpResponse, secret *corev1.Secret, labels map[string]string, annotations map[string]string) error {
	updated := false

	dataMap, err := prepareDataMap(data)
	if err != nil {
		return err
	}

	// Update labels
	if secret.Labels == nil && labels != nil {
		secret.Labels = make(map[string]string)
	}
	updated = syncMap(logger, &secret.Labels, labels, dataMap) || updated

	// Update annotations
	if secret.Annotations == nil && annotations != nil {
		secret.Annotations = make(map[string]string)
	}
	updated = syncMap(logger, &secret.Annotations, annotations, dataMap) || updated

	// Update the Secret only if changes were made
	if updated {
		logger.Debug(fmt.Sprintf(logUpdateSecretLabelsAndAnnotations, secret.Namespace, secret.Name))
		return kubehandler.UpdateSecret(ctx, kubeClient, secret)
	}

	logger.Debug(fmt.Sprintf(logNoUpdatesRequired, secret.Namespace, secret.Name))
	return nil
}

// updateSecretWithPatchedValue extracts a specified value from an HTTP response,
// transforms it if necessary, and patches it into a Kubernetes Secret. Additionally,
// it replaces the sensitive value in the HTTP response body and headers with a placeholder.
func updateSecretWithPatchedValue(ctx context.Context, kubeClient client.Client, logger logging.Logger, data *httpClient.HttpResponse, secret *corev1.Secret, secretKey string, requestFieldPath string) error {
	// Step 1: Parse and prepare data
	dataMap, err := prepareDataMap(data)
	if err != nil {
		return err
	}

	// Step 2: Extract the value to patch
	valueToPatch := extractValueToPatch(logger, dataMap, requestFieldPath)

	// Step 3: Check if the value is already present
	if isSecretDataUpToDate(secret, secretKey, valueToPatch) {
		return nil
	}

	// Step 4: Update the secret data
	updateSecretData(secret, secretKey, valueToPatch)

	// Step 5: Replace sensitive values in the HTTP response
	replaceSensitiveValues(data, secret, secretKey, valueToPatch)

	// Step 5: Save the updated secret to the Kubernetes API
	return kubehandler.UpdateSecret(ctx, kubeClient, secret)
}

// prepareDataMap converts an HTTP response into a map for parsing and manipulation.
func prepareDataMap(data *httpClient.HttpResponse) (map[string]interface{}, error) {
	dataMap, err := json_util.StructToMap(data)
	if err != nil {
		return nil, errors.Wrap(err, errConvertData)
	}
	json_util.ConvertJSONStringsToMaps(&dataMap)
	return dataMap, nil
}

// extractValueToPatch extracts a value from a data map based on the given field path.
// If the field is a boolean, it converts it to a string.
func extractValueToPatch(logger logging.Logger, dataMap map[string]interface{}, requestFieldPath string) string {
	// Attempt to parse the field as a string
	valueToPatch, err := jq.ParseString(requestFieldPath, dataMap)
	if err == nil {
		return valueToPatch
	}
	logger.Debug(fmt.Sprintf("Failed to parse the field %s as a string: %s", requestFieldPath, err))

	// Attempt to parse the field as a boolean
	boolResult, boolErr := jq.ParseBool(requestFieldPath, dataMap)
	if boolErr == nil {
		return strconv.FormatBool(boolResult)
	}
	logger.Debug(fmt.Sprintf("Failed to parse the field %s as a boolean: %s", requestFieldPath, boolErr))

	// Attempt to parse the field as a number
	numberResult, numberErr := jq.ParseFloat(requestFieldPath, dataMap)
	if numberErr == nil {
		return strconv.FormatFloat(numberResult, 'f', -1, 64)
	}
	logger.Debug(fmt.Sprintf("Failed to parse the field %s as a number: %s", requestFieldPath, numberErr))

	logger.Info(fmt.Sprintf("Failed to parse the field %s as a string, boolean, or number: %s, setting an empty string instead.", requestFieldPath, err))
	return ""
}

// updateSecretData updates the data field of a Kubernetes Secret with the given key and value.
func updateSecretData(secret *corev1.Secret, secretKey, valueToPatch string) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[secretKey] = []byte(valueToPatch)
}

// replaceSensitiveValues replaces occurrences of a sensitive value in the HTTP response body
// and headers with a placeholder.
func replaceSensitiveValues(data *httpClient.HttpResponse, secret *corev1.Secret, secretKey, valueToPatch string) {
	if valueToPatch == "" {
		return
	}

	placeholder := fmt.Sprintf("{{%s:%s:%s}}", secret.Name, secret.Namespace, secretKey)
	data.Body = strings.ReplaceAll(data.Body, valueToPatch, placeholder)

	for _, headersList := range data.Headers {
		for i, header := range headersList {
			headersList[i] = strings.ReplaceAll(header, valueToPatch, placeholder)
		}
	}
}

// isSecretDataUpToDate checks if the specified key in the Secret already contains the given value.
func isSecretDataUpToDate(secret *corev1.Secret, secretKey, valueToPatch string) bool {
	currentValue, exists := secret.Data[secretKey]
	return exists && string(currentValue) == valueToPatch
}

// syncMap synchronizes a Secret's existing map (labels or annotations) with the desired state.
// It adds or updates keys from the desired map and removes keys not present in the desired map.
// Returns true if any changes were made.
func syncMap(logger logging.Logger, existing *map[string]string, desired map[string]string, dataMap map[string]interface{}) bool {
	changed := false

	// Add or update keys
	for key, value := range desired {
		if jq.IsJQQuery(value) {
			newValue := extractValueToPatch(logger, dataMap, value)
			if len(newValue) != 0 {
				value = newValue
			}
		}
		if (*existing)[key] != value {
			(*existing)[key] = value
			changed = true
		}
	}

	// Remove keys not in the desired map
	for key := range *existing {
		if _, exists := desired[key]; !exists {
			delete(*existing, key)
			changed = true
		}
	}

	return changed
}