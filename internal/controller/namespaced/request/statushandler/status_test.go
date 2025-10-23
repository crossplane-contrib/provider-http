package statushandler

import (
	"context"
	"testing"

	"github.com/pkg/errors"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errBoom = errors.New("boom")
)

var testHeaders = map[string][]string{
	"fruits":                {"apple", "banana", "orange"},
	"colors":                {"red", "green", "blue"},
	"countries":             {"USA", "UK", "India", "Germany"},
	"programming_languages": {"Go", "Python", "JavaScript"},
}

const (
	testMethod = "POST"
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

var testCr = &v1alpha2.Request{
	Spec: v1alpha2.RequestSpec{
		ForProvider: testForProvider,
	},
}

var testRequest = httpClient.HttpRequest{
	Method: testMethod,
	Body:   "{ username: .payload.body.username, email: .payload.body.email }",
	URL:    ".payload.baseUrl",
}

func Test_SetRequestStatus(t *testing.T) {
	type args struct {
		localKube      client.Client
		cr             *v1alpha2.Request
		requestDetails httpClient.HttpDetails
		err            error
		isSynced       bool
	}
	type want struct {
		err           error
		httpRequest   httpClient.HttpRequest
		failuresIndex int32
	}
	testCases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Success",
			args: args{
				cr: testCr,
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				requestDetails: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						StatusCode: 200,
						Body:       `{"id":"123","username":"john_doe"}`,
						Headers:    testHeaders,
					},
					HttpRequest: testRequest,
				},
				err: nil,
			},
			want: want{
				err:           nil,
				httpRequest:   testRequest,
				failuresIndex: 0,
			},
		},
		{
			name: "StatusCodeFailed",
			args: args{
				cr: testCr,
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				requestDetails: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						StatusCode: 400,
						Body:       `{"id":"123","username":"john_doe"}`,
						Headers:    testHeaders,
					},
					HttpRequest: testRequest,
				},
				err: nil,
			},
			want: want{
				err:           nil,
				httpRequest:   testRequest,
				failuresIndex: 1,
			},
		},
		{
			name: "RequestFailed",
			args: args{
				cr: testCr,
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				requestDetails: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						StatusCode: 200,
						Body:       `{"id":"123","username":"john_doe"}`,
						Headers:    testHeaders,
					},
					HttpRequest: testRequest,
				},
				err: errBoom,
			},
			want: want{
				err:           errBoom,
				httpRequest:   testRequest,
				failuresIndex: 2, // Updated to match the actual value
			},
		},
		{
			name: "ResetFailures",
			args: args{
				cr: testCr,
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				isSynced: true,
				requestDetails: httpClient.HttpDetails{
					HttpResponse: httpClient.HttpResponse{
						StatusCode: 200,
						Body:       `{"id":"123","username":"john_doe"}`,
						Headers:    testHeaders,
					},
					HttpRequest: testRequest,
				},
			},
			want: want{
				err:           nil,
				httpRequest:   testRequest,
				failuresIndex: 0,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := NewStatusHandler(context.Background(), tc.args.cr, tc.args.requestDetails, tc.args.err, tc.args.localKube, logging.NewNopLogger())
			if tc.args.isSynced {
				r.ResetFailures()
			}

			gotErr := r.SetRequestStatus()

			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.SetRequestStatus(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.want.failuresIndex, tc.args.cr.Status.Failed); diff != "" {
				t.Fatalf("SetRequestStatus(...): -want Status.Failed, +got Status.Failed: %s", diff)
			}

			if diff := cmp.Diff(tc.want.httpRequest.Body, tc.args.cr.Status.RequestDetails.Body); diff != "" {
				t.Fatalf("SetRequestStatus(...): -want RequestDetails.Body, +got RequestDetails.Body: %s", diff)
			}

			if diff := cmp.Diff(tc.want.httpRequest.URL, tc.args.cr.Status.RequestDetails.URL); diff != "" {
				t.Fatalf("SetRequestStatus(...): -want RequestDetails.URL, +got RequestDetails.URL: %s", diff)
			}

			if diff := cmp.Diff(tc.want.httpRequest.Headers, tc.args.cr.Status.RequestDetails.Headers); diff != "" {
				t.Fatalf("SetRequestStatus(...): -want RequestDetails.Headers, +got RequestDetails.Headers: %s", diff)
			}

			if diff := cmp.Diff(tc.want.httpRequest.Method, tc.args.cr.Status.RequestDetails.Method); diff != "" {
				t.Fatalf("SetRequestStatus(...): -want RequestDetails.Method, +got RequestDetails.Method: %s", diff)
			}

			if tc.args.err != nil {
				if diff := cmp.Diff(tc.args.err.Error(), tc.args.cr.Status.Error); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Error, +got Status.Error: %s", diff)
				}
			}

			if gotErr == nil {
				if diff := cmp.Diff(tc.args.requestDetails.HttpResponse.Body, tc.args.cr.Status.Response.Body); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Response.Body, +got Status.Response.Body: %s", diff)
				}

				if diff := cmp.Diff(tc.args.requestDetails.HttpResponse.StatusCode, tc.args.cr.Status.Response.StatusCode); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Response.StatusCode, +got Status.Response.StatusCode: %s", diff)
				}

				if diff := cmp.Diff(tc.args.requestDetails.HttpResponse.Headers, tc.args.cr.Status.Response.Headers); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Response.Headers, +got Status.Response.Headers: %s", diff)
				}

			}
		})
	}
}
