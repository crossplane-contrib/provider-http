/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package desposiblerequest

import (
	"context"
	"testing"
	"time"

	"github.com/arielsepton/provider-http/apis/desposiblerequest/v1alpha1"

	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments. Crossplane encourages the
// use of table driven unit tests. The tests of the crossplane-runtime project
// are representative of the testing style Crossplane encourages.
//
// https://github.com/golang/go/wiki/TestComments
// https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md#contributing-code

var (
	errBoom = errors.New("boom")
)

const (
	providerName              = "http-test"
	testDesposibleRequestName = "test-request"
	testNamespace             = "testns"
)

var testHeaders = map[string][]string{
	"fruits":                {"apple", "banana", "orange"},
	"colors":                {"red", "green", "blue"},
	"countries":             {"USA", "UK", "India", "Germany"},
	"programming_languages": {"Go", "Python", "JavaScript"},
}

var testTimeout = &v1.Duration{
	Duration: 5 * time.Minute,
}

const (
	testURL    = "testchart"
	testMethod = "GET"
	testBody   = "{\"key1\": \"value1\"}"
)

type httpDesposibleRequestModifier func(request *v1alpha1.DesposibleRequest)

func httpDesposibleRequest(rm ...httpDesposibleRequestModifier) *v1alpha1.DesposibleRequest {
	r := &v1alpha1.DesposibleRequest{
		ObjectMeta: v1.ObjectMeta{
			Name:      testDesposibleRequestName,
			Namespace: testNamespace,
		},
		Spec: v1alpha1.DesposibleRequestSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: providerName,
				},
			},
			ForProvider: v1alpha1.DesposibleRequestParameters{
				URL:         testURL,
				Method:      testMethod,
				Headers:     testHeaders,
				Body:        testBody,
				WaitTimeout: testTimeout,
			},
		},
		Status: v1alpha1.DesposibleRequestStatus{},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

type MockSendRequestFn func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error)

type MockHttpClient struct {
	MockSendRequest MockSendRequestFn
}

func (c *MockHttpClient) SendRequest(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
	return c.MockSendRequest(ctx, method, url, body, headers)
}

type notHttpDesposibleRequest struct {
	resource.Managed
}

func Test_httpExternal_Create(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        resource.Managed
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NotDesposibleRequestResource": {
			args: args{
				mg: notHttpDesposibleRequest{},
			},
			want: want{
				err: errors.New(errNotDesposibleRequest),
			},
		},
		"DesposibleRequestFailed": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{}, errBoom
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpDesposibleRequest(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToSendHttpDesposibleRequest),
			},
		},
		"Success": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockCreate:       test.NewMockCreateFn(nil),
				},
				mg: httpDesposibleRequest(),
			},
			want: want{
				err: nil,
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
			_, gotErr := e.Create(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Create(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_httpExternal_Update(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        resource.Managed
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NotDesposibleRequestResource": {
			args: args{
				mg: notHttpDesposibleRequest{},
			},
			want: want{
				err: errors.New(errNotDesposibleRequest),
			},
		},
		"DesposibleRequestFailed": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{}, errBoom
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: httpDesposibleRequest(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToSendHttpDesposibleRequest),
			},
		},
		"Success": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp httpClient.HttpResponse, err error) {
						return httpClient.HttpResponse{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockCreate:       test.NewMockCreateFn(nil),
				},
				mg: httpDesposibleRequest(),
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			e := &external{
				localKube: tc.args.localKube,
				logger:    logging.NewNopLogger(),
				http:      tc.args.http}
			_, gotErr := e.Update(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Update(...): -want error, +got error: %s", diff)
			}
		})
	}
}
