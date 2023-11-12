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

func IsJSONString(jsonStr string) bool {
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
