package jq

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/itchyny/gojq"
)

const (
	errStringParseFailed = "failed to parse string: %s"
	errFloatParseFailed  = "failed to parse float: %s"
	errResultParseFailed = "failed to parse result on jq query: %s"
	errMapParseFailed    = "failed to parse map: %s"
	errQueryFailed       = "query should return at least one value, failed on: %s"
	errInvalidQuery      = "failed to parse given mapping - %s jq error: %s"
)

var mutex = &sync.Mutex{}

// runJQQuery runs a jq query on a given object and returns the result.
func runJQQuery(jqQuery string, obj interface{}) (interface{}, error) {
	query, err := gojq.Parse(jqQuery)
	if err != nil {
		return nil, err
	}

	mutex.Lock()
	queryRes, ok := query.Run(obj).Next()
	mutex.Unlock()

	if !ok {
		return nil, errors.Errorf(errQueryFailed, fmt.Sprint(queryRes))
	}

	err, ok = queryRes.(error)
	if ok {
		return nil, errors.Errorf(errInvalidQuery, jqQuery, err.Error())
	}

	return queryRes, nil
}

// ParseString runs a jq query on a given object and returns the result as a string.
func ParseString(jqQuery string, obj interface{}) (string, error) {
	queryRes, err := runJQQuery(jqQuery, obj)
	if err != nil {
		return "", err
	}

	str, ok := queryRes.(string)
	if !ok {
		return "", errors.Errorf(errStringParseFailed, fmt.Sprint(queryRes))
	}

	return str, nil
}

// ParseFloat runs a jq query on a given object and returns the result as a float64.
func ParseFloat(jqQuery string, obj interface{}) (float64, error) {
	queryRes, err := runJQQuery(jqQuery, obj)
	if err != nil {
		return 0, err
	}

	floatVal, ok := queryRes.(float64)
	if !ok {
		return 0, errors.Errorf(errFloatParseFailed, fmt.Sprint(queryRes))
	}

	return floatVal, nil
}

// ParseBool runs a jq query on a given object and returns the result as a bool.
func ParseBool(jqQuery string, obj interface{}) (bool, error) {
	queryRes, err := runJQQuery(jqQuery, obj)
	if err != nil {
		return false, err
	}

	boolean, ok := queryRes.(bool)
	if !ok {
		return false, errors.Errorf(errStringParseFailed, fmt.Sprint(queryRes))
	}

	return boolean, nil
}

// ParseMapInterface runs a jq query on a given object and returns the result as a map[string]interface{}.
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

		return mapInterface, nil
	}

	return nil, errors.Errorf(errMapParseFailed, fmt.Sprint(queryRes))
}

// ParseMapStrings runs a jq query on a given object and returns the result as a map[string][]string.
func ParseMapStrings(keyToJQQueries map[string][]string, obj interface{}) (map[string][]string, error) {
	result := make(map[string][]string, len(keyToJQQueries))

	for key, jqQueries := range keyToJQQueries {
		results := make([]string, len(jqQueries))

		for i, jqQuery := range jqQueries {
			queryRes, err := runJQQuery(jqQuery, obj)
			if err != nil {
				// Use the original query as a fallback
				results[i] = jqQuery
				continue
			}

			str, ok := queryRes.(string)
			if !ok {
				// Raise an error if the result is not a string
				return nil, errors.Errorf(errResultParseFailed, fmt.Sprint(queryRes))
			}

			results[i] = str
		}

		result[key] = results
	}

	return result, nil
}

// IsJQQuery checks if a given string is a valid jq query.
// It attempts to compile the string as a jq expression and returns true if successful.
func IsJQQuery(query string) bool {
	_, err := gojq.Parse(query)
	return err == nil
}

// Exists checks if the given jq query returns a non-nil value from the object.
// It returns true if the field exists, false otherwise.
func Exists(jqQuery string, obj interface{}) (bool, error) {
	query, err := gojq.Parse(jqQuery)
	if err != nil {
		return false, err
	}

	mutex.Lock()
	iter := query.Run(obj)
	result, ok := iter.Next()
	mutex.Unlock()

	if !ok || result == nil {
		return false, nil
	}

	if errResult, isErr := result.(error); isErr {
		return false, errResult
	}
	return true, nil
}
