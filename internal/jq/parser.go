package jq

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/itchyny/gojq"
)

const (
	errStringParseFailed = "failed to parse string: %s"
	errResultParseFailed = "failed to parse result on jq query: %s"
	errMapParseFailed    = "failed to parse map: %s"
	errQueryFailed       = "query should return at least one value, failed on: %s"
	errInvalidQuery      = "failed to parse given mapping - %s jq error: %s"
)

var mutex = &sync.Mutex{}

func runJQQuery(jqQuery string, obj interface{}, logger logging.Logger) (interface{}, error) {
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

func ParseString(jqQuery string, obj interface{}, logger logging.Logger) (string, error) {
	queryRes, err := runJQQuery(jqQuery, obj, logger)
	if err != nil {
		return "", err
	}

	str, ok := queryRes.(string)
	if !ok {
		return "", errors.Errorf(errStringParseFailed, fmt.Sprint(queryRes))
	}

	return str, nil
}

func ParseMapInterface(jqQuery string, obj interface{}, logger logging.Logger) (map[string]interface{}, error) {
	queryRes, err := runJQQuery(jqQuery, obj, logger)
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

func ParseMapStrings(keyToJQQueries map[string][]string, obj interface{}, logger logging.Logger) (map[string][]string, error) {
	result := make(map[string][]string, len(keyToJQQueries))

	for key, jqQueries := range keyToJQQueries {
		results := make([]string, len(jqQueries))

		for i, jqQuery := range jqQueries {
			queryRes, err := runJQQuery(jqQuery, obj, logger)
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
