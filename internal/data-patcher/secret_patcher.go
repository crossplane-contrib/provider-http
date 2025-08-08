package datapatcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/crossplane-contrib/provider-http/apis/common"
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
func updateSecretWithPatchedValue(ctx context.Context, kubeClient client.Client, logger logging.Logger, data, originalData *httpClient.HttpResponse, secret *corev1.Secret, mapping common.KeyInjection) error {
	// Step 1: Parse and prepare data
	dataMap, err := prepareDataMap(originalData)
	if err != nil {
		return err
	}

	// Step 2: Extract the value to patch
	valueToPatch := extractValueToPatch(logger, dataMap, mapping.ResponseJQ)

	// Step 3: Update the secret data based on the missing strategy.
	updateSecretData(secret, mapping.SecretKey, valueToPatch, mapping.MissingFieldStrategy)

	// Step 4: Replace sensitive values in the HTTP response (only if the field was found).
	replaceSensitiveValues(data, secret, mapping.SecretKey, valueToPatch)

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
// If the field is a boolean or number, it converts it to a string.
// If the field is not found or cannot be parsed, it returns nil.
func extractValueToPatch(logger logging.Logger, dataMap map[string]interface{}, requestFieldPath string) *string {
	// Check if the field exists
	exists, err := jq.Exists(requestFieldPath, dataMap)
	if err != nil {
		logger.Debug(fmt.Sprintf("Error checking existence of field %s: %s", requestFieldPath, err))
		return nil
	}
	if !exists {
		return nil
	}

	// Attempt to parse the field as a string.
	if valueToPatch, err := jq.ParseString(requestFieldPath, dataMap); err == nil {
		return &valueToPatch
	} else {
		logger.Debug(fmt.Sprintf("Failed to parse the field %s as a string: %s", requestFieldPath, err))
	}

	// Attempt to parse the field as a boolean
	if boolResult, boolErr := jq.ParseBool(requestFieldPath, dataMap); boolErr == nil {
		boolStr := strconv.FormatBool(boolResult)
		return &boolStr
	} else {
		logger.Debug(fmt.Sprintf("Failed to parse the field %s as a boolean: %s", requestFieldPath, boolErr))
	}

	// Attempt to parse the field as a number.
	if numberResult, err := jq.ParseFloat(requestFieldPath, dataMap); err == nil {
		numStr := strconv.FormatFloat(numberResult, 'f', -1, 64)
		return &numStr
	} else {
		logger.Debug(fmt.Sprintf("Failed to parse the field %s as a number: %s", requestFieldPath, err))
	}

	logger.Info(fmt.Sprintf("Failed to parse the field %s as a string, boolean, or number, treating it as missing field.", requestFieldPath))
	return nil
}

// updateSecretData updates the data field of a Kubernetes Secret with the given key.
// If valueToPatch is non-nil, it updates the secret with the provided value.
// If valueToPatch is nil, then the missingStrategy is used:
//   - common.PreserveMissingField: does nothing
//   - common.SetEmptyMissingField: sets the value to ""
//   - common.DeleteMissingField: deletes the key from the secret if it exists.
func updateSecretData(secret *corev1.Secret, secretKey string, valueToPatch *string, missingStrategy common.MissingFieldStrategy) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	if valueToPatch != nil {
		secret.Data[secretKey] = []byte(*valueToPatch)
	} else {
		switch missingStrategy {
		case common.PreserveMissingField:
		case common.SetEmptyMissingField:
			secret.Data[secretKey] = []byte("")
		case common.DeleteMissingField:
			delete(secret.Data, secretKey)
		}
	}
}

// replaceSensitiveValues replaces occurrences of sensitive values in the HTTP response body
// and headers with a placeholder, iff the value is a json string surrounded by double quotes.
func replaceSensitiveValues(data *httpClient.HttpResponse, secret *corev1.Secret, secretKey string, valueToPatch *string) {
	if valueToPatch == nil || *valueToPatch == "" {
		return
	}

	placeholder := fmt.Sprintf("{{%s:%s:%s}}", secret.Name, secret.Namespace, secretKey)
	quotedValue := fmt.Sprintf("\"%s\"", *valueToPatch)
	quotedPlaceholder := fmt.Sprintf("\"%s\"", placeholder)
	data.Body = strings.ReplaceAll(data.Body, quotedValue, quotedPlaceholder)

	for _, headersList := range data.Headers {
		for i, header := range headersList {
			headersList[i] = strings.ReplaceAll(header, *valueToPatch, placeholder)
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
		originalValue := value
		if jq.IsJQQuery(value) {
			logger.Debug(fmt.Sprintf("Processing JQ expression for key %s: %s", key, value))
			newValue := extractValueToPatch(logger, dataMap, value)
			if newValue != nil {
				value = *newValue
				logger.Debug(fmt.Sprintf("JQ expression evaluated to: %s", value))
			} else {
				logger.Debug(fmt.Sprintf("JQ expression returned nil, keeping original value: %s", originalValue))
			}
		}

		// Validate that the value is suitable for Kubernetes labels
		if !isValidLabelValue(value) {
			logger.Info(fmt.Sprintf("Invalid label value for key %s: %s (original: %s). Skipping.", key, value, originalValue))
			continue
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

// isValidLabelValue checks if a value is valid for use as a Kubernetes label value.
// According to Kubernetes specs, label values must be empty or consist of alphanumeric characters,
// '-', '_' or '.', and must start and end with an alphanumeric character (regex: (([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?)
func isValidLabelValue(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 63 {
		return false
	}

	return isValidLabelPattern(value)
}

// isValidLabelPattern validates that the value matches Kubernetes label value pattern
func isValidLabelPattern(value string) bool {
	for i, r := range value {
		if isFirstOrLastChar(i, len(value)) {
			if !isAlphanumeric(r) {
				return false
			}
		} else if !isValidMiddleChar(r) {
			return false
		}
	}
	return true
}

// isFirstOrLastChar checks if the character position is first or last
func isFirstOrLastChar(index, length int) bool {
	return index == 0 || index == length-1
}

// isAlphanumeric checks if a rune is alphanumeric
func isAlphanumeric(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// isValidMiddleChar checks if a rune is valid for middle positions in a label value
func isValidMiddleChar(r rune) bool {
	return isAlphanumeric(r) || r == '-' || r == '_' || r == '.'
}
