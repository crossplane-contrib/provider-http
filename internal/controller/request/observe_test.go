package request

import (
	"context"
	"net/http"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
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
		mg        *v1alpha2.Request
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
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.Body = ""
				}),
			},
			want: want{
				err: errNotFound,
			},
		},
		"ObjectNotFoundPostFailed": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.RequestDetails.Method = http.MethodPost
					r.Status.Response.StatusCode = 400
				}),
			},
			want: want{
				err: errNotFound,
			},
		},
		"ObjectNotFound404StatusCode": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.StatusCode = 404
				}),
			},
			want: want{
				err: errNotFound,
			},
		},
		"FailBodyNotJSON": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								Body: "not a JSON",
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.Body = `{"username":"john_doe_new_username"}`
				}),
			},
			want: want{
				err: errors.Errorf(errNotValidJSON, "response body", "not a JSON"),
			},
		},
		"SuccessNotSynced": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								Body:       `{"username":"old_name"}`,
								StatusCode: 200,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.Body = `{"username":"john_doe_new_username"}`
				}),
			},
			want: want{
				err: nil,
				result: ObserveRequestDetails{
					Details: httpClient.HttpDetails{
						HttpResponse: httpClient.HttpResponse{
							Body:       `{"username":"old_name"}`,
							Headers:    nil,
							StatusCode: 200,
						},
					},
					ResponseError: nil,
					Synced:        false,
				},
			},
		},
		"SuccessJSONBody": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								Body:       `{"username":"john_doe_new_username"}`,
								StatusCode: 200,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.Body = `{"username":"john_doe_new_username"}`
					r.Status.Response.StatusCode = 200
				}),
			},
			want: want{
				err: nil,
				result: ObserveRequestDetails{
					Details: httpClient.HttpDetails{
						HttpResponse: httpClient.HttpResponse{
							Body:       `{"username":"john_doe_new_username"}`,
							Headers:    nil,
							StatusCode: 200,
						},
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
