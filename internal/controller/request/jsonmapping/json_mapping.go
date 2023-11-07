package jsonmapping

import (
	"encoding/json"
	"fmt"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/jq"
	"golang.org/x/exp/maps"
)

// This function receives a mapping and transforms it into a jq query format.
func CreateJQQuery(mapping string) (string, error) {
	if mapping == "" {
		return "", nil
	}

	data, err := jsonStringToMap(mapping)
	if err != nil {
		return "", err
	}

	var jqQuery string = "{"
	iterateMappingForJQQuery(data, &jqQuery)
	jqQuery = jqQuery[:len(jqQuery)-1]
	jqQuery += " }"

	return jqQuery, nil
}

func iterateMappingForJQQuery(data map[string]interface{}, jqQuery *string) {
	for key, value := range data {
		switch valueToHandle := value.(type) {
		case string:
			*jqQuery += fmt.Sprintf(" %s: %s ,", key, value)
		case []interface{}:
			*jqQuery += fmt.Sprintf(" %s: %s ,", key, value)
		case map[string]interface{}:
			iterateMappingForJQQuery(valueToHandle, jqQuery)
		}
	}
}

func jsonStringToMap(jsonStr string) (map[string]interface{}, error) {
	var jsonData map[string]interface{}

	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}

func structToMap(obj interface{}) (newMap map[string]interface{}, err error) {
	data, err := json.Marshal(obj) // Convert to a json string

	if err != nil {
		return
	}

	err = json.Unmarshal(data, &newMap) // Convert to a map
	return
}

func ApplyGoJQ(jqQuery string, request *v1alpha1.Request) (string, error) {
	baseMap, _ := structToMap(request.Spec.ForProvider)
	statusMap, _ := structToMap(request.Status)

	maps.Copy(baseMap, statusMap)
	convertJSONStringsToMaps(&baseMap)

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

// Converts JSON strings within a map to maps for JSON data processing.
func convertJSONStringsToMaps(merged *map[string]interface{}) {
	for key, value := range *merged {

		switch valueToHandle := value.(type) {
		case string:
			if IsJSONString(valueToHandle) {
				mappedJSON, _ := jsonStringToMap(valueToHandle)
				(*merged)[key] = mappedJSON
			}
		case map[string]interface{}:
			convertJSONStringsToMaps(&valueToHandle)
		case []interface{}:
			structToMap, _ := (structToMap(valueToHandle))
			convertJSONStringsToMaps(&structToMap)
		}
	}
}

func IsJSONString(jsonStr string) bool {
	jsonStringBytes := []byte(jsonStr)
	return json.Valid(jsonStringBytes)
}
