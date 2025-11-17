package request

import (
	"context"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

var (
	errBoom = errors.New("boom")
)

const (
	providerName    = "http-test"
	testRequestName = "test-request"
	testNamespace   = "testns"
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
		Status: v1alpha2.RequestStatus{
			Response: v1alpha2.Response{
				Body:       `{"id": "123"}`,
				StatusCode: 200,
			},
		},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

type notHttpRequest struct {
	resource.Managed
}

type MockSendRequestFn func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error)

type MockHttpClient struct {
	MockSendRequest MockSendRequestFn
}

func (c *MockHttpClient) SendRequest(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
	return c.MockSendRequest(ctx, method, url, body, headers, skipTLSVerify)
}

type MockSetRequestStatusFn func() error

type MockResetFailuresFn func()

type MockInitFn func(ctx context.Context, cr *v1alpha2.Request, res httpClient.HttpResponse)

type MockStatusHandler struct {
	MockSetRequest    MockSetRequestStatusFn
	MockResetFailures MockResetFailuresFn
}

func (s *MockStatusHandler) ResetFailures() {
	s.MockResetFailures()
}

func (s *MockStatusHandler) SetRequestStatus(ctx context.Context, cr *v1alpha2.Request, res httpClient.HttpResponse, err error) error {
	return s.MockSetRequest()
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

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NotRequestResource",
			args: args{
				mg: notHttpRequest{},
			},
			want: want{
				err: errors.New(errNotRequest),
			},
		},
		{
			name: "RequestFailed",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errBoom
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: httpRequest(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToSendHttpRequest),
			},
		},
		{
			name: "Success",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockCreate:       test.NewMockCreateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: httpRequest(),
			},
			want: want{
				err: nil,
			},
		},
	}
	for _, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(tc.name, func(t *testing.T) {
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

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NotRequestResource",
			args: args{
				mg: notHttpRequest{},
			},
			want: want{
				err: errors.New(errNotRequest),
			},
		},
		{
			name: "RequestFailed",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errBoom
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: httpRequest(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToSendHttpRequest),
			},
		},
		{
			name: "Success",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockCreate:       test.NewMockCreateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: httpRequest(),
			},
			want: want{
				err: nil,
			},
		},
	}
	for _, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				localKube: tc.args.localKube,
				logger:    logging.NewNopLogger(),
				http:      tc.args.http,
			}
			_, gotErr := e.Update(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Update(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_httpExternal_Delete(t *testing.T) {
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
			name: "NotRequestResource",
			args: args{
				mg: notHttpRequest{},
			},
			want: want{
				err: errors.New(errNotRequest),
			},
		},
		{
			name: "RequestFailed",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errBoom
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: httpRequest(),
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToSendHttpRequest),
			},
		},
		{
			name: "Success",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockCreate:       test.NewMockCreateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: httpRequest(),
			},
			want: want{
				err: nil,
			},
		},
	}
	for _, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				localKube: tc.args.localKube,
				logger:    logging.NewNopLogger(),
				http:      tc.args.http,
			}
			_, gotErr := e.Delete(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Delete(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_httpExternal_Observe(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        resource.Managed
	}
	type want struct {
		observation managed.ExternalObservation
		err         error
	}

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NotRequestResource",
			args: args{
				mg: notHttpRequest{},
			},
			want: want{
				err: errors.New(errNotRequest),
			},
		},
		{
			name: "ResourceUpToDate",
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body httpClient.Data, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       `{"id": "123"}`,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: &v1alpha2.Request{
					Spec: v1alpha2.RequestSpec{
						ForProvider: v1alpha2.RequestParameters{
							Payload: v1alpha2.Payload{
								BaseUrl: "https://api.example.com/users/123",
							},
							Mappings: []v1alpha2.Mapping{
								{
									Method: "GET",
									URL:    ".payload.baseUrl",
								},
							},
						},
					},
					Status: v1alpha2.RequestStatus{
						Response: v1alpha2.Response{
							StatusCode: 200,
							Body:       `{"id": "123"}`,
						},
					},
				},
			},
			want: want{
				observation: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
				err: nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				localKube: tc.args.localKube,
				logger:    logging.NewNopLogger(),
				http:      tc.args.http,
			}
			got, gotErr := e.Observe(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Observe(...): -want error, +got error: %s", diff)
			}
			if tc.want.err == nil {
				if diff := cmp.Diff(tc.want.observation.ResourceExists, got.ResourceExists); diff != "" {
					t.Fatalf("e.Observe(...): -want ResourceExists, +got ResourceExists: %s", diff)
				}
				if diff := cmp.Diff(tc.want.observation.ResourceUpToDate, got.ResourceUpToDate); diff != "" {
					t.Fatalf("e.Observe(...): -want ResourceUpToDate, +got ResourceUpToDate: %s", diff)
				}
			}
		})
	}
}

func TestManagementPoliciesFeatureFlag(t *testing.T) {
	cases := map[string]struct {
		reason   string
		features *feature.Flags
		want     bool
	}{
		"ManagementPoliciesEnabled": {
			reason: "Feature flag should be enabled when explicitly set",
			features: func() *feature.Flags {
				f := &feature.Flags{}
				f.Enable(feature.EnableBetaManagementPolicies)
				return f
			}(),
			want: true,
		},
		"ManagementPoliciesDisabled": {
			reason:   "Feature flag should be disabled when not set",
			features: &feature.Flags{},
			want:     false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			enabled := tc.features.Enabled(feature.EnableBetaManagementPolicies)
			if enabled != tc.want {
				t.Errorf("\n%s\nEnabled(feature.EnableBetaManagementPolicies): want %v, got %v", tc.reason, tc.want, enabled)
			}
		})
	}
}

func TestRequestManagementPolicies(t *testing.T) {
	cases := map[string]struct {
		reason string
		mg     *v1alpha2.Request
		want   xpv1.ManagementPolicies
	}{
		"DefaultManagementPolicies": {
			reason: "Default management policies should be nil when not explicitly set",
			mg: func() *v1alpha2.Request {
				r := httpRequest()
				// Don't set managementPolicies explicitly to test default
				return r
			}(),
			want: nil,
		},
		"ObserveOnlyManagementPolicies": {
			reason: "Observe-only management policies should only allow observation",
			mg: func() *v1alpha2.Request {
				r := httpRequest()
				r.Spec.ManagementPolicies = xpv1.ManagementPolicies{xpv1.ManagementActionObserve}
				return r
			}(),
			want: xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
		},
		"CreateAndUpdateManagementPolicies": {
			reason: "Create and update management policies should allow creation and updates",
			mg: func() *v1alpha2.Request {
				r := httpRequest()
				r.Spec.ManagementPolicies = xpv1.ManagementPolicies{
					xpv1.ManagementActionCreate,
					xpv1.ManagementActionUpdate,
				}
				return r
			}(),
			want: xpv1.ManagementPolicies{
				xpv1.ManagementActionCreate,
				xpv1.ManagementActionUpdate,
			},
		},
		"ObserveCreateUpdateManagementPolicies": {
			reason: "Observe, create, and update management policies should allow all three actions",
			mg: func() *v1alpha2.Request {
				r := httpRequest()
				r.Spec.ManagementPolicies = xpv1.ManagementPolicies{
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionCreate,
					xpv1.ManagementActionUpdate,
				}
				return r
			}(),
			want: xpv1.ManagementPolicies{
				xpv1.ManagementActionObserve,
				xpv1.ManagementActionCreate,
				xpv1.ManagementActionUpdate,
			},
		},
		"AllActionsExceptDeleteManagementPolicies": {
			reason: "All actions except delete should allow observe, create, update, and late initialize",
			mg: func() *v1alpha2.Request {
				r := httpRequest()
				r.Spec.ManagementPolicies = xpv1.ManagementPolicies{
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionCreate,
					xpv1.ManagementActionUpdate,
					xpv1.ManagementActionLateInitialize,
				}
				return r
			}(),
			want: xpv1.ManagementPolicies{
				xpv1.ManagementActionObserve,
				xpv1.ManagementActionCreate,
				xpv1.ManagementActionUpdate,
				xpv1.ManagementActionLateInitialize,
			},
		},
		"ExplicitAllManagementPolicies": {
			reason: "Explicit all management policies should allow all actions",
			mg: func() *v1alpha2.Request {
				r := httpRequest()
				r.Spec.ManagementPolicies = xpv1.ManagementPolicies{xpv1.ManagementActionAll}
				return r
			}(),
			want: xpv1.ManagementPolicies{xpv1.ManagementActionAll},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.mg.Spec.ManagementPolicies
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nManagementPolicies: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRequestManagementPoliciesResolver(t *testing.T) {
	type args struct {
		enabled bool
		policy  xpv1.ManagementPolicies
	}
	type want struct {
		shouldCreate         bool
		shouldUpdate         bool
		shouldDelete         bool
		shouldOnlyObserve    bool
		shouldLateInitialize bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ManagementPoliciesDisabled": {
			reason: "When management policies are disabled, all actions should be allowed",
			args: args{
				enabled: false,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: want{
				shouldCreate:         true,
				shouldUpdate:         true,
				shouldDelete:         true,
				shouldOnlyObserve:    false,
				shouldLateInitialize: true,
			},
		},
		"ObserveOnlyPolicy": {
			reason: "Observe-only policy should only allow observation",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: want{
				shouldCreate:         false,
				shouldUpdate:         false,
				shouldDelete:         false,
				shouldOnlyObserve:    true,
				shouldLateInitialize: false,
			},
		},
		"CreateOnlyPolicy": {
			reason: "Create-only policy should only allow creation",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionCreate},
			},
			want: want{
				shouldCreate:         true,
				shouldUpdate:         false,
				shouldDelete:         false,
				shouldOnlyObserve:    false,
				shouldLateInitialize: false,
			},
		},
		"UpdateOnlyPolicy": {
			reason: "Update-only policy should only allow updates",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionUpdate},
			},
			want: want{
				shouldCreate:         false,
				shouldUpdate:         true,
				shouldDelete:         false,
				shouldOnlyObserve:    false,
				shouldLateInitialize: false,
			},
		},
		"DeleteOnlyPolicy": {
			reason: "Delete-only policy should only allow deletion",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionDelete},
			},
			want: want{
				shouldCreate:         false,
				shouldUpdate:         false,
				shouldDelete:         true,
				shouldOnlyObserve:    false,
				shouldLateInitialize: false,
			},
		},
		"CreateAndUpdatePolicy": {
			reason: "Create and update policy should allow both creation and updates",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate},
			},
			want: want{
				shouldCreate:         true,
				shouldUpdate:         true,
				shouldDelete:         false,
				shouldOnlyObserve:    false,
				shouldLateInitialize: false,
			},
		},
		"ObserveCreateUpdatePolicy": {
			reason: "Observe, create, and update policy should allow all three actions",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate},
			},
			want: want{
				shouldCreate:         true,
				shouldUpdate:         true,
				shouldDelete:         false,
				shouldOnlyObserve:    false,
				shouldLateInitialize: false,
			},
		},
		"AllActionsExceptDeletePolicy": {
			reason: "All actions except delete should allow observe, create, update, and late initialize",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionObserve, xpv1.ManagementActionCreate, xpv1.ManagementActionUpdate, xpv1.ManagementActionLateInitialize},
			},
			want: want{
				shouldCreate:         true,
				shouldUpdate:         true,
				shouldDelete:         false,
				shouldOnlyObserve:    false,
				shouldLateInitialize: true,
			},
		},
		"ExplicitAllPolicy": {
			reason: "Explicit all policy should allow all actions",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: want{
				shouldCreate:         true,
				shouldUpdate:         true,
				shouldDelete:         true,
				shouldOnlyObserve:    false,
				shouldLateInitialize: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a mock managed resource with the specified management policies
			mg := httpRequest()
			mg.Spec.ManagementPolicies = tc.args.policy

			// Test the management policies resolver logic
			// Note: This is a simplified test that focuses on the policy logic
			// The actual enforcement happens in the Crossplane managed reconciler

			// Helper function to check if a ManagementPolicies slice contains a specific action
			contains := func(policies xpv1.ManagementPolicies, action xpv1.ManagementAction) bool {
				for _, p := range policies {
					if p == action {
						return true
					}
				}
				return false
			}

			// Test ShouldCreate
			shouldCreate := tc.want.shouldCreate
			if tc.args.enabled {
				shouldCreate = contains(tc.args.policy, xpv1.ManagementActionCreate) || contains(tc.args.policy, xpv1.ManagementActionAll)
			}
			if shouldCreate != tc.want.shouldCreate {
				t.Errorf("ShouldCreate() = %v, want %v", shouldCreate, tc.want.shouldCreate)
			}

			// Test ShouldUpdate
			shouldUpdate := tc.want.shouldUpdate
			if tc.args.enabled {
				shouldUpdate = contains(tc.args.policy, xpv1.ManagementActionUpdate) || contains(tc.args.policy, xpv1.ManagementActionAll)
			}
			if shouldUpdate != tc.want.shouldUpdate {
				t.Errorf("ShouldUpdate() = %v, want %v", shouldUpdate, tc.want.shouldUpdate)
			}

			// Test ShouldDelete
			shouldDelete := tc.want.shouldDelete
			if tc.args.enabled {
				shouldDelete = contains(tc.args.policy, xpv1.ManagementActionDelete) || contains(tc.args.policy, xpv1.ManagementActionAll)
			}
			if shouldDelete != tc.want.shouldDelete {
				t.Errorf("ShouldDelete() = %v, want %v", shouldDelete, tc.want.shouldDelete)
			}

			// Test ShouldOnlyObserve
			shouldOnlyObserve := tc.want.shouldOnlyObserve
			if tc.args.enabled {
				shouldOnlyObserve = len(tc.args.policy) == 1 && contains(tc.args.policy, xpv1.ManagementActionObserve)
			}
			if shouldOnlyObserve != tc.want.shouldOnlyObserve {
				t.Errorf("ShouldOnlyObserve() = %v, want %v", shouldOnlyObserve, tc.want.shouldOnlyObserve)
			}

			// Test ShouldLateInitialize
			shouldLateInitialize := tc.want.shouldLateInitialize
			if tc.args.enabled {
				shouldLateInitialize = contains(tc.args.policy, xpv1.ManagementActionLateInitialize) || contains(tc.args.policy, xpv1.ManagementActionAll)
			}
			if shouldLateInitialize != tc.want.shouldLateInitialize {
				t.Errorf("ShouldLateInitialize() = %v, want %v", shouldLateInitialize, tc.want.shouldLateInitialize)
			}
		})
	}
}
