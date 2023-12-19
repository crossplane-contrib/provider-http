package json

import (
	"testing"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	"github.com/google/go-cmp/cmp"
)

var (
	testPostMapping = v1alpha1.Mapping{
		Method: "POST",
		Body:   "{ username: .payload.body.username, email: .payload.body.email }",
		URL:    ".payload.baseUrl",
		// Headers: testHeaders,
	}

	testPutMapping = v1alpha1.Mapping{
		Method: "PUT",
		Body:   "{ username: \"john_doe_new_username\" }",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
		// Headers: testHeaders,
	}

	testGetMapping = v1alpha1.Mapping{
		Method: "GET",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testDeleteMapping = v1alpha1.Mapping{
		Method: "DELETE",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}
)

var (
	testForProvider = v1alpha1.RequestParameters{
		Payload: v1alpha1.Payload{
			Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
			BaseUrl: "https://api.example.com/users",
		},
		Mappings: []v1alpha1.Mapping{
			testPostMapping,
			testGetMapping,
			testPutMapping,
			testDeleteMapping,
		},
	}
)

func Test_Contains(t *testing.T) {
	type args struct {
		container map[string]interface{}
		containee map[string]interface{}
	}
	type want struct {
		result bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				container: map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
				containee: map[string]any{"username": "john_doe"},
			},
			want: want{
				result: true,
			},
		},
		"SuccessMatchesJsons": {
			args: args{
				container: map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
				containee: map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
			},
			want: want{
				result: true,
			},
		},
		"Fails": {
			args: args{
				container: map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
				containee: map[string]any{"false": "false.false@example.com"},
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Contains(tc.args.container, tc.args.containee)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("Contains(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_IsJSONString(t *testing.T) {
	type args struct {
		jsonStr string
	}
	type want struct {
		result bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"True": {
			args: args{
				jsonStr: `{"username":"john_doe","email":"john.doe@example.com"}`,
			},
			want: want{
				result: true,
			},
		},
		"False": {
			args: args{
				jsonStr: "hi",
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsJSONString(tc.args.jsonStr)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("IsJSONString(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_JsonStringToMap(t *testing.T) {
	type args struct {
		jsonStr string
	}
	type want struct {
		result map[string]interface{}
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				jsonStr: `{"username":"john_doe","email":"john.doe@example.com"}`,
			},
			want: want{
				result: map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := JsonStringToMap(tc.args.jsonStr)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("JsonStringToMap(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_ConvertJSONStringsToMaps(t *testing.T) {
	type args struct {
		merged map[string]interface{}
	}
	type want struct {
		result map[string]interface{}
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				merged: map[string]any{
					"payload": map[string]any{
						"baseUrl": "https://api.example.com/users",
						"body":    `{"username":"john_doe","email":"john.doe@example.com"}`,
					},
				},
			},
			want: want{
				result: map[string]any{
					"payload": map[string]any{
						"baseUrl": "https://api.example.com/users",
						"body":    map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ConvertJSONStringsToMaps(&tc.args.merged)
			if diff := cmp.Diff(tc.args.merged, tc.want.result); diff != "" {
				t.Fatalf("ConvertJSONStringsToMaps(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_StructToMap(t *testing.T) {
	type args struct {
		obj interface{}
	}
	type want struct {
		result     map[string]interface{}
		errMessage string
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				obj: testForProvider,
			},
			want: want{
				result: map[string]any{
					"mappings": []any{
						map[string]any{
							"body":   "{ username: .payload.body.username, email: .payload.body.email }",
							"method": "POST",
							"url":    ".payload.baseUrl",
						},
						map[string]any{
							"method": "GET",
							"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
						},
						map[string]any{
							"body":   `{ username: "john_doe_new_username" }`,
							"method": "PUT",
							"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
						},
						map[string]any{
							"method": "DELETE",
							"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
						},
					},
					"payload": map[string]any{
						"baseUrl": "https://api.example.com/users",
						"body":    `{"username": "john_doe", "email": "john.doe@example.com"}`,
					},
				},
				errMessage: "",
			},
		},
		"Fail": {
			args: args{
				obj: "",
			},
			want: want{
				result:     nil,
				errMessage: "json: cannot unmarshal string into Go value of type map[string]interface {}",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := StructToMap(tc.args.obj)
			if gotErr != nil {
				if diff := cmp.Diff(tc.want.errMessage, gotErr.Error()); diff != "" {
					t.Fatalf("e.StructToMap(...): -want error, +got error: %s", diff)
				}
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("StructToMap(...): -want result, +got result: %s", diff)
			}
		})
	}
}
