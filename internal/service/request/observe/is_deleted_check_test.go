package observe

import (
	"context"
	"net/http"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

func Test_DefaultIsRemovedCheck(t *testing.T) {
	type args struct {
		ctx         context.Context
		cr          *v1alpha2.Request
		details     httpClient.HttpDetails
		responseErr error
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ValidRemovedState": {
			args: args{
				ctx: context.Background(),
				cr:  &v1alpha2.Request{},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       ``,
						Headers:    nil,
						StatusCode: http.StatusNotFound,
					},
				},
				responseErr: nil,
			},
			want: want{
				err: errors.New(ErrObjectNotFound),
			},
		},
		"RemovedStateWithValidJSON": {
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
				}, details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       `{"username": "john_doe"}`,
						Headers:    nil,
						StatusCode: http.StatusNotFound,
					},
				},
				responseErr: nil,
			},
			want: want{
				err: errors.New(ErrObjectNotFound),
			},
		},
		"ValidNotRemovedState": {
			args: args{
				ctx: context.Background(),
				cr:  &v1alpha2.Request{},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       ``,
						Headers:    nil,
						StatusCode: http.StatusOK,
					},
				},
				responseErr: nil,
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			e := &defaultIsRemovedResponseCheck{}
			svcCtx := service.NewServiceContext(tc.args.ctx, nil, logging.NewNopLogger(), nil)
			crCtx := service.NewRequestCRContext(tc.args.cr)
			gotErr := e.Check(svcCtx, crCtx, tc.args.details, tc.args.responseErr)

			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_CustomIsRemovedCheck(t *testing.T) {
	type args struct {
		ctx         context.Context
		cr          *v1alpha2.Request
		details     httpClient.HttpDetails
		responseErr error
	}

	type want struct {
		err error
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
							IsRemovedCheck: v1alpha2.ExpectedResponseCheck{
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
				err: errors.New(ErrObjectNotFound),
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
							IsRemovedCheck: v1alpha2.ExpectedResponseCheck{
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
				err: nil,
			},
		},
		"FailedParsing": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								Body: `{"password": "password"}`,
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
				err: errors.Errorf(errExpectedFormat, "isRemovedCheck", "failed to parse given mapping -  jq error: missing query (try \".\")"),
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			e := &customIsRemovedResponseCheck{}
			svcCtx := service.NewServiceContext(tc.args.ctx, nil, logging.NewNopLogger(), nil)
			crCtx := service.NewRequestCRContext(tc.args.cr)
			gotErr := e.Check(svcCtx, crCtx, tc.args.details, tc.args.responseErr)

			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}
		})
	}
}
