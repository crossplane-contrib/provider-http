package json

import (
	"bytes"
	"encoding/json"
)

func Contains(container, containee map[string]interface{}) bool {
	for key, value := range containee {
		if containerValue, exists := container[key]; !exists || !deepEqual(value, containerValue) {
			return false
		}
	}
	return true
}

func IsJSONString(jsonStr string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(jsonStr), &js) == nil
}

func JsonStringToMap(jsonStr string) map[string]interface{} {
	var jsonData map[string]interface{}
	_ = json.Unmarshal([]byte(jsonStr), &jsonData)
	return jsonData
}

// Converts JSON strings within a map to maps for JSON data processing.
func ConvertJSONStringsToMaps(merged *map[string]interface{}) {
	for key, value := range *merged {

		switch valueToHandle := value.(type) {
		case string:
			if IsJSONString(valueToHandle) {
				mappedJSON := JsonStringToMap(valueToHandle)
				(*merged)[key] = mappedJSON
			}
		case map[string]interface{}:
			ConvertJSONStringsToMaps(&valueToHandle)
		case []interface{}:
			structToMap, _ := (StructToMap(valueToHandle))
			ConvertJSONStringsToMaps(&structToMap)
		}
	}
}

func StructToMap(obj interface{}) (newMap map[string]interface{}, err error) {
	data, err := json.Marshal(obj) // Convert to a json string

	if err != nil {
		return
	}

	err = json.Unmarshal(data, &newMap) // Convert to a map
	return
}

func ConvertMapToJson(m map[string]interface{}) ([]byte, bool) {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return nil, false
	}

	return jsonData, true
}

func deepEqual(a, b interface{}) bool {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false
	}

	bBytes, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return bytes.Equal(aBytes, bBytes)
}
