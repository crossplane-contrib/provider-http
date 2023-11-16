package requestprocessing

import (
	"encoding/json"
	"strings"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/jq"
	json_util "github.com/arielsepton/provider-http/internal/json"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"golang.org/x/exp/maps"
)

func ConvertStringToJQQuery(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

// ApplyJQOnStr applies a jq query to a Request, returning the result as a string.
// The function handles complex results by converting them to JSON format.
func ApplyJQOnStr(jqQuery string, request *v1alpha1.Request, logger logging.Logger) (string, error) {
	baseMap := generateJQObject(request)

	if result, _ := jq.ParseMapInterface(jqQuery, baseMap, logger); result != nil {
		transformedData, err := json.Marshal(result)
		if err != nil {
			return "", err
		}
		return string(transformedData), nil
	}

	stringResult, err := jq.ParseString(jqQuery, baseMap, logger)
	if err != nil {
		return "", err
	}

	return stringResult, nil
}

// ApplyJQOnMapStrings applies the provided JQ queries to a map of strings, using the given Request.
// It generates a base JQ object from the provided Request and then parses the queries to produce the resulting map.
func ApplyJQOnMapStrings(keyToJQQueries map[string][]string, request *v1alpha1.Request, logger logging.Logger) (map[string][]string, error) {
	baseMap := generateJQObject(request)
	return jq.ParseMapStrings(keyToJQQueries, baseMap, logger)
}

// generateJQObject creates a JSON-compatible map from the specified Request's ForProvider and Status fields.
// It merges the two maps, converts JSON strings to nested maps, and returns the resulting map.
func generateJQObject(request *v1alpha1.Request) map[string]interface{} {
	baseMap, _ := json_util.StructToMap(request.Spec.ForProvider)
	statusMap, _ := json_util.StructToMap(request.Status)

	maps.Copy(baseMap, statusMap)
	json_util.ConvertJSONStringsToMaps(&baseMap)

	return baseMap
}
