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
)

func Test_CustomCheck(t *testing.T) {
	type args struct {
		ctx     context.Context
		cr      *v1alpha2.Request
		details httpClient.HttpDetails
		logic   string
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
				logic: `.response.body.password == .payload.body.password`,
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
				logic: `.response.body.password == .payload.body.password`,
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
			e := &customCheck{}
			svcCtx := service.NewServiceContext(tc.args.ctx, nil, logging.NewNopLogger(, nil), nil)
			got, gotErr := e.check(svcCtx, &tc.args.cr.Spec.ForProvider, tc.args.details, tc.args.logic)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("Check(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("Check(...): -want result, +got result: %s", diff)
			}
		})
	}
}
