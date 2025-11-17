package utils

import (
	"context"
	"testing"

	v1alpha1_disposable "github.com/crossplane-contrib/provider-http/apis/disposablerequest/v1alpha2"
	v1alpha1_request "github.com/crossplane-contrib/provider-http/apis/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
)

var (
	errBoom = errors.New("boom")
)

var (
	testPostMapping = v1alpha1_request.Mapping{
		Method: "POST",
		Body:   "{ username: .payload.body.username, email: .payload.body.email }",
		URL:    ".payload.baseUrl",
	}

	testPutMapping = v1alpha1_request.Mapping{
		Method: "PUT",
		Body:   "{ username: \"john_doe_new_username\" }",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testGetMapping = v1alpha1_request.Mapping{
		Method: "GET",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testDeleteMapping = v1alpha1_request.Mapping{
		Method: "DELETE",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}
)

var (
	testDisposableForProvider = v1alpha1_disposable.DisposableRequestParameters{
		Body:   "{\"key1\": \"value1\"}",
		URL:    "http://example",
		Method: "GET",
	}

	testDisposableCr = &v1alpha1_disposable.DisposableRequest{
		Spec: v1alpha1_disposable.DisposableRequestSpec{
			ForProvider: testDisposableForProvider,
		},
	}

	testDisposableResource = RequestResource{
		StatusWriter:   testDisposableCr,
		Resource:       testDisposableCr,
		RequestContext: context.Background(),
		HttpResponse: httpClient.HttpResponse{
			StatusCode: 200,
			Body:       `{"id":"123","username":"john_doe"}`,
		},
		LocalClient: &test.MockClient{
			MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
		},
	}
)

var (
	testRequestForProvider = v1alpha1_request.RequestParameters{
		Payload: v1alpha1_request.Payload{
			Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
			BaseUrl: "https://api.example.com/users",
		},
		Mappings: []v1alpha1_request.Mapping{
			testPostMapping,
			testGetMapping,
			testPutMapping,
			testDeleteMapping,
		},
	}

	testRequestCr = &v1alpha1_request.Request{
		Spec: v1alpha1_request.RequestSpec{
			ForProvider: testRequestForProvider,
		},
		Status: v1alpha1_request.RequestStatus{
			Failed: int32(3),
		},
	}

	testRequestResource = RequestResource{
		StatusWriter:   testRequestCr,
		Resource:       testRequestCr,
		RequestContext: context.Background(),
		HttpResponse: httpClient.HttpResponse{
			StatusCode: 200,
			Body:       `{"ids":"123","username":"john_doe"}`,
		},
		HttpRequest: httpClient.HttpRequest{
			Method: "GET",
			URL:    "https://example",
		},
		LocalClient: &test.MockClient{
			MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
		},
	}
)

func Test_SetRequestResourceStatus(t *testing.T) {
	type args struct {
		rr          RequestResource
		statusFuncs []SetRequestStatusFunc
	}
	type want struct {
		failures int32
		err      error
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				rr: testRequestResource,
				statusFuncs: []SetRequestStatusFunc{
					testRequestResource.SetBody(),
					testRequestResource.SetRequestDetails(),
					testRequestResource.SetHeaders(),
					testRequestResource.SetStatusCode(),
					testRequestResource.ResetFailures(),
					testRequestResource.SetCache(),
				},
			},
			want: want{
				failures: 0,
				err:      nil,
			},
		},
		"SetError": {
			args: args{
				rr: testRequestResource,
				statusFuncs: []SetRequestStatusFunc{
					testRequestResource.SetBody(),
					testRequestResource.SetRequestDetails(),
					testRequestResource.SetHeaders(),
					testRequestResource.SetStatusCode(),
					testRequestResource.ResetFailures(),
					testRequestResource.SetCache(),
					testRequestResource.SetError(errBoom),
				},
			},
			want: want{
				failures: 1,
				err:      nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErr := SetRequestResourceStatus(tc.args.rr, tc.args.statusFuncs...)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.Body, testRequestCr.Status.Response.Body); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want response body, +got response body: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.Headers, testRequestCr.Status.Response.Headers); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want response headers, +got response headers: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.StatusCode, testRequestCr.Status.Response.StatusCode); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want response status code, +got response status code: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.StatusCode, testRequestCr.Status.Cache.Response.StatusCode); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want cache status code, +got cahce status code: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.Body, testRequestCr.Status.Cache.Response.Body); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want cache body, +got cahce body: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.Headers, testRequestCr.Status.Cache.Response.Headers); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want cache headers, +got cahce headers: %s", diff)
			}

			if diff := cmp.Diff(tc.want.failures, testRequestCr.Status.Failed); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want failures amount, +got failures amount: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpRequest.Method, testRequestCr.Status.RequestDetails.Method); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want request method, +got request method: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpRequest.URL, testRequestCr.Status.RequestDetails.URL); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want request url, +got request url: %s", diff)
			}
		})
	}
}

func Test_DisposableRequest_SetRequestResourceStatus(t *testing.T) {
	type args struct {
		rr          RequestResource
		statusFuncs []SetRequestStatusFunc
	}
	type want struct {
		err      error
		failures int32
		synced   bool
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				rr: testDisposableResource,
				statusFuncs: []SetRequestStatusFunc{
					testDisposableResource.SetBody(),
					testDisposableResource.SetHeaders(),
					testDisposableResource.SetStatusCode(),
					testDisposableResource.SetSynced(),
				},
			},
			want: want{
				failures: 0,
				err:      nil,
				synced:   true,
			},
		},
		"SetError": {
			args: args{
				rr: testDisposableResource,
				statusFuncs: []SetRequestStatusFunc{
					testDisposableResource.SetError(errBoom),
					testDisposableResource.SetBody(),
					testDisposableResource.SetHeaders(),
					testDisposableResource.SetStatusCode(),
				},
			},
			want: want{
				failures: int32(1),
				err:      nil,
				synced:   false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErr := SetRequestResourceStatus(tc.args.rr, tc.args.statusFuncs...)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.Body, testDisposableCr.Status.Response.Body); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want response body, +got response body: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.Headers, testDisposableCr.Status.Response.Headers); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want response headers, +got response headers: %s", diff)
			}

			if diff := cmp.Diff(tc.args.rr.HttpResponse.StatusCode, testDisposableCr.Status.Response.StatusCode); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want response status code, +got response status code: %s", diff)
			}

			if diff := cmp.Diff(tc.want.synced, testDisposableCr.Status.Synced); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want synced, +got synced: %s", diff)
			}

			if diff := cmp.Diff(tc.want.failures, testDisposableCr.Status.Failed); diff != "" {
				t.Fatalf("SetRequestResourceStatus(...): -want failures, +got failures: %s", diff)
			}
		})
	}
}
