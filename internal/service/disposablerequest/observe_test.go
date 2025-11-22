package disposablerequest

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestValidateStoredResponse(t *testing.T) {
	type args struct {
		ctx       context.Context
		spec      *v1alpha2.DisposableRequestParameters
		dr        *v1alpha2.DisposableRequest
		localKube client.Client
	}

	type want struct {
		valid      bool
		err        error
		statusCode int
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoStoredResponse": {
			reason: "Should return false when no response is stored",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".body.status == \"success\"",
				},
				dr: &v1alpha2.DisposableRequest{
					Status: v1alpha2.DisposableRequestStatus{
						Response: v1alpha2.Response{
							StatusCode: 0,
						},
					},
				},
				localKube: &test.MockClient{},
			},
			want: want{
				valid: false,
				err:   nil,
			},
		},
		"ValidStoredResponse": {
			reason: "Should return true when stored response matches expected criteria",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".body.status == \"success\"",
				},
				dr: &v1alpha2.DisposableRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
					Spec: v1alpha2.DisposableRequestSpec{
						ForProvider: v1alpha2.DisposableRequestParameters{
							ExpectedResponse: ".body.status == \"success\"",
						},
					},
					Status: v1alpha2.DisposableRequestStatus{
						Response: v1alpha2.Response{
							StatusCode: 200,
							Body:       `{"status": "success"}`,
							Headers:    map[string][]string{"Content-Type": {"application/json"}},
						},
					},
				},
				localKube: &test.MockClient{},
			},
			want: want{
				valid:      true,
				err:        nil,
				statusCode: 200,
			},
		},
		"InvalidStoredResponse": {
			reason: "Should return false when stored response doesn't match expected criteria",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					ExpectedResponse: ".body.status == \"success\"",
				},
				dr: &v1alpha2.DisposableRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
					Spec: v1alpha2.DisposableRequestSpec{
						ForProvider: v1alpha2.DisposableRequestParameters{
							ExpectedResponse: ".body.status == \"success\"",
						},
					},
					Status: v1alpha2.DisposableRequestStatus{
						Response: v1alpha2.Response{
							StatusCode: 200,
							Body:       `{"status": "failed"}`,
						},
					},
				},
				localKube: &test.MockClient{},
			},
			want: want{
				valid: false,
				err:   nil,
			},
		},
		"NoExpectedResponseValidation": {
			reason: "Should return true when no expected response is defined",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					URL:    "https://api.example.com/test",
					Method: "GET",
				},
				dr: &v1alpha2.DisposableRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
					Status: v1alpha2.DisposableRequestStatus{
						Response: v1alpha2.Response{
							StatusCode: 200,
							Body:       `{"data": "anything"}`,
						},
					},
				},
				localKube: &test.MockClient{},
			},
			want: want{
				valid:      true,
				err:        nil,
				statusCode: 200,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svcCtx := service.NewServiceContext(
				tc.args.ctx,
				tc.args.localKube,
				logging.NewNopLogger(),
				nil,
				nil,
			)
			crCtx := service.NewDisposableRequestCRContext(
				tc.args.dr,
			)
			valid, response, err := ValidateStoredResponse(
				svcCtx,
				crCtx,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateStoredResponse(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if valid != tc.want.valid {
				t.Errorf("\n%s\nValidateStoredResponse(...): wanted valid=%v, got %v", tc.reason, tc.want.valid, valid)
			}

			if tc.want.valid && response.StatusCode != tc.want.statusCode {
				t.Errorf("\n%s\nValidateStoredResponse(...): wanted status code %d, got %d", tc.reason, tc.want.statusCode, response.StatusCode)
			}
		})
	}
}

func TestCalculateUpToDateStatus(t *testing.T) {
	type args struct {
		reconciliationPolicy *v1alpha2.DisposableRequestParameters
		rollbackPolicy       *v1alpha2.DisposableRequestParameters
		currentStatus        bool
	}

	type want struct {
		upToDate bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"LoopInfinitelyWithoutRollbackLimit": {
			reason: "Should never be up-to-date when looping infinitely without rollback limit",
			args: args{
				reconciliationPolicy: &v1alpha2.DisposableRequestParameters{
					ShouldLoopInfinitely: true,
				},
				rollbackPolicy: &v1alpha2.DisposableRequestParameters{
					RollbackRetriesLimit: nil,
				},
				currentStatus: true,
			},
			want: want{
				upToDate: false,
			},
		},
		"LoopInfinitelyWithRollbackLimit": {
			reason: "Should respect current status when looping infinitely with rollback limit",
			args: args{
				reconciliationPolicy: &v1alpha2.DisposableRequestParameters{
					ShouldLoopInfinitely: true,
				},
				rollbackPolicy: &v1alpha2.DisposableRequestParameters{
					RollbackRetriesLimit: func() *int32 { v := int32(3); return &v }(),
				},
				currentStatus: true,
			},
			want: want{
				upToDate: true,
			},
		},
		"NormalReconciliation": {
			reason: "Should respect current status when not looping infinitely",
			args: args{
				reconciliationPolicy: &v1alpha2.DisposableRequestParameters{
					ShouldLoopInfinitely: false,
				},
				rollbackPolicy: &v1alpha2.DisposableRequestParameters{},
				currentStatus:  true,
			},
			want: want{
				upToDate: true,
			},
		},
		"NormalReconciliationNotUpToDate": {
			reason: "Should return false when current status is false",
			args: args{
				reconciliationPolicy: &v1alpha2.DisposableRequestParameters{
					ShouldLoopInfinitely: false,
				},
				rollbackPolicy: &v1alpha2.DisposableRequestParameters{},
				currentStatus:  false,
			},
			want: want{
				upToDate: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Merge reconciliationPolicy and rollbackPolicy into a single ForProvider
			forProvider := *tc.args.reconciliationPolicy
			if tc.args.rollbackPolicy != nil {
				forProvider.RollbackRetriesLimit = tc.args.rollbackPolicy.RollbackRetriesLimit
			}
			dr := &v1alpha2.DisposableRequest{
				Spec: v1alpha2.DisposableRequestSpec{
					ForProvider: forProvider,
				},
			}
			crCtx := service.NewDisposableRequestCRContext(dr)
			got := CalculateUpToDateStatus(
				crCtx,
				tc.args.currentStatus,
			)

			if got != tc.want.upToDate {
				t.Errorf("\n%s\nCalculateUpToDateStatus(...): wanted %v, got %v", tc.reason, tc.want.upToDate, got)
			}
		})
	}
}

func TestUpdateResourceStatus(t *testing.T) {
	type args struct {
		ctx       context.Context
		dr        *v1alpha2.DisposableRequest
		localKube client.Client
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulUpdate": {
			reason: "Should successfully update resource status to Available",
			args: args{
				ctx: context.Background(),
				dr: &v1alpha2.DisposableRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
				},
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							dr.Name = "test"
							dr.Namespace = "default"
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := UpdateResourceStatus(
				tc.args.ctx,
				tc.args.dr,
				tc.args.localKube,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpdateResourceStatus(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestApplySecretInjectionsFromStoredResponse(t *testing.T) {
	type args struct {
		ctx            context.Context
		spec           *v1alpha2.DisposableRequestParameters
		storedResponse httpClient.HttpResponse
		dr             *v1alpha2.DisposableRequest
		localKube      client.Client
	}

	cases := map[string]struct {
		reason string
		args   args
	}{
		"ApplySecretsSuccessfully": {
			reason: "Should apply secret injections from stored response",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					URL:    "https://api.example.com/test",
					Method: "GET",
				},
				storedResponse: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"token": "secret-value"}`,
					Headers:    map[string][]string{"Content-Type": {"application/json"}},
				},
				dr: &v1alpha2.DisposableRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
				},
				localKube: &test.MockClient{},
			},
		},
		"EmptyResponse": {
			reason: "Should handle empty stored response",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					URL:    "https://api.example.com/test",
					Method: "DELETE",
				},
				storedResponse: httpClient.HttpResponse{
					StatusCode: 204,
					Body:       "",
				},
				dr: &v1alpha2.DisposableRequest{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
				},
				localKube: &test.MockClient{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svcCtx := service.NewServiceContext(
				tc.args.ctx,
				tc.args.localKube,
				logging.NewNopLogger(),
				nil,
				nil,
			)
			crCtx := service.NewDisposableRequestCRContext(
				tc.args.dr,
			)
			// This function doesn't return anything, so we just verify it doesn't panic
			ApplySecretInjectionsFromStoredResponse(
				svcCtx,
				crCtx,
				tc.args.storedResponse,
			)
		})
	}
}
