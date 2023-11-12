package json

import "encoding/json"

func Contains(container, containee map[string]interface{}) bool {
	for key, value := range containee {
		if containerValue, exists := container[key]; !exists || !deepEqual(value, containerValue) {
			return false
		}
	}
	return true
}

func isJSONString(jsonStr string) bool {
	jsonStringBytes := []byte(jsonStr)
	return json.Valid(jsonStringBytes)
}

func JsonStringToMap(jsonStr string) (map[string]interface{}, error) {
	var jsonData map[string]interface{}

	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}

// Converts JSON strings within a map to maps for JSON data processing.
func ConvertJSONStringsToMaps(merged *map[string]interface{}) {
	for key, value := range *merged {

		switch valueToHandle := value.(type) {
		case string:
			if isJSONString(valueToHandle) {
				mappedJSON, _ := JsonStringToMap(valueToHandle)
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

func deepEqual(a, b interface{}) bool {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false
	}

	bBytes, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return string(aBytes) == string(bBytes)
}
