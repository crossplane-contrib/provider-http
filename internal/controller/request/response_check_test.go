package request

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

func Test_DefaultResponseCheck(t *testing.T) {
	type args struct {
		ctx         context.Context
		cr          *v1alpha2.Request
		details     httpClient.HttpDetails
		responseErr error
	}

	type want struct {
		result ObserveRequestDetails
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
				result: ObserveRequestDetails{
					Synced: true,
				},
				err: nil,
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
								Type: v1alpha2.ExpectedResponseCheckTypeDefault,
							},
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
				result: ObserveRequestDetails{
					Details: httpClient.HttpDetails{
						HttpResponse: httpClient.HttpResponse{
							Body: `{"username": "john_doe"}`,
						},
					},
					Synced: false,
				},
				err: nil,
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
								Type: v1alpha2.ExpectedResponseCheckTypeDefault,
							},
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
				result: ObserveRequestDetails{
					Synced: false,
				},
				err: errors.New("response body is not a valid JSON string: {"),
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			e := &DefaultResponseCheck{
				client: &external{
					localKube: nil,
					http:      nil,
					logger:    logging.NewNopLogger(),
				},
			}
			got, gotErr := e.Check(tc.args.ctx, tc.args.cr, tc.args.details, tc.args.responseErr)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("Check(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_CustomResponseCheck(t *testing.T) {
	type args struct {
		ctx         context.Context
		cr          *v1alpha2.Request
		details     httpClient.HttpDetails
		responseErr error
	}

	type want struct {
		result ObserveRequestDetails
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
								Type:  v1alpha2.ExpectedResponseCheckTypeCustom,
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
				result: ObserveRequestDetails{
					Details: httpClient.HttpDetails{
						HttpResponse: httpClient.HttpResponse{
							Body: `{"password":"password"}`,
						},
					},
					Synced: true,
				},
				err: nil,
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
								Type:  v1alpha2.ExpectedResponseCheckTypeCustom,
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
				result: ObserveRequestDetails{
					Details: httpClient.HttpDetails{
						HttpResponse: httpClient.HttpResponse{
							Body: `{"password":"wrong_password"}`,
						},
					},
					Synced: false,
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			e := &CustomResponseCheck{
				client: &external{
					localKube: nil,
					http:      nil,
					logger:    logging.NewNopLogger(),
				},
			}
			got, gotErr := e.Check(tc.args.ctx, tc.args.cr, tc.args.details, tc.args.responseErr)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("Check(...): -want result, +got result: %s", diff)
			}
		})
	}
}
