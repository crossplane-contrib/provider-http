package datapatcher

import (
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/common"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	json_util "github.com/crossplane-contrib/provider-http/internal/json"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestIsSecretDataUpToDate(t *testing.T) {
	type args struct {
		secret       *corev1.Secret
		secretKey    string
		valueToPatch string
	}

	type want struct {
		result bool
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldReturnSecretIsUpToDate": {
			args: args{
				secret:       createSpecificSecret("name", "namespace", "key", "value"),
				secretKey:    "key",
				valueToPatch: "value",
			},
			want: want{
				result: true,
			},
		},
		"ShouldReturnSecretIsNotUpToDate": {
			args: args{
				secret:       createSpecificSecret("name", "namespace", "key1", "value"),
				secretKey:    "key",
				valueToPatch: "value",
			},
			want: want{
				result: false,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got := isSecretDataUpToDate(tc.args.secret, tc.args.secretKey, tc.args.valueToPatch)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("isUpToDate(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func TestSyncMap(t *testing.T) {
	type args struct {
		existing map[string]string
		desired  map[string]string
		dataMap  map[string]interface{}
	}

	type want struct {
		changed  bool
		expected map[string]string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldAddAndUpdateKeys": {
			args: args{
				existing: map[string]string{
					"key1": "value1",
				},
				desired: map[string]string{
					"key1": "newValue1",
					"key2": "value2",
				},
				dataMap: map[string]interface{}{},
			},
			want: want{
				changed: true,
				expected: map[string]string{
					"key1": "newValue1",
					"key2": "value2",
				},
			},
		},
		"ShouldRemoveKeysNotInDesired": {
			args: args{
				existing: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				desired: map[string]string{
					"key1": "value1",
				},
				dataMap: map[string]interface{}{},
			},
			want: want{
				changed: true,
				expected: map[string]string{
					"key1": "value1",
				},
			},
		},
		"ShouldProcessJQQuery": {
			args: args{
				existing: map[string]string{
					"key1": "value1",
				},
				desired: map[string]string{
					"key1": ".key1", // JQ query
				},
				dataMap: map[string]interface{}{
					"key1": "newValueFromDataMap",
				},
			},
			want: want{
				changed: true,
				expected: map[string]string{
					"key1": "newValueFromDataMap",
				},
			},
		},
		"ShouldProcessJQQueryWithNumericValue": {
			args: args{
				existing: map[string]string{
					"status": "unknown",
				},
				desired: map[string]string{
					"status": ".body.status", // JQ query that returns a number
				},
				dataMap: map[string]interface{}{
					"body": map[string]interface{}{
						"status": float64(409), // Use float64 like JSON parsing would
					},
				},
			},
			want: want{
				changed: true,
				expected: map[string]string{
					"status": "409", // Should be converted to string
				},
			},
		},
		"ShouldSkipInvalidLabelValues": {
			args: args{
				existing: map[string]string{},
				desired: map[string]string{
					"valid-key":   "valid-value",
					"invalid-key": ".body.invalid", // JQ query that returns invalid label value
				},
				dataMap: map[string]interface{}{
					"body": map[string]interface{}{
						"invalid": "@invalid-label-value!",
					},
				},
			},
			want: want{
				changed: true,
				expected: map[string]string{
					"valid-key": "valid-value",
					// invalid-key should be skipped
				},
			},
		},
		"ShouldDoNothingIfUpToDate": {
			args: args{
				existing: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				desired: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				dataMap: map[string]interface{}{},
			},
			want: want{
				changed: false,
				expected: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			changed := syncMap(logging.NewNopLogger(), &tc.args.existing, tc.args.desired, tc.args.dataMap)

			if changed != tc.want.changed {
				t.Errorf("syncMap(...): expected changed = %v, got %v", tc.want.changed, changed)
			}

			if diff := cmp.Diff(tc.want.expected, tc.args.existing); diff != "" {
				t.Errorf("syncMap(...): -want map, +got map: %s", diff)
			}
		})
	}
}

func TestReplaceSensitiveValues(t *testing.T) {
	type args struct {
		data         *httpClient.HttpResponse
		secret       *corev1.Secret
		secretKey    string
		valueToPatch *string
	}

	type want struct {
		body    string
		headers map[string][]string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldReplaceSensitiveValueInBodyAndHeaders": {
			args: args{
				data: &httpClient.HttpResponse{
					Body: "Sensitive value is here.",
					Headers: map[string][]string{
						"Authorization": {"Bearer sensitive-value"},
						"Content-Type":  {"application/json"},
					},
				},
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "default",
					},
				},
				secretKey:    "sensitiveKey",
				valueToPatch: ptr.To("sensitive-value"),
			},
			want: want{
				body: "Sensitive value is here.",
				headers: map[string][]string{
					"Authorization": {"Bearer {{my-secret:default:sensitiveKey}}"},
					"Content-Type":  {"application/json"},
				},
			},
		},
		"ShouldDoNothingIfValueToPatchIsEmpty": {
			args: args{
				data: &httpClient.HttpResponse{
					Body: "Nothing to replace here.",
					Headers: map[string][]string{
						"Authorization": {"Bearer nothing"},
					},
				},
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "default",
					},
				},
				secretKey:    "sensitiveKey",
				valueToPatch: ptr.To(""),
			},
			want: want{
				body: "Nothing to replace here.",
				headers: map[string][]string{
					"Authorization": {"Bearer nothing"},
				},
			},
		},
		"ShouldHandleEmptyHeadersGracefully": {
			args: args{
				data: &httpClient.HttpResponse{
					Body: `{"message": "Sensitive value in the body", "token": "value"}`,
					Headers: map[string][]string{
						"Authorization": {},
					},
				},
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "default",
					},
				},
				secretKey:    "sensitiveKey",
				valueToPatch: ptr.To("value"),
			},
			want: want{
				body: `{"message": "Sensitive value in the body", "token": "{{my-secret:default:sensitiveKey}}"}`,
				headers: map[string][]string{
					"Authorization": {},
				},
			},
		},
		"ShouldReplaceJSONObjectInBody": {
			args: args{
				data: &httpClient.HttpResponse{
					Body: `{"credentials": {"client_id":"test_client","client_secret":"test_secret"}, "other": "data"}`,
					Headers: map[string][]string{
						"Content-Type": {"application/json"},
					},
				},
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: "default",
					},
				},
				secretKey:    "creds",
				valueToPatch: ptr.To(`{"client_id":"test_client","client_secret":"test_secret"}`),
			},
			want: want{
				body: `{"credentials": "{{my-secret:default:creds}}", "other": "data"}`,
				headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
		},
		"ShouldReplaceJSONObjectInHeaders": {
			args: args{
				data: &httpClient.HttpResponse{
					Body: "Normal body content",
					Headers: map[string][]string{
						"X-Auth-Data":  {`{"token":"abc123","expires":3600}`},
						"Content-Type": {"application/json"},
					},
				},
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "auth-secret",
						Namespace: "kube-system",
					},
				},
				secretKey:    "auth",
				valueToPatch: ptr.To(`{"token":"abc123","expires":3600}`),
			},
			want: want{
				body: "Normal body content",
				headers: map[string][]string{
					"X-Auth-Data":  {"{{auth-secret:kube-system:auth}}"},
					"Content-Type": {"application/json"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			replaceSensitiveValues(tc.args.data, tc.args.secret, tc.args.secretKey, tc.args.valueToPatch)

			if diff := cmp.Diff(tc.want.body, tc.args.data.Body); diff != "" {
				t.Errorf("Body mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.headers, tc.args.data.Headers); diff != "" {
				t.Errorf("Headers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsJSONObject(t *testing.T) {
	cases := map[string]struct {
		input    string
		expected bool
	}{
		"ValidJSONObject":           {`{"key":"value"}`, true},
		"ValidJSONObjectWithSpaces": {`  {"key":"value"}  `, true},
		"EmptyJSONObject":           {`{}`, true},
		"ComplexJSONObject":         {`{"client_id":"test","client_secret":"secret"}`, true},
		"JSONArray":                 {`["a","b","c"]`, false},
		"SimpleString":              {`"just a string"`, false},
		"PlainText":                 {`plain text`, false},
		"EmptyString":               {``, false},
		"OnlyOpeningBrace":          {`{`, false},
		"OnlyClosingBrace":          {`}`, false},
		"MismatchedBraces":          {`{"key":"value"]`, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := isJSONObject(tc.input)
			if result != tc.expected {
				t.Errorf("isJSONObject(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestUpdateSecretData(t *testing.T) {
	type args struct {
		secret               *corev1.Secret
		secretKey            string
		valueToPatch         *string
		missingFieldStrategy common.MissingFieldStrategy
	}

	type want struct {
		data map[string][]byte
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldAddKeyToEmptyData": {
			args: args{
				secret:       &corev1.Secret{},
				secretKey:    "key1",
				valueToPatch: ptr.To("value1"),
			},
			want: want{
				data: map[string][]byte{
					"key1": []byte("value1"),
				},
			},
		},
		"ShouldUpdateExistingKey": {
			args: args{
				secret: &corev1.Secret{
					Data: map[string][]byte{
						"key1": []byte("oldValue"),
					},
				},
				secretKey:    "key1",
				valueToPatch: ptr.To("newValue"),
			},
			want: want{
				data: map[string][]byte{
					"key1": []byte("newValue"),
				},
			},
		},
		"ShouldAddNewKeyToExistingData": {
			args: args{
				secret: &corev1.Secret{
					Data: map[string][]byte{
						"key1": []byte("value1"),
					},
				},
				secretKey:    "key2",
				valueToPatch: ptr.To("value2"),
			},
			want: want{
				data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
			},
		},
		"ShouldSetEmptyMissingFieldWhenFieldMissing": {
			// Secret already contains key "key1" but the response did not return a value;
			// missing field strategy "setEmpty" should override it to empty string.
			args: args{
				secret: &corev1.Secret{
					Data: map[string][]byte{
						"key1": []byte("existingValue"),
					},
				},
				secretKey:            "key1",
				valueToPatch:         nil,
				missingFieldStrategy: common.SetEmptyMissingField,
			},
			want: want{
				data: map[string][]byte{
					"key1": []byte(""),
				},
			},
		},
		"ShouldPreserveExistingValueWhenFieldMissing": {
			// Secret already contains key "key1" but the response did not return a value;
			// missing field strategy "preserve" should leave the value unchanged.
			args: args{
				secret: &corev1.Secret{
					Data: map[string][]byte{
						"key1": []byte("existingValue"),
					},
				},
				secretKey:            "key1",
				valueToPatch:         nil,
				missingFieldStrategy: common.PreserveMissingField,
			},
			want: want{
				data: map[string][]byte{
					"key1": []byte("existingValue"),
				},
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables
		t.Run(name, func(t *testing.T) {
			updateSecretData(tc.args.secret, tc.args.secretKey, tc.args.valueToPatch, tc.args.missingFieldStrategy)
			if diff := cmp.Diff(tc.want.data, tc.args.secret.Data); diff != "" {
				t.Errorf("updateSecretData(...): -want data, +got data: %s", diff)
			}
		})
	}
}

func TestExtractValueToPatch(t *testing.T) {
	type args struct {
		dataMap          map[string]interface{}
		requestFieldPath string
	}

	type want struct {
		result *string
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldExtractStringValue": {
			args: args{
				dataMap: map[string]interface{}{
					"stringField": "testString",
				},
				requestFieldPath: ".stringField",
			},
			want: want{
				result: ptr.To("testString"),
				err:    nil,
			},
		},
		"ShouldExtractBooleanValueAsString": {
			args: args{
				dataMap: map[string]interface{}{
					"booleanField": true,
				},
				requestFieldPath: ".booleanField",
			},
			want: want{
				result: ptr.To("true"),
				err:    nil,
			},
		},
		"ShouldExtractNumericValueAsString": {
			args: args{
				dataMap: map[string]interface{}{
					"numberField": 123.45,
				},
				requestFieldPath: ".numberField",
			},
			want: want{
				result: ptr.To("123.45"),
				err:    nil,
			},
		},
		"ShouldExtractNestedNumericValueAsString": {
			args: args{
				dataMap: map[string]interface{}{
					"body": map[string]interface{}{
						"status": float64(409), // Use float64 instead of int
					},
				},
				requestFieldPath: ".body.status",
			},
			want: want{
				result: ptr.To("409"),
				err:    nil,
			},
		},
		"ShouldReturnEmptyStringIfFieldNotFound": {
			args: args{
				dataMap: map[string]interface{}{
					"existingField": "value",
				},
				requestFieldPath: ".nonExistentField",
			},
			want: want{
				result: nil,
				err:    nil,
			},
		},
		"ShouldReturnEmptyStringIfUnsupportedType": {
			args: args{
				dataMap: map[string]interface{}{
					"arrayField": []string{"value1", "value2"},
				},
				requestFieldPath: ".arrayField",
			},
			want: want{
				result: nil,
				err:    nil,
			},
		},
		"ShouldExtractMapAsJSONString": {
			args: args{
				dataMap: map[string]interface{}{
					"credentials": map[string]interface{}{
						"client_id":     "test_client",
						"client_secret": "test_secret",
					},
				},
				requestFieldPath: ".credentials",
			},
			want: want{
				result: ptr.To(`{"client_id":"test_client","client_secret":"test_secret"}`),
				err:    nil,
			},
		},
		"ShouldExtractNestedMapAsJSONString": {
			args: args{
				dataMap: map[string]interface{}{
					"response": map[string]interface{}{
						"body": map[string]interface{}{
							"auth": map[string]interface{}{
								"token":   "abc123",
								"expires": 3600,
							},
						},
					},
				},
				requestFieldPath: ".response.body.auth",
			},
			want: want{
				result: ptr.To(`{"expires":3600,"token":"abc123"}`),
				err:    nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := extractValueToPatch(logging.NewNopLogger(), tc.args.dataMap, tc.args.requestFieldPath)

			if diff := cmp.Diff(tc.want.result, result); diff != "" {
				t.Errorf("extractValueToPatch(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func TestPrepareDataMap(t *testing.T) {
	type args struct {
		data *httpClient.HttpResponse
	}

	type want struct {
		result map[string]interface{}
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldConvertHttpResponseToMap": {
			args: args{
				data: &httpClient.HttpResponse{
					Body: `{"key1": "value1", "key2": {"subkey": "subvalue"}}`,
					Headers: map[string][]string{
						"Content-Type": {"application/json"},
					},
				},
			},
			want: want{
				result: map[string]interface{}{
					"body": map[string]interface{}{
						"key1": "value1",
						"key2": map[string]interface{}{
							"subkey": "subvalue",
						},
					},
					"headers": map[string]interface{}{
						"Content-Type": []any{"application/json"},
					},
					"statusCode": float64(0),
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := prepareDataMap(tc.args.data)

			if diff := cmp.Diff(tc.want.result, result); diff != "" {
				t.Errorf("prepareDataMap(...): -want result, +got result: %s", diff)
			}

			if (err != nil || tc.want.err != nil) && (err == nil || tc.want.err == nil || err.Error() != tc.want.err.Error()) {
				t.Errorf("prepareDataMap(...): expected err = %v, got %v", tc.want.err, err)
			}
		})
	}
}

func TestIsValidLabelValue(t *testing.T) {
	cases := map[string]struct {
		value string
		want  bool
	}{
		"EmptyString": {
			value: "",
			want:  true,
		},
		"ValidAlphanumeric": {
			value: "409",
			want:  true,
		},
		"ValidWithDash": {
			value: "app-name",
			want:  true,
		},
		"ValidWithUnderscore": {
			value: "app_name",
			want:  true,
		},
		"ValidWithDot": {
			value: "app.name",
			want:  true,
		},
		"ValidMixed": {
			value: "app-1_test.2",
			want:  true,
		},
		"InvalidStartsWithDash": {
			value: "-invalid",
			want:  false,
		},
		"InvalidEndsWithDash": {
			value: "invalid-",
			want:  false,
		},
		"InvalidSpecialChars": {
			value: "app@name",
			want:  false,
		},
		"InvalidTooLong": {
			value: "this-is-a-very-long-label-value-that-exceeds-the-maximum-length-limit-of-63-characters-for-kubernetes-labels",
			want:  false,
		},
		"ValidExactly63Chars": {
			value: "this-is-exactly-63-characters-long-and-should-be-valid-12345",
			want:  true,
		},
		"SingleChar": {
			value: "a",
			want:  true,
		},
		"SingleDigit": {
			value: "1",
			want:  true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := isValidLabelValue(tc.value)
			if got != tc.want {
				t.Errorf("isValidLabelValue(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestUpdateSecretLabelsAndAnnotationsEndToEnd(t *testing.T) {
	// This test simulates the complete flow from HTTP response to secret metadata update
	response := &httpClient.HttpResponse{
		Body: `{"message": "created", "status": 201, "data": {"id": "test-123"}}`,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		StatusCode: 201,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}

	labels := map[string]string{
		"status-code": ".statusCode",   // Should get 201 from StatusCode (note: camelCase)
		"app-status":  ".body.status",  // Should get 201 from body
		"static-val":  "my-app",        // Static value
		"msg-prefix":  ".body.message", // Should get "created"
	}

	annotations := map[string]string{
		"response-id": ".body.data.id", // Should get "test-123"
		"http-method": "POST",          // Static value for annotations
	}

	// Prepare data map like the real function does
	dataMap, err := json_util.StructToMap(response)
	if err != nil {
		t.Fatalf("Failed to prepare data map: %v", err)
	}
	json_util.ConvertJSONStringsToMaps(&dataMap)

	// Initialize secret maps
	secret.Labels = make(map[string]string)
	secret.Annotations = make(map[string]string)

	// Test labels
	labelsChanged := syncMap(logging.NewNopLogger(), &secret.Labels, labels, dataMap)
	annotationsChanged := syncMap(logging.NewNopLogger(), &secret.Annotations, annotations, dataMap)

	if !labelsChanged {
		t.Error("Expected labels to be changed")
	}
	if !annotationsChanged {
		t.Error("Expected annotations to be changed")
	}

	// Verify results
	expectedLabels := map[string]string{
		"status-code": "201",
		"app-status":  "201",
		"static-val":  "my-app",
		"msg-prefix":  "created",
	}

	expectedAnnotations := map[string]string{
		"response-id": "test-123",
		"http-method": "POST",
	}

	if diff := cmp.Diff(expectedLabels, secret.Labels); diff != "" {
		t.Errorf("Labels mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(expectedAnnotations, secret.Annotations); diff != "" {
		t.Errorf("Annotations mismatch (-want +got):\n%s", diff)
	}
}
