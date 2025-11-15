package request

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testURL    = "https://api.example.com/users"
	testMethod = "POST"
	testBody   = `{"username": "john_doe", "email": "john@example.com"}`
	testRespID = `{"id": "123", "username": "john_doe"}`
)

var (
	testHeaders = map[string][]string{
		"Content-Type": {"application/json"},
	}
)

func TestDeployAction(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx        context.Context
		cr         *v1alpha2.Request
		action     string
		localKube  client.Client
		httpClient httpClient.Client
	}

	type want struct {
		err        error
		statusCode int
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulPOSTAction": {
			reason: "Should successfully execute POST action with JQ expressions",
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-request",
						Namespace: "testns",
					},
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								Body:    testBody,
								BaseUrl: testURL,
							},
							Mappings: []v1alpha2.Mapping{
								{
									Method: "POST",
									Body:   ".payload.body",
									URL:    ".payload.baseUrl",
								},
							},
						},
					},
					Status: v1alpha2.RequestStatus{},
				},
				action: "CREATE",
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 201,
								Body:       testRespID,
								Headers:    testHeaders,
							},
							HttpRequest: httpClient.HttpRequest{
								Method: "POST",
								URL:    testURL,
								Body:   testBody,
							},
						}, nil
					},
				},
			},
			want: want{
				err:        nil,
				statusCode: 201,
			},
		},
		"SuccessfulGETAction": {
			reason: "Should successfully execute GET action",
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-request",
						Namespace: "testns",
					},
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								BaseUrl: testURL + "/123",
							},
							Mappings: []v1alpha2.Mapping{
								{
									Method: "GET",
									URL:    ".payload.baseUrl",
								},
							},
						},
					},
					Status: v1alpha2.RequestStatus{},
				},
				action: "OBSERVE",
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       testRespID,
								Headers:    testHeaders,
							},
							HttpRequest: httpClient.HttpRequest{
								Method: "GET",
								URL:    testURL + "/123",
							},
						}, nil
					},
				},
			},
			want: want{
				err:        nil,
				statusCode: 200,
			},
		},
		"SuccessfulPUTAction": {
			reason: "Should successfully execute PUT action",
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-request",
						Namespace: "testns",
					},
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								Body:    `{"username": "john_updated"}`,
								BaseUrl: testURL + "/123",
							},
							Mappings: []v1alpha2.Mapping{
								{
									Method: "PUT",
									Body:   ".payload.body",
									URL:    ".payload.baseUrl",
								},
							},
						},
					},
					Status: v1alpha2.RequestStatus{},
				},
				action: "UPDATE",
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       `{"id": "123", "username": "john_updated"}`,
								Headers:    testHeaders,
							},
							HttpRequest: httpClient.HttpRequest{
								Method: "PUT",
								URL:    testURL + "/123",
								Body:   `{"username": "john_updated"}`,
							},
						}, nil
					},
				},
			},
			want: want{
				err:        nil,
				statusCode: 200,
			},
		},
		"SuccessfulDELETEAction": {
			reason: "Should successfully execute DELETE action",
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-request",
						Namespace: "testns",
					},
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								BaseUrl: testURL + "/123",
							},
							Mappings: []v1alpha2.Mapping{
								{
									Method: "DELETE",
									URL:    ".payload.baseUrl",
								},
							},
						},
					},
					Status: v1alpha2.RequestStatus{},
				},
				action: "REMOVE",
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 204,
								Headers:    testHeaders,
							},
							HttpRequest: httpClient.HttpRequest{
								Method: "DELETE",
								URL:    testURL + "/123",
							},
						}, nil
					},
				},
			},
			want: want{
				err:        nil,
				statusCode: 204,
			},
		},
		"HttpRequestError": {
			reason: "Should handle HTTP request errors",
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-request",
						Namespace: "testns",
					},
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								Body:    testBody,
								BaseUrl: testURL,
							},
							Mappings: []v1alpha2.Mapping{
								{
									Method: "POST",
									Body:   ".payload.body",
									URL:    ".payload.baseUrl",
								},
							},
						},
					},
					Status: v1alpha2.RequestStatus{},
				},
				action: "CREATE",
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errBoom
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
		"MappingNotFound": {
			reason: "Should return nil when mapping not found for action",
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test-request",
						Namespace: "testns",
					},
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Mappings: []v1alpha2.Mapping{
								{
									Method: "GET",
									URL:    testURL,
								},
							},
						},
					},
					Status: v1alpha2.RequestStatus{},
				},
				action: "CREATE", // Mapping only has GET, not POST for CREATE
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
			},
			want: want{
				err: nil, // DeployAction returns nil when mapping not found (logged but not error)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svcCtx := service.NewServiceContext(
				tc.args.ctx,
				tc.args.localKube,
				logging.NewNopLogger(),
				tc.args.httpClient,
			)
			crCtx := service.NewRequestCRContext(tc.args.cr)
			err := DeployAction(
				svcCtx,
				crCtx,
				tc.args.action,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDeployAction(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Only check status code if we expect success
			if tc.want.err == nil && tc.want.statusCode != 0 {
				if tc.args.cr.Status.Response.StatusCode != tc.want.statusCode {
					t.Errorf("\n%s\nExpected status code %d, got %d", tc.reason, tc.want.statusCode, tc.args.cr.Status.Response.StatusCode)
				}
			}
		})
	}
}
