package observe

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

var (
	testPostMapping = v1alpha2.Mapping{
		Method: "POST",
		Body:   "{ username: .payload.body.username, email: .payload.body.email }",
		URL:    ".payload.baseUrl",
	}

	testPutMapping = v1alpha2.Mapping{
		Method: "PUT",
		Body:   "{ username: \"john_doe_new_username\" }",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
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

func Test_DefaultIsUpToDateCheck(t *testing.T) {
	type args struct {
		ctx         context.Context
		cr          *v1alpha2.Request
		details     httpClient.HttpDetails
		responseErr error
	}

	type want struct {
		result bool
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ValidJSONSyncedState": {
			args: args{
				ctx: context.Background(),
				cr:  &v1alpha2.Request{},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       ``,
						Headers:    nil,
						StatusCode: 0,
					},
				},
				responseErr: nil,
			},
			want: want{
				result: true,
				err:    nil,
			},
		},
		"UnsyncedStateWithValidJSON": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
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
								Type: common.ExpectedResponseCheckTypeDefault,
							},
						},
					},
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body:       `{"id": "123"}`,
							StatusCode: 200,
						},
					},
				}, details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       `{"username": "john_doe"}`,
						Headers:    nil,
						StatusCode: 0,
					},
				},
				responseErr: nil,
			},
			want: want{
				result: false,
				err:    nil,
			},
		},
		"InvalidResponseJSON": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
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
								Type: common.ExpectedResponseCheckTypeDefault,
							},
						},
					},
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body:       `{"id": "123"}`,
							StatusCode: 200,
						},
					},
				},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       `{`,
						Headers:    nil,
						StatusCode: 200,
					},
				},
				responseErr: nil,
			},
			want: want{
				result: false,
				err:    errors.New("response body is not a valid JSON string: {"),
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			e := &defaultIsUpToDateResponseCheck{}
			svcCtx := service.NewServiceContext(tc.args.ctx, nil, logging.NewNopLogger(, nil), nil)
			crCtx := service.NewRequestCRContext(tc.args.cr)
			got, gotErr := e.Check(svcCtx, crCtx, tc.args.details, tc.args.responseErr)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("Check(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_CustomIsUpToDateCheck(t *testing.T) {
	type args struct {
		ctx         context.Context
		cr          *v1alpha2.Request
		details     httpClient.HttpDetails
		responseErr error
	}

	type want struct {
		result bool
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"CustomCheckPasses": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								Body: `{"password": "password"}`,
							},
							ExpectedResponseCheck: v1alpha2.ExpectedResponseCheck{
								Type:  common.ExpectedResponseCheckTypeCustom,
								Logic: `.response.body.password == .payload.body.password`,
							},
						},
					},
				},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       `{"password":"password"}`,
						Headers:    nil,
						StatusCode: 0,
					},
				},
				responseErr: nil,
			},
			want: want{
				result: true,
				err:    nil,
			},
		},
		"CustomCheckFails": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								Body: `{"password": "password"}`,
							},
							ExpectedResponseCheck: v1alpha2.ExpectedResponseCheck{
								Type:  common.ExpectedResponseCheckTypeCustom,
								Logic: `.response.body.password == .payload.body.password`,
							},
						},
					},
				},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       `{"password":"wrong_password"}`,
						Headers:    nil,
						StatusCode: 0,
					},
				},
				responseErr: nil,
			},
			want: want{
				result: false,
				err:    nil,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			e := &customIsUpToDateResponseCheck{}
			svcCtx := service.NewServiceContext(tc.args.ctx, nil, logging.NewNopLogger(, nil), nil)
			crCtx := service.NewRequestCRContext(tc.args.cr)
			got, gotErr := e.Check(svcCtx, crCtx, tc.args.details, tc.args.responseErr)

			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("Check(...): -want result, +got result: %s", diff)
			}
		})
	}
}
