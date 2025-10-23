package request

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

type MockHttpClient struct {
	MockSendRequest func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error)
}

func (m *MockHttpClient) SendRequest(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
	return m.MockSendRequest(ctx, method, url, body, headers, skipTLSVerify)
}

type notNamespacedRequest struct {
	resource.Managed
}

func namespacedRequest(modifiers ...func(*v1alpha2.Request)) *v1alpha2.Request {
	cr := &v1alpha2.Request{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-request",
			Namespace: "default",
		},
		Spec: v1alpha2.RequestSpec{
			ForProvider: v1alpha2.RequestParameters{
				Payload: v1alpha2.Payload{
					Body:    `{"test": true}`,
					BaseUrl: "http://example.com/test",
				},
				Mappings: []v1alpha2.Mapping{
					{
						Method: "POST",
						Action: "CREATE",
						URL:    ".payload.baseUrl",
						Body:   ".payload.body",
					},
				},
			},
		},
	}

	for _, modifier := range modifiers {
		modifier(cr)
	}

	return cr
}

func namespacedRequestWithDeletion() *v1alpha2.Request {
	now := metav1.Now()
	return namespacedRequest(func(cr *v1alpha2.Request) {
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
			name: "NotNamespacedRequest",
			args: args{
				mg: notNamespacedRequest{},
			},
			want: want{
				err: errors.New(errNotRequest),
			},
		},
		{
			name: "ResourceBeingDeleted",
			args: args{
				mg: namespacedRequestWithDeletion(),
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

func TestDeployAction_SkipSecretInjectionDuringDeletion(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		cr        *v1alpha2.Request
		action    string
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
			name: "SkipSecretInjectionForDeletingResource",
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
				cr:     namespacedRequestWithDeletion(),
				action: v1alpha2.ActionRemove,
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "NormalSecretInjectionForNonDeletingResource",
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
				cr:     namespacedRequest(),
				action: v1alpha2.ActionCreate,
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

			err := e.deployAction(context.Background(), tc.args.cr, tc.args.action)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("deployAction(...): -want error, +got error: %s", diff)
			}
		})
	}
}
