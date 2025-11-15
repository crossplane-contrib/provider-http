package request

import (
	"context"
	"net/http"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/request/observe"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestmapping"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	providerName    = "http-test"
	testRequestName = "test-request"
	testNamespace   = "testns"
)

var (
	errNotFound = errors.New(observe.ErrObjectNotFound)
)

var (
	testForProvider = v1alpha2.RequestParameters{
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
	}
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

type httpRequestModifier func(request *v1alpha2.Request)

func httpRequest(rm ...httpRequestModifier) *v1alpha2.Request {
	r := &v1alpha2.Request{
		ObjectMeta: v1.ObjectMeta{
			Name:      testRequestName,
			Namespace: testNamespace,
		},
		Spec: v1alpha2.RequestSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: providerName,
				},
			},
			ForProvider: testForProvider,
		},
		Status: v1alpha2.RequestStatus{},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

type MockSendRequestFn func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error)

type MockHttpClient struct {
	MockSendRequest MockSendRequestFn
}

func (c *MockHttpClient) SendRequest(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
	return c.MockSendRequest(ctx, method, url, body, headers, skipTLSVerify)
}

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
		"ObjectIdKnownBeforeCreate": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
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
					r.Spec.ForProvider.Mappings = []v1alpha2.Mapping{
						{
							Method: "GET",
							URL:    "(\"http://some.org/\" + \"1423\")",
						},
					}
				}),
			},
			want: want{
				err: nil,
				result: ObserveRequestDetails{
					Details: httpClient.HttpDetails{
						HttpResponse: httpClient.HttpResponse{
							Body:       `{"username":"john_doe_new_username"}`,
							StatusCode: 200,
						},
					},
					Synced: true,
				},
			},
		},
		"ObjectNotFoundEmptyStatus": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.Body = ""
					r.Status.Response.StatusCode = 0
				}),
			},
			want: want{
				err: errNotFound,
			},
		},
		"ObjectNotFoundPostFailed": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
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
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								Body:       "",
								StatusCode: http.StatusNotFound,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.StatusCode = http.StatusNotFound
				}),
			},
			want: want{
				err: errNotFound,
			},
		},
		"FailBodyNotJSON": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
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
					r.Status.Response.StatusCode = http.StatusOK
				}),
			},
			want: want{
				err: errors.Errorf(errNotValidJSON, "response body", "not a JSON"),
			},
		},
		"SuccessNotSynced": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
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
					r.Status.Response.StatusCode = http.StatusOK
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
		"SuccessNoPUTMapping": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
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
					r.Status.Response.StatusCode = 200
					r.Spec.ForProvider.Mappings = []v1alpha2.Mapping{
						testPostMapping,
						testGetMapping,
						testDeleteMapping,
					}
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
					Synced:        true,
				},
			},
		},
		"SuccessJSONBody": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
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
		"MissingMappingObjectNotCreated": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.Body = ""
					r.Status.Response.StatusCode = 0
					r.Spec.ForProvider.Mappings = []v1alpha2.Mapping{
						testPostMapping,
						testPutMapping,
						testDeleteMapping,
						// No GET or OBSERVE mapping
					}
				}),
			},
			want: want{
				err: errors.New("OBSERVE or GET mapping doesn't exist in request, skipping operation"),
			},
		},
		"MissingMappingObjectCreated": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpRequest(func(r *v1alpha2.Request) {
					r.Status.Response.Body = `{"id": "123"}`
					r.Status.Response.StatusCode = 201
					r.Status.RequestDetails.Method = http.MethodPost
					r.Spec.ForProvider.Mappings = []v1alpha2.Mapping{
						testPostMapping,
						testPutMapping,
						testDeleteMapping,
						// No GET or OBSERVE mapping
					}
				}),
			},
			want: want{
				err: errors.New("OBSERVE or GET mapping doesn't exist in request, skipping operation"),
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			svcCtx := service.NewServiceContext(context.Background(), tc.args.localKube, logging.NewNopLogger(), tc.args.http)
			crCtx := service.NewRequestCRContext(tc.args.mg)
			got, gotErr := IsUpToDate(svcCtx, crCtx)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("isUpToDate(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("isUpToDate(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_determineResponseCheck(t *testing.T) {
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
		"DefaultResponseCheckSynced": {
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
								testDeleteMapping,
								testPutMapping,
							},
							ExpectedResponseCheck: v1alpha2.ExpectedResponseCheck{
								Type: v1alpha2.ExpectedResponseCheckTypeDefault,
							},
						},
					},
				},
				details: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						Body:       `{"username": "john_doe_new_username"}`,
						Headers:    nil,
						StatusCode: 200,
					},
				},
				responseErr: nil,
			},
			want: want{
				result: ObserveRequestDetails{
					Details: httpClient.HttpDetails{
						HttpResponse: httpClient.HttpResponse{
							Body:       `{"username": "john_doe_new_username"}`,
							StatusCode: 200,
						},
					},
					Synced: true,
				},
				err: nil,
			},
		},
		"DefaultResponseCheckUnsynced": {
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
								testDeleteMapping,
								testPutMapping,
							},
							ExpectedResponseCheck: v1alpha2.ExpectedResponseCheck{
								Type: v1alpha2.ExpectedResponseCheckTypeDefault,
							},
						},
					},
				},
				details: httpClient.HttpDetails{
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
		"CustomResponseCheckFails": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							ExpectedResponseCheck: v1alpha2.ExpectedResponseCheck{
								Type:  v1alpha2.ExpectedResponseCheckTypeCustom,
								Logic: `.foo == "baz"`,
							},
						},
					},
				},
				details: httpClient.HttpDetails{
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
		"UnknownResponseCheckType": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							ExpectedResponseCheck: v1alpha2.ExpectedResponseCheck{
								Type: "UnknownType",
							},
						},
					},
				},
				details: httpClient.HttpDetails{
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
					Synced: true,
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			svcCtx := service.NewServiceContext(tc.args.ctx, nil, logging.NewNopLogger(), nil)
			crCtx := service.NewRequestCRContext(tc.args.cr)
			got, gotErr := determineIfUpToDate(svcCtx, crCtx, tc.args.details, tc.args.responseErr)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("determineResponseCheck(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("determineResponseCheck(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_isObjectValidForObservation(t *testing.T) {
	type args struct {
		cr *v1alpha2.Request
	}

	type want struct {
		valid bool
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ValidStatusCode": {
			args: args{
				cr: &v1alpha2.Request{
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body:       "",
							StatusCode: http.StatusOK,
						},
						RequestDetails: v1alpha2.Mapping{
							Method: http.MethodGet,
						},
					},
				},
			},
			want: want{
				valid: true,
			},
		},
		"EmptyStatusCode": {
			args: args{
				cr: &v1alpha2.Request{
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body:       "",
							StatusCode: 0,
						},
					},
				},
			},
			want: want{
				valid: false,
			},
		},
		"POSTMethodWithErrorResponse": {
			args: args{
				cr: &v1alpha2.Request{
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body:       "some response",
							StatusCode: http.StatusInternalServerError,
						},
						RequestDetails: v1alpha2.Mapping{
							Method: http.MethodPost,
						},
					},
				},
			},
			want: want{
				valid: false,
			},
		},
		"POSTMethodWithoutErrorResponse": {
			args: args{
				cr: &v1alpha2.Request{
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body:       "some response",
							StatusCode: http.StatusOK,
						},
						RequestDetails: v1alpha2.Mapping{
							Method: http.MethodPost,
						},
					},
				},
			},
			want: want{
				valid: true,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			crCtx := service.NewRequestCRContext(tc.args.cr)
			got := isObjectValidForObservation(crCtx)

			if diff := cmp.Diff(tc.want.valid, got); diff != "" {
				t.Errorf("isObjectValidForObservation(...): -want valid, +got valid: %s", diff)
			}
		})
	}
}

func Test_requestDetails(t *testing.T) {
	type args struct {
		ctx    context.Context
		cr     *v1alpha2.Request
		action string
	}

	type want struct {
		result requestgen.RequestDetails
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ValidMappingForGET": {
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
								testGetMapping,
							},
						},
					},
				},
				action: v1alpha2.ActionObserve,
			},
			want: want{
				result: requestgen.RequestDetails{
					Url: "https://api.example.com/users/",
					Body: httpClient.Data{
						Encrypted: "",
						Decrypted: "",
					},
					Headers: httpClient.Data{
						Encrypted: map[string][]string{},
						Decrypted: map[string][]string{},
					},
				},
				err: nil,
			},
		},
		"ValidMappingForPOST": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
								BaseUrl: "https://api.example.com/users",
							}, Mappings: []v1alpha2.Mapping{
								testPostMapping,
							},
						},
					},
				},
				action: v1alpha2.ActionCreate,
			},
			want: want{
				result: requestgen.RequestDetails{
					Url: "https://api.example.com/users",
					Body: httpClient.Data{
						Encrypted: `{"email":"john.doe@example.com","username":"john_doe"}`,
						Decrypted: `{"email":"john.doe@example.com","username":"john_doe"}`,
					},
					Headers: httpClient.Data{
						Encrypted: map[string][]string{},
						Decrypted: map[string][]string{},
					},
				},
				err: nil,
			},
		},
		"MappingNotFound": {
			args: args{
				ctx: context.Background(),
				cr: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{},
					},
				},
				action: "UNKNOWN_METHOD",
			},
			want: want{
				result: requestgen.RequestDetails{},
				err:    errors.Errorf(requestmapping.ErrMappingNotFound, "UNKNOWN_METHOD", http.MethodGet),
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			mapping, err := requestmapping.GetMapping(&tc.args.cr.Spec.ForProvider, tc.args.action, logging.NewNopLogger())
			if err != nil {
				if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
					t.Fatalf("requestDetails(...): -want error, +got error: %s", diff)
				}
				return
			}
			svcCtx := service.NewServiceContext(tc.args.ctx, nil, logging.NewNopLogger(), nil)
			crCtx := service.NewRequestCRContext(tc.args.cr)
			got, gotErr := requestgen.GenerateValidRequestDetails(svcCtx, crCtx, mapping)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("requestDetails(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("requestDetails(...): -want result, +got result: %s", diff)
			}
		})
	}
}
