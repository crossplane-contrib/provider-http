package jq

import (
	"errors"
	"fmt"
	"sync"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/itchyny/gojq"
)

const (
	errStringParseFailed = "failed to parse string:"
	errResultParseFailed = "failed to parse result on jq query: "
	errMapParseFailed    = "failed to parse map:"
	errQueryFailed       = "query should return at least one value"
	errInvalidQuery      = "failed to parse given mapping -"
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
		return nil, errors.New(fmt.Sprint(errQueryFailed, queryRes))
	}

	err, ok = queryRes.(error)
	if ok {
		return nil, errors.New(fmt.Sprint(errInvalidQuery, " ", jqQuery, " jq error: ", err.Error()))
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
		return "", errors.New(fmt.Sprint(errStringParseFailed, queryRes))
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

	return nil, errors.New(fmt.Sprint(errMapParseFailed, queryRes))
}

func ParseMapStrings(keyToJQQueries map[string][]string, obj interface{}, logger logging.Logger) (map[string][]string, error) {
	result := make(map[string][]string, len(keyToJQQueries))

	for key, jqQueries := range keyToJQQueries {
		results := make([]string, len(jqQueries))

		for i, jqQuery := range jqQueries {
			queryRes, err := runJQQuery(jqQuery, obj, logger)
			if err != nil {
				// Log the error and use the original query as a fallback
				logger.Debug("Error parsing query: ", jqQuery, " using the original value instead.")
				results[i] = jqQuery
				continue
			}

			str, ok := queryRes.(string)
			if !ok {
				// Raise an error if the result is not a string
				return nil, errors.New(fmt.Sprint(errResultParseFailed, jqQuery))
			}

			results[i] = str
		}

		result[key] = results
	}

	return result, nil
}
