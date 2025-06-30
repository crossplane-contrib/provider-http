package datapatcher

import (
	"context"
	"fmt"
	"strings"

	"github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	errParseJqFailed           = "jq regex matching failed in placeholder '%s'"
	errInvalidJq               = "given expression is not valid jq: '%s'"
	errPatchJqExpressionFailed = "failed to patch jq expression, %s"

// errPatchToReferencedSecret = "cannot patch to referenced secret"
// errPatchDataToSecret       = "Warning, couldn't patch data from request to secret %s:%s, error: %s"
)

// PatchSecretsIntoString patches jq filters against the AuthenticationResponse into the provided string.
func PatchAuthIntoString(ctx context.Context, authResponse http.HttpResponse, str string, logger logging.Logger) (string, error) {
	return patchJqPlaceholdersToValue(authResponse, str, logger)
}

// PatchSecretsIntoHeaders takes a map of headers and applies security measures to
// sensitive values within the headers. It creates a copy of the input map
// to avoid modifying the original map and iterates over the copied map
// to process each list of headers. It then applies the necessary modifications
// to each header using patchJqPlaceholdersToValue function.
func PatchAuthIntoHeaders(ctx context.Context, authResponse http.HttpResponse, headers map[string][]string, logger logging.Logger) (map[string][]string, error) {
	headersCopy := copyHeaders(headers)

	for _, headersList := range headersCopy {
		for i, header := range headersList {
			newHeader, err := patchJqPlaceholdersToValue(authResponse, header, logger)
			if err != nil {
				return nil, err
			}

			headersList[i] = newHeader
		}
	}
	return headersCopy, nil
}

// patchJqPlaceholdersToValue patches jq filters referenced in the provided value with matching values from the AuthenticationReponse
func patchJqPlaceholdersToValue(authResponse http.HttpResponse, valueToHandle string, logger logging.Logger) (string, error) {
	placeholders := removeDuplicates(findJqPlaceholders(valueToHandle))
	for _, jqPlaceholder := range placeholders {

		jqExpr, err := parseJqPlaceholder(jqPlaceholder)
		if err != nil {
			return "", err
		}

		dataMap, err := prepareDataMap(&authResponse)
		if err != nil {
			return "", err
		}

		jqResult := extractValueToPatch(logger, dataMap, jqExpr)
		if jqResult != nil {
			valueToHandle = strings.ReplaceAll(valueToHandle, jqPlaceholder, *jqResult)
		} else {
			return "", fmt.Errorf("jq expression '%s' returned no match of type string, bool or float. The HttpResponse could contain invalid json", jqExpr)
		}
	}
	return valueToHandle, nil
}

// example '{{jq .authResponse.body.token }}'
const jqSecretPattern = `\{\{jq\s+(.*?[^\s])\s*\}\}`

func findJqPlaceholders(value string) []string {
	return jqRe.FindAllString(value, -1)
}

func parseJqPlaceholder(placeholder string) (jqFilter string, err error) {
	matches := jqRe.FindStringSubmatch(placeholder)
	// we look for exactly one entire jq expression
	if len(matches) != 2 {
		return "", fmt.Errorf(errParseJqFailed, placeholder)
	}
	valid := jq.IsJQQuery(matches[1])
	if !valid {
		return "", fmt.Errorf(errInvalidJq, matches[1])
	}

	return matches[1], nil
}
