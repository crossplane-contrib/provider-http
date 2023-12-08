package statushandler

import (
	"context"
	"strconv"
	"testing"

	"github.com/pkg/errors"

	"github.com/arielsepton/provider-http/apis/request/v1alpha1"
	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"github.com/arielsepton/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
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
	testURL    = "https://example.com/another"
	testMethod = "GET"
	testBody   = "{\"key1\": \"value1\"}"
)

var (
	testPostMapping = v1alpha1.Mapping{
		Method: "POST",
		Body:   "{ username: .payload.body.username, email: .payload.body.email }",
		URL:    ".payload.baseUrl",
	}

	testPutMapping = v1alpha1.Mapping{
		Method: "PUT",
		Body:   "{ username: \"john_doe_new_username\" }",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testGetMapping = v1alpha1.Mapping{
		Method: "GET",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}

	testDeleteMapping = v1alpha1.Mapping{
		Method: "DELETE",
		URL:    "(.payload.baseUrl + \"/\" + .response.body.id)",
	}
)

var (
	testForProvider = v1alpha1.RequestParameters{
		Payload: v1alpha1.Payload{
			Body:    "{\"username\": \"john_doe\", \"email\": \"john.doe@example.com\"}",
			BaseUrl: "https://api.example.com/users",
		},
		Mappings: []v1alpha1.Mapping{
			testPostMapping,
			testGetMapping,
			testPutMapping,
			testDeleteMapping,
		},
	}
)

var testCr = &v1alpha1.Request{
	Spec: v1alpha1.RequestSpec{
		ForProvider: testForProvider,
	},
}

func Test_SetRequestStatus(t *testing.T) {
	type args struct {
		localKube client.Client
		cr        *v1alpha1.Request
		res       httpClient.HttpResponse
		err       error
	}
	type want struct {
		err           error
		failuresIndex int32
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				cr: testCr,
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				res: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"id":"123","username":"john_doe"}`,
					Headers:    testHeaders,
					Method:     testMethod,
				},
				err: nil,
			},
			want: want{
				err:           nil,
				failuresIndex: 0,
			},
		},
		"StatusCodeFailed": {
			args: args{
				cr: testCr,
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				res: httpClient.HttpResponse{
					StatusCode: 400,
					Body:       `{"id":"123","username":"john_doe"}`,
					Headers:    testHeaders,
					Method:     testMethod,
				},
				err: nil,
			},
			want: want{
				err:           errors.Errorf(utils.ErrStatusCode, testMethod, strconv.Itoa(400)),
				failuresIndex: 1,
			},
		},
		"RequestFailed": {
			args: args{
				cr: testCr,
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				res: httpClient.HttpResponse{
					StatusCode: 200,
					Body:       `{"id":"123","username":"john_doe"}`,
					Headers:    testHeaders,
					Method:     testMethod,
				},
				err: errBoom,
			},
			want: want{
				err:           errBoom,
				failuresIndex: 2,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &requestStatusHandler{
				localKube: tc.args.localKube,
				logger:    logging.NewNopLogger(),
			}
			gotErr := r.SetRequestStatus(context.Background(), tc.args.cr, tc.args.res, tc.args.err)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.SetRequestStatus(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tc.args.cr.Status.Failed, tc.want.failuresIndex); diff != "" {
				t.Fatalf("SetRequestStatus(...): -want Status.Failed, +got Status.Failed: %s", diff)
			}

			if tc.args.err != nil {
				if diff := cmp.Diff(tc.args.cr.Status.Error, tc.args.err.Error()); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Error, +got Status.Error: %s", diff)
				}
			}

			if gotErr == nil {
				if diff := cmp.Diff(tc.args.cr.Status.Response.Body, tc.args.res.Body); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Response.Body, +got Status.Response.Body: %s", diff)
				}

				if diff := cmp.Diff(tc.args.cr.Status.Response.StatusCode, tc.args.res.StatusCode); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Response.StatusCode, +got Status.Response.StatusCode: %s", diff)
				}

				if diff := cmp.Diff(tc.args.cr.Status.Response.Headers, tc.args.res.Headers); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Response.Headers, +got Status.Response.Headers: %s", diff)
				}

				if diff := cmp.Diff(tc.args.cr.Status.Response.Method, tc.args.res.Method); diff != "" {
					t.Fatalf("SetRequestStatus(...): -want Status.Response.Method, +got Status.Response.Method: %s", diff)
				}
			}
		})
	}
}
