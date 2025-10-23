package requestmapping

import (
	"net/http"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

var (
	testPostMapping = v1alpha2.Mapping{
		Method: "POST",
		Action: v1alpha2.ActionCreate,
		Body:   "{ username: .payload.body.username, email: .payload.body.email }",
		URL:    ".payload.baseUrl",
	}

	testPutMapping = v1alpha2.Mapping{
		Method: "PUT",
		Action: v1alpha2.ActionUpdate,
		Body:   "{ username: \"john_doe_new_username\" }",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testGetMapping = v1alpha2.Mapping{
		Method: "GET",
		Action: v1alpha2.ActionObserve,
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testDeleteMapping = v1alpha2.Mapping{
		Method: "DELETE",
		Action: v1alpha2.ActionRemove,
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}
)

func Test_getMappingByMethod(t *testing.T) {
	type args struct {
		requestParams *v1alpha2.RequestParameters
		method        string
	}
	type want struct {
		mapping *v1alpha2.Mapping
		ok      bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Fail": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
					Payload: v1alpha2.Payload{
						Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
						BaseUrl: "https://api.example.com/users",
					},
					Mappings: []v1alpha2.Mapping{
						testGetMapping,
						testPutMapping,
						testDeleteMapping,
					},
				},
				method: "POST",
			},
			want: want{
				mapping: nil,
				ok:      false,
			},
		},
		"Success": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
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
				},
				method: "POST",
			},
			want: want{
				mapping: &testPostMapping,
				ok:      true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := getMappingByMethod(tc.args.requestParams, tc.args.method)
			if diff := cmp.Diff(tc.want.mapping, got); diff != "" {
				t.Fatalf("getMappingByMethod(...): -want result, +got result: %s", diff)
			}

			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Fatalf("getMappingByMethod(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_getMappingByAction(t *testing.T) {
	type args struct {
		requestParams *v1alpha2.RequestParameters
		action        string
	}
	type want struct {
		mapping *v1alpha2.Mapping
		ok      bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Fail": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
					Payload: v1alpha2.Payload{
						Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
						BaseUrl: "https://api.example.com/users",
					},
					Mappings: []v1alpha2.Mapping{
						testGetMapping,
						testPutMapping,
						testDeleteMapping,
					},
				},
				action: v1alpha2.ActionCreate,
			},
			want: want{
				mapping: nil,
				ok:      false,
			},
		},
		"Success": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
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
				},
				action: v1alpha2.ActionCreate,
			},
			want: want{
				mapping: &testPostMapping,
				ok:      true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := getMappingByAction(tc.args.requestParams, tc.args.action)
			if diff := cmp.Diff(tc.want.mapping, got); diff != "" {
				t.Fatalf("getMappingByAction(...): -want result, +got result: %s", diff)
			}

			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Fatalf("getMappingByAction(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_GetMapping(t *testing.T) {
	type args struct {
		requestParams *v1alpha2.RequestParameters
		action        string
	}
	type want struct {
		mapping *v1alpha2.Mapping
		err     error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Fail": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
					Payload: v1alpha2.Payload{
						Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
						BaseUrl: "https://api.example.com/users",
					},
					Mappings: []v1alpha2.Mapping{
						testGetMapping,
						testPutMapping,
						testDeleteMapping,
					},
				},
				action: v1alpha2.ActionCreate,
			},
			want: want{
				mapping: nil,
				err:     errors.Errorf(ErrMappingNotFound, v1alpha2.ActionCreate, http.MethodPost),
			},
		},
		"Success": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
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
				},
				action: v1alpha2.ActionCreate,
			},
			want: want{
				mapping: &testPostMapping,
				err:     nil,
			},
		},
		"SuccessWithoutMethod": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
					Payload: v1alpha2.Payload{
						Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
						BaseUrl: "https://api.example.com/users",
					},
					Mappings: []v1alpha2.Mapping{
						{
							Action: v1alpha2.ActionCreate,
							Body:   "{ username: .payload.body.username, email: .payload.body.email }",
							URL:    ".payload.baseUrl",
						},
						testGetMapping,
						testPutMapping,
						testDeleteMapping,
					},
				},
				action: v1alpha2.ActionCreate,
			},
			want: want{
				mapping: &testPostMapping,
				err:     nil,
			},
		},
		"SuccessWithoutAction": {
			args: args{
				requestParams: &v1alpha2.RequestParameters{
					Payload: v1alpha2.Payload{
						Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
						BaseUrl: "https://api.example.com/users",
					},
					Mappings: []v1alpha2.Mapping{
						{
							Method: http.MethodPost,
							Body:   "{ username: .payload.body.username, email: .payload.body.email }",
							URL:    ".payload.baseUrl",
						},
						testGetMapping,
						testPutMapping,
						testDeleteMapping,
					},
				},
				action: v1alpha2.ActionCreate,
			},
			want: want{
				mapping: &v1alpha2.Mapping{
					Method: http.MethodPost,
					Body:   "{ username: .payload.body.username, email: .payload.body.email }",
					URL:    ".payload.baseUrl",
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := GetMapping(tc.args.requestParams, tc.args.action, logging.NewNopLogger())
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("isUpToDate(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.mapping, got); diff != "" {
				t.Fatalf("GetMapping(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_getDefaultMethodByAction(t *testing.T) {
	type args struct {
		action string
	}
	type want struct {
		method string
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldReturnPostMethod": {
			args: args{
				action: v1alpha2.ActionCreate,
			},
			want: want{
				method: http.MethodPost,
			},
		},
		"ShouldReturnGetMethod": {
			args: args{
				action: v1alpha2.ActionObserve,
			},
			want: want{
				method: http.MethodGet,
			},
		},
		"ShouldReturnPutMethod": {
			args: args{
				action: v1alpha2.ActionUpdate,
			},
			want: want{
				method: http.MethodPut,
			},
		},
		"ShouldReturnDeleteMethod": {
			args: args{
				action: v1alpha2.ActionRemove,
			},
			want: want{
				method: http.MethodDelete,
			},
		},
		"ShouldReturnGetMethodByDefault": {
			args: args{
				action: "UNKNOWN",
			},
			want: want{
				method: http.MethodGet,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getDefaultMethodByAction(tc.args.action)
			if diff := cmp.Diff(tc.want.method, got); diff != "" {
				t.Fatalf("getDefaultMethodByAction(...): -want result, +got result: %s", diff)
			}
		})
	}
}
