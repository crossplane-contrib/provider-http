package datapatcher

import (
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/common"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
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
					Body: "Sensitive value in the body.",
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
				body: "Sensitive {{my-secret:default:sensitiveKey}} in the body.",
				headers: map[string][]string{
					"Authorization": {},
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

func TestUpdateSecretData(t *testing.T) {
	type args struct {
		secret          *corev1.Secret
		secretKey       string
		valueToPatch    *string
		missingStrategy common.MissingFieldStrategy
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			updateSecretData(tc.args.secret, tc.args.secretKey, tc.args.valueToPatch, tc.args.missingStrategy)

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
