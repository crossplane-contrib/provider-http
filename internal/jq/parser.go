package jq

import (
	"fmt"
	"sync"

	"github.com/itchyny/gojq"
)

var mutex = &sync.Mutex{}

func runJQQuery(jqQuery string, obj interface{}) (interface{}, error) {
	query, err := gojq.Parse(jqQuery)
	if err != nil {
		return nil, err
	}

	mutex.Lock()
	queryRes, ok := query.Run(obj).Next()
	mutex.Unlock()

	if !ok {
		return nil, fmt.Errorf("query should return at least one value")
	}

	err, ok = queryRes.(error)
	if ok {
		return nil, err
	}

	return queryRes, nil
}

func ParseString(jqQuery string, obj interface{}) (string, error) {
	queryRes, err := runJQQuery(jqQuery, obj)
	if err != nil {
		return "", err
	}

	str, ok := queryRes.(string)
	if !ok {
		return "", fmt.Errorf("failed to parse string: %#v", queryRes)
	}

	return str, nil
}

func ParseMapInterface(jqQuery string, obj interface{}) (map[string]interface{}, error) {
	queryRes, err := runJQQuery(jqQuery, obj)
	if err != nil {
		return nil, err
	}

	mapInterface := map[string]interface{}{}

	if obj, ok := queryRes.(map[string]interface{}); ok {
		for key, value := range obj {
			mapInterface[key] = value
		}
	}

	return mapInterface, nil
}
