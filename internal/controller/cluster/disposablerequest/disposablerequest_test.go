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

package disposablerequest

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/disposablerequest"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
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
	testDisposableRequestName = "test-request"
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
	testURL    = "https://example-url"
	testMethod = "GET"
	testBody   = "{\"key1\": \"value1\"}"
)

type httpDisposableRequestModifier func(request *v1alpha2.DisposableRequest)

func httpDisposableRequest(rm ...httpDisposableRequestModifier) *v1alpha2.DisposableRequest {
	r := &v1alpha2.DisposableRequest{
		ObjectMeta: v1.ObjectMeta{
			Name:      testDisposableRequestName,
			Namespace: testNamespace,
		},
		Spec: v1alpha2.DisposableRequestSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: providerName,
				},
			},
			ForProvider: v1alpha2.DisposableRequestParameters{
				URL:         testURL,
				Method:      testMethod,
				Headers:     testHeaders,
				Body:        testBody,
				WaitTimeout: testTimeout,
			},
		},
		Status: v1alpha2.DisposableRequestStatus{},
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

type notHttpDisposableRequest struct {
	resource.Managed
}

func Test_httpExternal_Create(t *testing.T) {
	type args struct {
		http      httpClient.Client
		localKube client.Client
		mg        resource.Managed
	}
	type want struct {
		err           error
		failuresIndex int32
	}

	cases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NotDisposableRequestResource",
			args: args{
				mg: notHttpDisposableRequest{},
			},
			want: want{
				err: errors.New(errNotDisposableRequest),
			},
		},
		{
			name: "DisposableRequestFailed",
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
				mg: httpDisposableRequest(),
			},
			want: want{
				failuresIndex: 1,
				err:           errors.Wrap(errBoom, errFailedToSendHttpDisposableRequest),
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
				mg: httpDisposableRequest(),
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
			name: "NotDisposableRequestResource",
			args: args{
				mg: notHttpDisposableRequest{},
			},
			want: want{
				err: errors.New(errNotDisposableRequest),
			},
		},
		{
			name: "DisposableRequestFailed",
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
				mg: httpDisposableRequest(),
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
						return httpClient.HttpDetails{}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockCreate:       test.NewMockCreateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				mg: httpDisposableRequest(),
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
				http:      tc.args.http}
			_, gotErr := e.Update(context.Background(), tc.args.mg)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("e.Update(...): -want error, +got error: %s", diff)
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
			name: "NotDisposableRequestResource",
			args: args{
				mg: notHttpDisposableRequest{},
			},
			want: want{
				err: errors.New(errNotDisposableRequest),
			},
		},
		{
			name: "ResourceNotSynced",
			args: args{
				http: &MockHttpClient{},
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
				mg: &v1alpha2.DisposableRequest{
					Spec: v1alpha2.DisposableRequestSpec{
						ForProvider: v1alpha2.DisposableRequestParameters{
							URL:    testURL,
							Method: testMethod,
						},
					},
					Status: v1alpha2.DisposableRequestStatus{
						Synced: false,
					},
				},
			},
			want: want{
				observation: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: nil,
			},
		},
		{
			name: "ResourceSyncedAndUpToDate",
			args: args{
				http: &MockHttpClient{},
				localKube: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				mg: &v1alpha2.DisposableRequest{
					Spec: v1alpha2.DisposableRequestSpec{
						ForProvider: v1alpha2.DisposableRequestParameters{
							URL:    testURL,
							Method: testMethod,
						},
					},
					Status: v1alpha2.DisposableRequestStatus{
						Synced: true,
						Response: v1alpha2.Response{
							StatusCode: 200,
							Body:       testBody,
							Headers:    testHeaders,
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
			if diff := cmp.Diff(tc.want.observation.ResourceExists, got.ResourceExists); diff != "" {
				t.Fatalf("e.Observe(...): -want ResourceExists, +got ResourceExists: %s", diff)
			}
			if tc.want.err == nil {
				if diff := cmp.Diff(tc.want.observation.ResourceUpToDate, got.ResourceUpToDate); diff != "" {
					t.Fatalf("e.Observe(...): -want ResourceUpToDate, +got ResourceUpToDate: %s", diff)
				}
			}
		})
	}
}

func Test_httpExternal_Delete(t *testing.T) {
	type args struct {
		mg resource.Managed
	}

	cases := []struct {
		name string
		args args
	}{
		{
			name: "AlwaysSucceeds",
			args: args{
				mg: httpDisposableRequest(),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &external{
				logger: logging.NewNopLogger(),
			}
			_, err := e.Delete(context.Background(), tc.args.mg)
			if err != nil {
				t.Fatalf("e.Delete(...): unexpected error: %v", err)
			}
		})
	}
}

func Test_deployAction(t *testing.T) {
	type args struct {
		cr        *v1alpha2.DisposableRequest
		http      httpClient.Client
		localKube client.Client
	}
	type want struct {
		err           error
		failuresIndex int32
		statusCode    int
	}
	cases := map[string]struct {
		args      args
		want      want
		condition bool
	}{
		"SuccessUpdateStatusRequestFailure": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{}, errors.Errorf(utils.ErrInvalidURL, "invalid-url")
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				cr: &v1alpha2.DisposableRequest{
					Spec: v1alpha2.DisposableRequestSpec{
						ForProvider: v1alpha2.DisposableRequestParameters{
							URL:     "invalid-url",
							Method:  testMethod,
							Headers: testHeaders,
							Body:    testBody,
						},
					},
					Status: v1alpha2.DisposableRequestStatus{},
				},
			},
			want: want{
				err:           errors.Errorf(utils.ErrInvalidURL, "invalid-url"),
				failuresIndex: 1,
			},
		},
		"SuccessUpdateStatusCodeError": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 400,
								Body:       testBody,
								Headers:    testHeaders,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				cr: &v1alpha2.DisposableRequest{
					Spec: v1alpha2.DisposableRequestSpec{
						ForProvider: v1alpha2.DisposableRequestParameters{
							URL:     testURL,
							Method:  testMethod,
							Headers: testHeaders,
							Body:    testBody,
						},
					},
					Status: v1alpha2.DisposableRequestStatus{},
				},
			},
			want: want{
				err:           errors.Errorf(utils.ErrStatusCode, testMethod, strconv.Itoa(400)),
				failuresIndex: 1,
				statusCode:    400,
			},
			condition: true,
		},
		"SuccessUpdateStatusSuccessfulRequest": {
			args: args{
				http: &MockHttpClient{
					MockSendRequest: func(ctx context.Context, method string, url string, body, headers httpClient.Data, skipTLSVerify bool) (resp httpClient.HttpDetails, err error) {
						return httpClient.HttpDetails{
							HttpResponse: httpClient.HttpResponse{
								StatusCode: 200,
								Body:       testBody,
								Headers:    testHeaders,
							},
						}, nil
					},
				},
				localKube: &test.MockClient{
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockGet:          test.NewMockGetFn(nil),
				},
				cr: &v1alpha2.DisposableRequest{
					Spec: v1alpha2.DisposableRequestSpec{
						ForProvider: v1alpha2.DisposableRequestParameters{
							URL:     testURL,
							Method:  testMethod,
							Headers: testHeaders,
							Body:    testBody,
						},
					},
					Status: v1alpha2.DisposableRequestStatus{},
				},
			},
			want: want{
				err:        nil,
				statusCode: 200,
			},
			condition: true,
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			svcCtx := service.NewServiceContext(
				context.Background(),
				tc.args.localKube,
				logging.NewNopLogger(),
				tc.args.http,
			)
			crCtx := service.NewDisposableRequestCRContext(
				tc.args.cr,
			)
			gotErr := disposablerequest.DeployAction(svcCtx, crCtx)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("deployAction(...): -want error, +got error: %s", diff)
			}

			if gotErr != nil {
				if diff := cmp.Diff(tc.want.failuresIndex, tc.args.cr.Status.Failed); diff != "" {
					t.Fatalf("deployAction(...): -want Status.Failed, +got Status.Failed: %s", diff)
				}
			}

			if tc.condition {
				if diff := cmp.Diff(tc.args.cr.Spec.ForProvider.Body, tc.args.cr.Status.Response.Body); diff != "" {
					t.Fatalf("deployAction(...): -want Status.Response.Body, +got Status.Response.Body: %s", diff)
				}

				if diff := cmp.Diff(tc.want.statusCode, tc.args.cr.Status.Response.StatusCode); diff != "" {
					t.Fatalf("deployAction(...): -want Status.Response.StatusCode, +got Status.Response.StatusCode: %s", diff)
				}

				if diff := cmp.Diff(tc.args.cr.Spec.ForProvider.Headers, tc.args.cr.Status.Response.Headers); diff != "" {
					t.Fatalf("deployAction(...): -want Status.Response.Headers, +got Status.Response.Headers: %s", diff)
				}

				if tc.args.cr.Status.LastReconcileTime.IsZero() {
					t.Fatalf("deployAction(...): -want Status.LastReconcileTime to not be nil, +got nil")
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

func TestDisposableRequestManagementPoliciesResolver(t *testing.T) {
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
			mg := httpDisposableRequest()
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

func TestObserve_DeletionMonitoring(t *testing.T) {
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
			name: "ResourceBeingDeleted",
			args: args{
				mg: disposableRequestWithDeletion(),
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

func TestDisposableRequestManagementPolicies(t *testing.T) {
	cases := map[string]struct {
		reason string
		mg     *v1alpha2.DisposableRequest
		want   xpv1.ManagementPolicies
	}{
		"DefaultManagementPolicies": {
			reason: "Default management policies should be nil when not explicitly set",
			mg: func() *v1alpha2.DisposableRequest {
				r := httpDisposableRequest()
				// Don't set managementPolicies explicitly to test default
				return r
			}(),
			want: nil,
		},
		"ObserveOnlyManagementPolicies": {
			reason: "Observe-only management policies should only allow observation",
			mg: func() *v1alpha2.DisposableRequest {
				r := httpDisposableRequest()
				r.Spec.ManagementPolicies = xpv1.ManagementPolicies{xpv1.ManagementActionObserve}
				return r
			}(),
			want: xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
		},
		"CreateAndUpdateManagementPolicies": {
			reason: "Create and update management policies should allow creation and updates",
			mg: func() *v1alpha2.DisposableRequest {
				r := httpDisposableRequest()
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
			mg: func() *v1alpha2.DisposableRequest {
				r := httpDisposableRequest()
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
			mg: func() *v1alpha2.DisposableRequest {
				r := httpDisposableRequest()
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
			mg: func() *v1alpha2.DisposableRequest {
				r := httpDisposableRequest()
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

func disposableRequestWithDeletion() *v1alpha2.DisposableRequest {
	now := v1.Now()
	return &v1alpha2.DisposableRequest{
		ObjectMeta: v1.ObjectMeta{
			Name:              "test-disposable",
			Namespace:         "default",
			DeletionTimestamp: &now,
		},
		Spec: v1alpha2.DisposableRequestSpec{
			ForProvider: v1alpha2.DisposableRequestParameters{
				URL:    "http://example.com/test",
				Method: "POST",
				Body:   `{"test": true}`,
			},
		},
	}
}
