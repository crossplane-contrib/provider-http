package requestprocessing

import (
	"encoding/json"
	"strings"

	"github.com/arielsepton/provider-http/internal/jq"
)

func ConvertStringToJQQuery(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

// ApplyJQOnStr applies a jq query to a Request, returning the result as a string.
// The function handles complex results by converting them to JSON format.
func ApplyJQOnStr(jqQuery string, baseMap map[string]interface{}) (string, error) {
	if result, _ := jq.ParseMapInterface(jqQuery, baseMap); result != nil {
		transformedData, err := json.Marshal(result)
		if err != nil {
			return "", err
		}
		return string(transformedData), nil
	}

	stringResult, err := jq.ParseString(jqQuery, baseMap)
	if err != nil {
		return "", err
	}

	return stringResult, nil
}

// ApplyJQOnMapStrings applies the provided JQ queries to a map of strings, using the given Request.
// It generates a base JQ object from the provided Request and then parses the queries to produce the resulting map.
func ApplyJQOnMapStrings(keyToJQQueries map[string][]string, baseMap map[string]interface{}) (map[string][]string, error) {
	return jq.ParseMapStrings(keyToJQQueries, baseMap)
}
