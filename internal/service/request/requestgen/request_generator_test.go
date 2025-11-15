package requestgen

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

var testHeaders = map[string][]string{
	"fruits":                {"apple", "banana", "orange"},
	"colors":                {"red", "green", "blue"},
	"countries":             {"USA", "UK", "India", "Germany"},
	"programming_languages": {"Go", "Python", "JavaScript"},
}

var testHeaders2 = map[string][]string{
	"countries": {"USA", "UK", "India", "Germany"},
}

var (
	testPostMapping = v1alpha2.Mapping{
		Method:  "POST",
		Body:    "{ username: .payload.body.username, email: .payload.body.email }",
		URL:     ".payload.baseUrl",
		Headers: testHeaders,
	}

	testPutMapping = v1alpha2.Mapping{
		Method:  "PUT",
		Body:    "{ username: \"john_doe_new_username\" }",
		URL:     "(.payload.baseUrl + \"/\" + .response.body.id)",
		Headers: testHeaders,
	}

	testGetMapping = v1alpha2.Mapping{
		Method: "GET",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testDeleteMapping = v1alpha2.Mapping{
		Method: "DELETE",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}
)

var (
	testForProvider = v1alpha2.RequestParameters{
		Payload: v1alpha2.Payload{
			Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
			BaseUrl: "https://api.example.com/users",
		},
		Mappings: []v1alpha2.Mapping{
			testPostMapping,
			testGetMapping,
			testPutMapping,
			testDeleteMapping,
		},
		ExpectedResponseCheck: v1alpha2.ExpectedResponseCheck{
			Type:  v1alpha2.ExpectedResponseCheckTypeCustom,
			Logic: "logic example",
		},
		IsRemovedCheck: v1alpha2.ExpectedResponseCheck{
			Type:  v1alpha2.ExpectedResponseCheckTypeCustom,
			Logic: "logic example",
		},
	}
)

func Test_GenerateRequestDetails(t *testing.T) {
	type args struct {
		methodMapping v1alpha2.Mapping
		forProvider   v1alpha2.RequestParameters
		response      v1alpha2.Response
		logger        logging.Logger
		localKube     client.Client
	}
	type want struct {
		requestDetails RequestDetails
		err            error
		ok             bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessPost": {
			args: args{
				methodMapping: testPostMapping,
				forProvider:   testForProvider,
				response:      v1alpha2.Response{},
				logger:        logging.NewNopLogger(),
			},
			want: want{
				requestDetails: RequestDetails{
					Url: "https://api.example.com/users",
					Body: httpClient.Data{
						Encrypted: `{"email":"john.doe@example.com","username":"john_doe"}`,
						Decrypted: `{"email":"john.doe@example.com","username":"john_doe"}`,
					},
					Headers: httpClient.Data{
						Decrypted: testHeaders,
						Encrypted: testHeaders,
					},
				},
				err: nil,
				ok:  true,
			},
		},
		"SuccessPut": {
			args: args{
				methodMapping: testPutMapping,
				forProvider:   testForProvider,
				response: v1alpha2.Response{
					StatusCode: 200,
					Body:       `{"id":"123","username":"john_doe"}`,
					Headers:    testHeaders,
				},
				logger: logging.NewNopLogger(),
			},
			want: want{
				requestDetails: RequestDetails{
					Url: "https://api.example.com/users/123",
					Body: httpClient.Data{
						Encrypted: `{"username":"john_doe_new_username"}`,
						Decrypted: `{"username":"john_doe_new_username"}`,
					},
					Headers: httpClient.Data{
						Decrypted: testHeaders,
						Encrypted: testHeaders,
					},
				},
				err: nil,
				ok:  true,
			},
		},
		"SuccessDelete": {
			args: args{
				methodMapping: testDeleteMapping,
				forProvider:   testForProvider,
				response: v1alpha2.Response{
					StatusCode: 200,
					Body:       `{"id":"123","username":"john_doe"}`,
					Headers:    testHeaders,
				},
				logger: logging.NewNopLogger(),
			},
			want: want{
				requestDetails: RequestDetails{
					Url: "https://api.example.com/users/123",
					Headers: httpClient.Data{
						Decrypted: map[string][]string{},
						Encrypted: map[string][]string{},
					},
					Body: httpClient.Data{
						Decrypted: "",
						Encrypted: "",
					},
				},
				err: nil,
				ok:  true,
			},
		},
		"SuccessGet": {
			args: args{
				methodMapping: testGetMapping,
				forProvider:   testForProvider,
				response: v1alpha2.Response{
					StatusCode: 200,
					Body:       `{"id":"123","username":"john_doe"}`,
					Headers:    testHeaders,
				},
				logger: logging.NewNopLogger(),
			},
			want: want{
				requestDetails: RequestDetails{
					Url: "https://api.example.com/users/123",
					Headers: httpClient.Data{
						Decrypted: map[string][]string{},
						Encrypted: map[string][]string{},
					},
					Body: httpClient.Data{
						Decrypted: "",
						Encrypted: "",
					},
				},
				err: nil,
				ok:  true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svcCtx := service.NewServiceContext(context.Background(), tc.args.localKube, tc.args.logger, nil)
			got, gotErr, ok := GenerateRequestDetails(svcCtx, &tc.args.methodMapping, &tc.args.forProvider, &tc.args.response)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("GenerateRequestDetails(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Fatalf("GenerateRequestDetails(...): -want ok, +got ok: %s", diff)
			}

			if diff := cmp.Diff(tc.want.requestDetails, got); diff != "" {
				t.Errorf("GenerateRequestDetails(...): -want result, +got result: %s", diff)
			}
		})
	}

}

func Test_IsRequestValid(t *testing.T) {
	type args struct {
		requestDetails RequestDetails
	}
	type want struct {
		ok bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"ValidRequestDetails": {
			args: args{
				requestDetails: RequestDetails{
					Body: httpClient.Data{
						Encrypted: `{"id": "123", "username": "john_doe"}`,
						Decrypted: `{"id": "123", "username": "john_doe"}`,
					},
					Headers: httpClient.Data{
						Decrypted: nil,
						Encrypted: nil,
					},
					Url: "https://example",
				},
			},
			want: want{
				ok: true,
			},
		},
		"NonValidRequestDetails": {
			args: args{
				requestDetails: RequestDetails{
					Body: httpClient.Data{
						Encrypted: "",
						Decrypted: "",
					},
					Headers: httpClient.Data{
						Decrypted: nil,
						Encrypted: nil,
					},
					Url: "",
				},
			},
			want: want{
				ok: false,
			},
		},
		"NonValidUrl": {
			args: args{
				requestDetails: RequestDetails{
					Body: httpClient.Data{
						Encrypted: `{"id": "123", "username": "john_doe"}`,
						Decrypted: `{"id": "123", "username": "john_doe"}`,
					},
					Headers: httpClient.Data{
						Decrypted: nil,
						Encrypted: nil,
					},
					Url: "",
				},
			},
			want: want{
				ok: false,
			},
		},
		"NonValidBody": {
			args: args{
				requestDetails: RequestDetails{
					Body: httpClient.Data{
						Encrypted: `{"id": "null", "username": "john_doe"}`,
						Decrypted: `{"id": "null", "username": "john_doe"}`,
					},
					Headers: httpClient.Data{
						Decrypted: nil,
						Encrypted: nil,
					},
					Url: "https://example",
				},
			},
			want: want{
				ok: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsRequestValid(tc.args.requestDetails)
			if diff := cmp.Diff(tc.want.ok, got); diff != "" {
				t.Fatalf("IsRequestValid(...): -want bool, +got bool: %s", diff)
			}
		})
	}

}

func Test_coalesceHeaders(t *testing.T) {
	type args struct {
		mapping v1alpha2.Mapping
		spec    v1alpha2.RequestParameters
	}
	type want struct {
		headers map[string][]string
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"NonNilMappingHeaders": {
			args: args{
				mapping: v1alpha2.Mapping{Headers: testHeaders},
				spec:    v1alpha2.RequestParameters{Headers: testHeaders2},
			},
			want: want{
				headers: testHeaders,
			},
		},
		"NilMappingHeaders": {
			args: args{
				mapping: v1alpha2.Mapping{Headers: nil},
				spec:    v1alpha2.RequestParameters{Headers: testHeaders2},
			},
			want: want{
				headers: testHeaders2,
			},
		},
		"NilDefaultHeaders": {
			args: args{
				mapping: v1alpha2.Mapping{Headers: testHeaders},
				spec:    v1alpha2.RequestParameters{Headers: nil},
			},
			want: want{
				headers: testHeaders,
			},
		},
		"NilHeaders": {
			args: args{
				mapping: v1alpha2.Mapping{Headers: nil},
				spec:    v1alpha2.RequestParameters{Headers: nil},
			},
			want: want{
				headers: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := coalesceHeaders(&tc.args.mapping, &tc.args.spec)
			if diff := cmp.Diff(tc.want.headers, got); diff != "" {
				t.Fatalf("coalesceHeaders(...): -want headers, +got headers: %s", diff)
			}
		})
	}
}

func Test_generateRequestObject(t *testing.T) {
	type args struct {
		forProvider v1alpha2.RequestParameters
		response    v1alpha2.Response
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
				forProvider: testForProvider,
				response: v1alpha2.Response{
					StatusCode: 200,
					Body:       `{"id": "123"}`,
					Headers:    nil,
				},
			},
			want: want{
				result: map[string]any{
					"expectedResponseCheck": map[string]any{
						"type":  v1alpha2.ExpectedResponseCheckTypeCustom,
						"logic": "logic example",
					},
					"isRemovedCheck": map[string]any{
						"type":  v1alpha2.ExpectedResponseCheckTypeCustom,
						"logic": "logic example",
					},
					"mappings": []any{
						map[string]any{
							"body":   "{ username: .payload.body.username, email: .payload.body.email }",
							"method": "POST",
							"headers": map[string]any{
								"colors":                []any{"red", "green", "blue"},
								"countries":             []any{"USA", "UK", "India", "Germany"},
								"fruits":                []any{"apple", "banana", "orange"},
								"programming_languages": []any{"Go", "Python", "JavaScript"},
							},
							"url": ".payload.baseUrl",
						},
						map[string]any{
							"method": "GET",
							"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
						},
						map[string]any{
							"body":   `{ username: "john_doe_new_username" }`,
							"method": "PUT",
							"headers": map[string]any{
								"colors":                []any{"red", "green", "blue"},
								"countries":             []any{"USA", "UK", "India", "Germany"},
								"fruits":                []any{"apple", "banana", "orange"},
								"programming_languages": []any{"Go", "Python", "JavaScript"},
							},
							"url": `(.payload.baseUrl + "/" + .response.body.id)`,
						},
						map[string]any{
							"method": "DELETE",
							"url":    `(.payload.baseUrl + "/" + .response.body.id)`,
						},
					},
					"payload": map[string]any{
						"baseUrl": "https://api.example.com/users",
						"body":    map[string]any{"email": "john.doe@example.com", "username": "john_doe"},
					},
					"response": map[string]any{
						"body":       map[string]any{"id": "123"},
						"headers":    nil,
						"statusCode": float64(200),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GenerateRequestContext(&tc.args.forProvider, &tc.args.response)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("generateRequestObject(...): -want result, +got result: %s", diff)
			}
		})
	}
}
