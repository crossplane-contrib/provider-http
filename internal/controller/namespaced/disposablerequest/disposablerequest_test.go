/*
Copyright 2023 The Crossplane Authors.

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

package disposablerequest

import (
	"context"
	"testing"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/disposablerequest/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errBoom = errors.New("boom")
)

type MockHttpClient struct {
	MockSendRequest func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error)
}

func (m *MockHttpClient) SendRequest(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
	return m.MockSendRequest(ctx, method, url, body, headers, skipTLSVerify)
}

type notNamespacedDisposableRequest struct {
	resource.Managed
}

func namespacedDisposableRequest(modifiers ...func(*v1alpha2.DisposableRequest)) *v1alpha2.DisposableRequest {
	cr := &v1alpha2.DisposableRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-disposable",
			Namespace: "default",
		},
		Spec: v1alpha2.DisposableRequestSpec{
			ForProvider: v1alpha2.DisposableRequestParameters{
				URL:    "http://example.com/test",
				Method: "POST",
				Body:   `{"test": true}`,
			},
		},
	}

	for _, modifier := range modifiers {
		modifier(cr)
	}

	return cr
}

func namespacedDisposableRequestWithDeletion() *v1alpha2.DisposableRequest {
	now := metav1.Now()
	return namespacedDisposableRequest(func(cr *v1alpha2.DisposableRequest) {
		cr.DeletionTimestamp = &now
	})
}

func TestObserve(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        resource.Managed
	}
	type want struct {
		obs managed.ExternalObservation
		err error
	}

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NotNamespacedDisposableRequest",
			args: args{
				mg: notNamespacedDisposableRequest{},
			},
			want: want{
				err: errors.New(errNotNamespacedDisposableRequest),
			},
		},
		{
			name: "ResourceBeingDeleted",
			args: args{
				mg: namespacedDisposableRequestWithDeletion(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
		{
			name: "ResourceNotSynced",
			args: args{
				mg: namespacedDisposableRequest(),
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				logger:    logging.NewNopLogger(),
				localKube: tc.args.localKube,
				http:      tc.args.http,
			}

			got, err := e.Observe(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Observe(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.obs, got); diff != "" {
				t.Errorf("Observe(...): -want, +got: %s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        resource.Managed
	}
	type want struct {
		err error
	}

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NotNamespacedDisposableRequest",
			args: args{
				mg: notNamespacedDisposableRequest{},
			},
			want: want{
				err: errors.New(errNotNamespacedDisposableRequest),
			},
		},
		{
			name: "HttpRequestFailed",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errBoom
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: namespacedDisposableRequest(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToSendHttpDisposableRequest),
			},
		},
		{
			name: "Success",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       `{"result": "success"}`,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: namespacedDisposableRequest(),
			},
			want: want{
				err: nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				logger:    logging.NewNopLogger(),
				localKube: tc.args.localKube,
				http:      tc.args.http,
			}

			_, err := e.Create(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Create(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        resource.Managed
	}
	type want struct {
		err error
	}

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NotNamespacedDisposableRequest",
			args: args{
				mg: notNamespacedDisposableRequest{},
			},
			want: want{
				err: errors.New(errNotNamespacedDisposableRequest),
			},
		},
		{
			name: "Success",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       `{"result": "updated"}`,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: namespacedDisposableRequest(),
			},
			want: want{
				err: nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				logger:    logging.NewNopLogger(),
				localKube: tc.args.localKube,
				http:      tc.args.http,
			}

			_, err := e.Update(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Update(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type args struct {
		mg resource.Managed
	}
	type want struct {
		result managed.ExternalDelete
		err    error
	}

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Success",
			args: args{
				mg: namespacedDisposableRequest(),
			},
			want: want{
				result: managed.ExternalDelete{},
				err:    nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				logger: logging.NewNopLogger(),
			}

			got, err := e.Delete(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Delete(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("Delete(...): -want result, +got result: %s", diff)
			}
		})
	}
}
