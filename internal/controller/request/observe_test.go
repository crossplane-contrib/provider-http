package request

import (
	"context"
	"net/http"
	"testing"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"github.com/arielsepton/provider-http/internal/controller/request/statushandler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errNotFound = errors.New(errObjectNotFound)
)

func Test_isUpToDate(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        *v1alpha1.Request
		status    statushandler.RequestStatusHandler
	}
	type want struct {
		result ObserveRequestDetails
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ObjectNotFoundEmptyBody": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha1.Request) {
					r.Status.Response.Body = ""
				}),
				status: &MockStatusHandler{
					MockSetRequest: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error {
						return nil
					},
					MockResetFailures: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error {
						return nil
					},
				},
			},
			want: want{
				err: errNotFound,
			},
		},
		"ObjectNotFoundPostFailed": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha1.Request) {
					r.Status.Response.Method = http.MethodPost
					r.Status.Response.StatusCode = 400
				}),
				status: &MockStatusHandler{
					MockSetRequest: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error {
						return nil
					},
					MockResetFailures: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error {
						return nil
					},
				},
			},
			want: want{
				err: errNotFound,
			},
		},
		"ObjectNotFound404StatusCode": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha1.Request) {
					r.Status.Response.StatusCode = 404
				}),
				status: &MockStatusHandler{
					MockSetRequest: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error {
						return nil
					},
					MockResetFailures: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error {
						return nil
					},
				},
			},
			want: want{
				err: errNotFound,
			},
		},
		"FailBodyNotJSON": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{
							Body: "not a JSON",
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha1.Request) {
					r.Status.Response.Body = `{"username":"john_doe_new_username"}`
				}),
				status: &MockStatusHandler{
					MockSetRequest: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error {
						return nil
					},
					MockResetFailures: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error {
						return nil
					},
				},
			},
			want: want{
				err: errors.Errorf(errNotValidJSON, "response body", "not a JSON"),
			},
		},
		"SuccessNotSynced": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{
							Body:       `{"username":"old_name"}`,
							StatusCode: 200,
							Method:     http.MethodGet,
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha1.Request) {
					r.Status.Response.Body = `{"username":"john_doe_new_username"}`
				}),
				status: &MockStatusHandler{
					MockSetRequest: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error {
						return nil
					},
					MockResetFailures: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error {
						return nil
					},
				},
			},
			want: want{
				err: nil,
				result: ObserveRequestDetails{
					Response: httpClient.HttpResponse{
						Body:       `{"username":"old_name"}`,
						Headers:    nil,
						StatusCode: 200,
						Method:     "GET",
					},
					ResponseError: nil,
					Synced:        false,
				},
			},
		},
		"SuccessJSONBody": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{
							Body:       `{"username":"john_doe_new_username"}`,
							StatusCode: 200,
							Method:     http.MethodGet,
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha1.Request) {
					r.Status.Response.Body = `{"username":"john_doe_new_username"}`
					r.Status.Response.StatusCode = 200
				}),
				status: &MockStatusHandler{
					MockSetRequest: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse, err error) error {
						return nil
					},
					MockResetFailures: func(ctx context.Context, cr *v1alpha1.Request, res httpClient.HttpResponse) error {
						return nil
					},
				},
			},
			want: want{
				err: nil,
				result: ObserveRequestDetails{
					Response: httpClient.HttpResponse{
						Body:       `{"username":"john_doe_new_username"}`,
						Headers:    nil,
						StatusCode: 200,
						Method:     "GET",
					},
					ResponseError: nil,
					Synced:        true,
				},
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			e := &external{
				localKube: tc.args.localKube,
				logger:    logging.NewNopLogger(),
				http:      tc.args.http,
				status:    tc.args.status,
			}
			got, gotErr := e.isUpToDate(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("isUpToDate(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("isUpToDate(...): -want result, +got result: %s", diff)
			}
		})
	}
}
