package requestprocessing

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/arielsepton/provider-http/internal/jq"
	json_util "github.com/arielsepton/provider-http/internal/json"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"golang.org/x/exp/maps"
)

func ConvertStringToJQQuery(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

// This function receives a jq query format and Request and returns the jq result.
func ApplyJQOnStr(jqQuery string, request *v1alpha1.Request, logger logging.Logger) (string, error) {
	baseMap := generateJQObject(request)

	if result, _ := jq.ParseMapInterface(jqQuery, baseMap, logger); result != nil {
		transformedData, err := json.Marshal(result)
		if err != nil {
			return "", err
		}
		return string(transformedData), nil
	}

	stringResult, err := jq.ParseString(jqQuery, baseMap, logger)
	if err != nil {
		return "", err
	}

	return stringResult, nil
}

// This function receives a jq query format and Requet and returns the jq result.
func ApplyJQOnMap(keyToJQQueries map[string][]string, request *v1alpha1.Request, logger logging.Logger) (map[string][]string, error) {
	mapInterface := make(map[string][]string, len(keyToJQQueries))

	for key, jqQueries := range keyToJQQueries {
		results := make([]string, len(jqQueries))

		for i, jqQuery := range jqQueries {
			queryRes, err := ApplyJQOnStr(jqQuery, request, logger)

			if err != nil {
				logger.Debug(fmt.Sprint(err))

				return nil, err
			}

			results[i] = queryRes
		}

		mapInterface[key] = results
	}

	return mapInterface, nil
}

func generateJQObject(request *v1alpha1.Request) map[string]interface{} {
	baseMap, _ := json_util.StructToMap(request.Spec.ForProvider)
	statusMap, _ := json_util.StructToMap(request.Status)

	maps.Copy(baseMap, statusMap)
	json_util.ConvertJSONStringsToMaps(&baseMap)

	return baseMap
}
