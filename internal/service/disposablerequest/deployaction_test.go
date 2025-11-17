package disposablerequest

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/disposablerequest/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testDisposableName = "test-disposable"
	testNamespace      = "testns"
	testURL            = "https://api.example.com/users"
	testBody           = `{"username": "john_doe"}`
)

type MockSendRequestFn func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error)

type MockHttpClient struct {
	MockSendRequest MockSendRequestFn
}

func (c *MockHttpClient) SendRequest(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
	return c.MockSendRequest(ctx, method, url, body, headers, tlsConfigData)
}

func disposableRequest(modifiers ...func(*v1alpha2.DisposableRequest)) *v1alpha2.DisposableRequest {
	dr := &v1alpha2.DisposableRequest{
		ObjectMeta: v1.ObjectMeta{
			Name:      testDisposableName,
			Namespace: testNamespace,
		},
		Spec: v1alpha2.DisposableRequestSpec{
			ForProvider: v1alpha2.DisposableRequestParameters{
				URL:    testURL,
				Method: "POST",
				Body:   testBody,
			},
		},
		Status: v1alpha2.DisposableRequestStatus{},
	}

	for _, m := range modifiers {
		m(dr)
	}

	return dr
}

func TestDeployAction(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx        context.Context
		dr         *v1alpha2.DisposableRequest
		localKube  client.Client
		httpClient httpClient.Client
	}

	type want struct {
		err    error
		synced bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AlreadySynced": {
			reason: "Should skip deployment when resource is already synced",
			args: args{
				ctx: context.Background(),
				dr: disposableRequest(func(dr *v1alpha2.DisposableRequest) {
					dr.Status.Synced = true
				}),
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{},
			},
			want: want{
				err:    nil,
				synced: true,
			},
		},
		"RetriesLimitReached": {
			reason: "Should not retry when retries limit is reached",
			args: args{
				ctx: context.Background(),
				dr: disposableRequest(func(dr *v1alpha2.DisposableRequest) {
					limit := int32(3)
					dr.Spec.ForProvider.RollbackRetriesLimit = &limit
					dr.Status.Failed = 3
				}),
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{},
			},
			want: want{
				err: nil,
			},
		},
		"HttpRequestError": {
			reason: "Should handle HTTP request error and update status",
			args: args{
				ctx: context.Background(),
				dr:  disposableRequest(),
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							dr.Name = testDisposableName
							dr.Namespace = testNamespace
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errBoom
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
		"HttpErrorStatusCode": {
			reason: "Should handle HTTP error status codes (4xx, 5xx) and still succeed",
			args: args{
				ctx: context.Background(),
				dr:  disposableRequest(),
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							dr.Name = testDisposableName
							dr.Namespace = testNamespace
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 500,
								Body:       `{"error": "internal server error"}`,
								Headers:    map[string][]string{"Content-Type": {"application/json"}},
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
				err: errors.New("HTTP POST request failed with status code: 500"),
			},
		},
		"ResponseValidationFailed": {
			reason: "Should handle response validation failure",
			args: args{
				ctx: context.Background(),
				dr: disposableRequest(func(dr *v1alpha2.DisposableRequest) {
					dr.Spec.ForProvider.ExpectedResponse = ".body.status == \"success\""
				}),
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							dr.Name = testDisposableName
							dr.Namespace = testNamespace
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       `{"status": "failed"}`,
								Headers:    map[string][]string{"Content-Type": {"application/json"}},
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
				err: nil,
			},
		},
		"SuccessfulDeployment": {
			reason: "Should successfully deploy and update status as synced",
			args: args{
				ctx: context.Background(),
				dr: disposableRequest(func(dr *v1alpha2.DisposableRequest) {
					dr.Spec.ForProvider.ExpectedResponse = ".body.status == \"success\""
				}),
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							dr.Name = testDisposableName
							dr.Namespace = testNamespace
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							if !dr.Status.Synced {
								return errors.New("expected status to be synced")
							}
							if dr.Status.Response.StatusCode != 200 {
								return errors.New("expected status code to be 200")
							}
						}
						return nil
					}),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       `{"status": "success"}`,
								Headers:    map[string][]string{"Content-Type": {"application/json"}},
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
				err:    nil,
				synced: true,
			},
		},
		"NoExpectedResponseValidation": {
			reason: "Should succeed when no expected response is defined",
			args: args{
				ctx: context.Background(),
				dr:  disposableRequest(),
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							dr.Name = testDisposableName
							dr.Namespace = testNamespace
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 201,
								Body:       `{"id": "123"}`,
								Headers:    map[string][]string{"Content-Type": {"application/json"}},
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
				err:    nil,
				synced: true,
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
				nil,
			)
			crCtx := service.NewDisposableRequestCRContext(
				tc.args.dr,
			)
			err := DeployAction(
				svcCtx,
				crCtx,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDeployAction(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSendHttpRequest(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx        context.Context
		spec       *v1alpha2.DisposableRequestParameters
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
		"SuccessfulRequest": {
			reason: "Should successfully send HTTP request",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					URL:    testURL,
					Method: "POST",
					Body:   testBody,
				},
				localKube: &test.MockClient{},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       `{"id": "123"}`,
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
		"RequestError": {
			reason: "Should return error when HTTP request fails",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					URL:    testURL,
					Method: "POST",
					Body:   testBody,
				},
				localKube: &test.MockClient{},
				httpClient: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, tlsConfigData *httpClient.TLSConfigData) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errBoom
					},
				},
			},
			want: want{
				err: errBoom,
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
			nil,
		)
			details, err := sendHttpRequest(
				svcCtx,
				tc.args.spec,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nsendHttpRequest(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if tc.want.err == nil && details.HttpResponse.StatusCode != tc.want.statusCode {
				t.Errorf("\n%s\nsendHttpRequest(...): wanted status code %d, got %d", tc.reason, tc.want.statusCode, details.HttpResponse.StatusCode)
			}
		})
	}
}

func TestPrepareRequestResource(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx       context.Context
		obj       client.Object
		details   httpClient.HttpDetails
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
		"SuccessfulPreparation": {
			reason: "Should successfully prepare request resource",
			args: args{
				ctx: context.Background(),
				obj: disposableRequest(),
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						StatusCode: 200,
						Body:       `{"id": "123"}`,
					},
				},
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if dr, ok := obj.(*v1alpha2.DisposableRequest); ok {
							dr.Name = testDisposableName
							dr.Namespace = testNamespace
						}
						return nil
					}),
				},
			},
			want: want{
				err: nil,
			},
		},
		"GetResourceError": {
			reason: "Should return error when failing to get latest resource version",
			args: args{
				ctx: context.Background(),
				obj: disposableRequest(),
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						StatusCode: 200,
					},
				},
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, "failed to get the latest version of the resource"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dr := tc.args.obj.(*v1alpha2.DisposableRequest)
			crCtx := service.NewDisposableRequestCRContext(dr)
			svcCtx := service.NewServiceContext(
				tc.args.ctx,
				tc.args.localKube,
				logging.NewNopLogger(),
				nil,
			nil,
		)
			_, err := prepareRequestResource(
				svcCtx,
				crCtx,
				tc.args.details,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nprepareRequestResource(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestHandleHttpResponse(t *testing.T) {
	type args struct {
		ctx               context.Context
		spec              *v1alpha2.DisposableRequestParameters
		rollbackPolicy    *v1alpha2.DisposableRequestParameters
		sensitiveResponse httpClient.HttpResponse
		localKube         client.Client
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{

		"ValidationSuccess": {
			reason: "Should succeed when response validation passes",
			args: args{
				ctx: context.Background(),
				spec: &v1alpha2.DisposableRequestParameters{
					URL:              testURL,
					Method:           "POST",
					ExpectedResponse: ".body.status == \"success\"",
				},
				rollbackPolicy: &v1alpha2.DisposableRequestParameters{},
				sensitiveResponse: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"status": "success"}`,
				},
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
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
			dr := disposableRequest()
			svcCtx := service.NewServiceContext(
				tc.args.ctx,
				tc.args.localKube,
				logging.NewNopLogger(),
				nil, // httpClient not needed for handleHttpResponse
			nil,
		)
			crCtx := service.NewDisposableRequestCRContext(
				dr,
			)
			resource := &utils.RequestResource{
				StatusWriter:   crCtx.StatusWriter(),
				Resource:       dr,
				RequestContext: tc.args.ctx,
				HttpResponse:   tc.args.sensitiveResponse,
				LocalClient:    tc.args.localKube,
			}

			err := handleHttpResponse(
				svcCtx,
				crCtx,
				tc.args.sensitiveResponse,
				resource,
			)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nhandleHttpResponse(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
