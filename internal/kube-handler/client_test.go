package kubehandler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	errorspkg "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errBoom = errors.New("boom")
)

func createSpecificSecret(name, namespace, key, value string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if key != "" && value != "" {
		secret.Data = map[string][]byte{
			key: []byte(value),
		}
	}

	return secret
}

func Test_GetSecret(t *testing.T) {
	type args struct {
		localKube client.Client
		name      string
		namespace string
	}
	type want struct {
		result *corev1.Secret
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldGetSecret": {
			args: args{
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("specific-secret-name", "specific-secret-namespace", "specific-key", "specific-value")
						return nil
					},
				},
				name:      "specific-secret-name",
				namespace: "specific-secret-namespace",
			},
			want: want{
				result: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "specific-secret-namespace",
						Name:      "specific-secret-name",
					},
					Data: map[string][]uint8{
						"specific-key": []byte("specific-value"),
					},
				},
				err: nil,
			},
		},
		"ShouldFail": {
			args: args{
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				name:      "secret",
				namespace: "default",
			},
			want: want{
				result: nil,
				err:    errorspkg.Wrap(errBoom, fmt.Sprintf(errGetSecret, "secret", "default")),
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got, gotErr := GetSecret(context.Background(), tc.args.localKube, tc.args.name, tc.args.namespace)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("GetSecret(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("GetSecret(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_GetOrCreateSecret(t *testing.T) {
	type args struct {
		localKube client.Client
		name      string
		namespace string
	}
	type want struct {
		result *corev1.Secret
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldGetExistingSecret": {
			args: args{
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("specific-secret-name", "specific-secret-namespace", "specific-key", "specific-value")
						return nil
					},
				},
				name:      "specific-secret-name",
				namespace: "specific-secret-namespace",
			},
			want: want{
				result: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "specific-secret-namespace",
						Name:      "specific-secret-name",
					},
					Data: map[string][]byte{
						"specific-key": []byte("specific-value"),
					},
				},
				err: nil,
			},
		},
		"ShouldCreateNewSecret": {
			args: args{
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("new-secret-name", "new-secret-namespace", "new-key", "new-value")
						return nil
					},
					MockCreate: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("new-secret-name", "new-secret-namespace", "new-key", "new-value")
						return nil
					},
				},
				name:      "new-secret-name",
				namespace: "new-secret-namespace",
			},
			want: want{
				result: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "new-secret-namespace",
						Name:      "new-secret-name",
					},
					Data: map[string][]byte{
						"new-key": []byte("new-value"),
					},
				},
				err: nil,
			},
		},

		"ShouldFail": {
			args: args{
				localKube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				name:      "secret",
				namespace: "default",
			},
			want: want{
				result: nil,
				err:    errorspkg.Wrap(errBoom, fmt.Sprintf(errGetSecret, "secret", "default")),
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got, gotErr := GetOrCreateSecret(context.Background(), tc.args.localKube, tc.args.name, tc.args.namespace, nil)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("GetOrCreateSecret(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("GetOrCreateSecret(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_UpdateSecret(t *testing.T) {
	type args struct {
		localKube client.Client
		secret    *corev1.Secret
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldUpdateSecret": {
			args: args{
				localKube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
				secret: createSpecificSecret("update-secret-name", "update-secret-namespace", "update-key", "update-value"),
			},
			want: want{
				err: nil,
			},
		},
		"ShouldFail": {
			args: args{
				localKube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				secret: createSpecificSecret("update-secret-name", "update-secret-namespace", "update-key", "update-value"),
			},
			want: want{
				err: errorspkg.Wrap(errBoom, errUpdateFailed),
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			gotErr := UpdateSecret(context.Background(), tc.args.localKube, tc.args.secret)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("UpdateSecret(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_createSecret(t *testing.T) {
	type args struct {
		localKube client.Client
		name      string
		namespace string
		owner     metav1.Object
	}
	type want struct {
		result *corev1.Secret
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldCreateSecret": {
			args: args{
				localKube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
					MockCreate: test.NewMockCreateFn(nil),
					MockGet:    test.NewMockGetFn(nil),
				},
				name:      "new-secret-name",
				namespace: "new-secret-namespace",
				owner:     nil,
			},
			want: want{
				result: createSpecificSecret("new-secret-name", "new-secret-namespace", "", ""),
				err:    nil,
			},
		},
		"ShouldFailToCreateSecret": {
			args: args{
				localKube: &test.MockClient{
					MockCreate: test.NewMockCreateFn(errBoom),
				},
				name:      "new-secret-name",
				namespace: "new-secret-namespace",
				owner:     nil,
			},
			want: want{
				result: nil,
				err:    errorspkg.Wrap(errBoom, errCreateSecret),
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got, gotErr := createSecret(context.Background(), tc.args.localKube, tc.args.name, tc.args.namespace, tc.args.owner)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("createSecret(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("createSecret(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_hasOwnerReference(t *testing.T) {
	type args struct {
		secret *corev1.Secret
		owner  metav1.Object
	}
	type want struct {
		result bool
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldHaveOwnerReference": {
			args: args{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "owner-uid",
							},
						},
					},
				},
				owner: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID: "owner-uid",
					},
				},
			},
			want: want{
				result: true,
			},
		},
		"ShouldNotHaveOwnerReference": {
			args: args{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "other-owner-uid",
							},
						},
					},
				},
				owner: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID: "owner-uid",
					},
				},
			},
			want: want{
				result: false,
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got := hasOwnerReference(tc.args.secret, tc.args.owner)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("hasOwnerReference(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_GetSecret_ErrorHandling(t *testing.T) {
	// Mock Kubernetes client that always returns an error
	kubeClient := &test.MockClient{
		MockGet: test.NewMockGetFn(errBoom),
	}

	_, err := GetSecret(context.Background(), kubeClient, "some-secret", "some-namespace")

	// Verify that the error returned is wrapped correctly
	if err == nil || !errors.Is(err, errBoom) {
		t.Errorf("GetSecret() expected error %v, got: %v", errBoom, err)
	}
}

func Test_GetOrCreateSecret_EmptyName(t *testing.T) {
	kubeClient := &test.MockClient{
		MockGet: test.NewMockGetFn(errBoom),
	}
	// Pass an empty secret name
	_, err := GetOrCreateSecret(context.Background(), kubeClient, "", "some-namespace", nil)

	// Verify that an error is returned for an empty secret name
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf(errGetSecret, "", "some-namespace")) {
		t.Errorf("GetOrCreateSecret() with empty name: expected error, got: %v", err)
	}
}

func Test_UpdateSecret_ErrorHandling(t *testing.T) {
	// Mock Kubernetes client that always returns an error
	kubeClient := &test.MockClient{
		MockUpdate: test.NewMockUpdateFn(errBoom),
	}

	// Create a dummy secret for testing
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "some-namespace",
			Name:      "some-secret",
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}

	err := UpdateSecret(context.Background(), kubeClient, secret)

	// Verify that the error returned is wrapped correctly
	if err == nil || !errors.Is(err, errBoom) {
		t.Errorf("UpdateSecret() expected error %v, got: %v", errBoom, err)
	}
}
