package request

import (
	"context"
	"net/http"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/controller/request/requestgen"
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
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data) (resp httpClient.HttpDetails, err error) {
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
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data) (resp httpClient.HttpDetails, err error) {
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
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data) (resp httpClient.HttpDetails, err error) {
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
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data) (resp httpClient.HttpDetails, err error) {
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
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data) (resp httpClient.HttpDetails, err error) {
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
		"SuccessNoPUTMapping": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data) (resp httpClient.HttpDetails, err error) {
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
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data) (resp httpClient.HttpDetails, err error) {
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
			e := &external{
				localKube: nil,
				logger:    logging.NewNopLogger(),
				http:      nil,
			}

			got, gotErr := e.determineResponseCheck(tc.args.ctx, tc.args.cr, tc.args.details, tc.args.responseErr)
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
		"ValidResponseBody": {
			args: args{
				cr: &v1alpha2.Request{
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body: "some response",
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
		"EmptyResponseBody": {
			args: args{
				cr: &v1alpha2.Request{
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							Body: "",
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
			e := &external{}

			got := e.isObjectValidForObservation(tc.args.cr)

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
		method string
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
				method: "GET",
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
				method: "POST",
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
				method: "UNKNOWN_METHOD",
			},
			want: want{
				result: requestgen.RequestDetails{},
				err:    errors.Errorf(errMappingNotFound, "UNKNOWN_METHOD"),
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			e := &external{}

			got, gotErr := e.requestDetails(tc.args.ctx, tc.args.cr, tc.args.method)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("requestDetails(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Fatalf("requestDetails(...): -want result, +got result: %s", diff)
			}
		})
	}
}
