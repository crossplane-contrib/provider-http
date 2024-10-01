package json

import (
	"bytes"
	"encoding/json"
)

// Contains checks if the containee map is contained within the container map, including nested JSON structures.
func Contains(container, containee map[string]interface{}) bool {
	for key, value := range containee {
		containerValue, exists := container[key]
		if !exists {
			return false
		}
		if nestedMap, ok := value.(map[string]interface{}); ok {
			if containerNestedMap, ok := containerValue.(map[string]interface{}); ok {
				if !Contains(containerNestedMap, nestedMap) {
					return false
				}
			} else {
				return false
			}
		} else if !deepEqual(value, containerValue) {
			return false
		}
	}
	return true
}

// IsJSONString checks if a given string is a valid JSON.
func IsJSONString(jsonStr string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(jsonStr), &js) == nil
}

// JsonStringToMap converts a JSON string to a map.
func JsonStringToMap(jsonStr string) map[string]interface{} {
	var jsonData map[string]interface{}
	_ = json.Unmarshal([]byte(jsonStr), &jsonData)
	return jsonData
}

// ConvertJSONStringsToMaps converts JSON strings within a map to maps for JSON data processing.
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
			structToMap, _ := StructToMap(valueToHandle)
			ConvertJSONStringsToMaps(&structToMap)
		}
	}
}

// StructToMap converts a struct to a map.
func StructToMap(obj interface{}) (newMap map[string]interface{}, err error) {
	data, err := json.Marshal(obj) // Convert to a JSON string
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &newMap) // Convert to a map
	return
}

// ConvertMapToJson converts a map to a JSON string.
func ConvertMapToJson(m map[string]interface{}) (string, error) {
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// deepEqual checks if two interfaces are deeply equal by comparing their JSON representations.
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
