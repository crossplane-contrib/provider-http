package datapatcher

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPatchSecretsIntoBody(t *testing.T) {
	type args struct {
		ctx       context.Context
		localKube client.Client
		body      string
	}

	type want struct {
		result string
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldPatchSecretsIntoBody": {
			args: args{
				ctx: context.Background(),
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("name", "namespace", "key", "value")
						return nil
					},
				},
				body: "body with secrets: {{name:namespace:key}}",
			},
			want: want{
				result: "body with secrets: value",
				err:    nil,
			},
		},
		"ShouldNotPatchInvalidPlaceholder": {
			args: args{
				ctx: context.Background(),
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("name", "namespace", "key", "value")
						return nil
					},
				},
				body: "body with invalid placeholder: {{invalid-placeholder}}",
			},
			want: want{
				result: "body with invalid placeholder: {{invalid-placeholder}}",
				err:    nil,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got, gotErr := PatchSecretsIntoBody(tc.args.ctx, tc.args.localKube, tc.args.body)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("isUpToDate(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("isUpToDate(...): -want result, +got result: %s", diff)
			}
		})
	}
}
func TestPatchSecretsIntoHeaders(t *testing.T) {
	type args struct {
		ctx       context.Context
		localKube client.Client
		headers   map[string][]string
	}

	type want struct {
		result map[string][]string
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldPatchSingleHeader": {
			args: args{
				ctx: context.Background(),
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("name", "namespace", "key", "value")
						return nil
					},
				},
				headers: map[string][]string{
					"Authorization": {"Bearer {{name:namespace:key}}"},
				},
			},
			want: want{
				result: map[string][]string{
					"Authorization": {"Bearer value"},
				},
				err: nil,
			},
		},
		"ShouldPatchMultipleHeaders": {
			args: args{
				ctx: context.Background(),
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("name", "namespace", "key", "value")
						return nil
					},
				},
				headers: map[string][]string{
					"Authorization": {"Bearer {{name:namespace:key}}"},
					"X-Secret":      {"{{name:namespace:other-key}}"},
				},
			},
			want: want{
				result: map[string][]string{
					"Authorization": {"Bearer value"},
					"X-Secret":      {"otherSecretValue"},
				},
				err: nil,
			},
		},
		"ShouldNotPatchInvalidPlaceholder": {
			args: args{
				ctx: context.Background(),
				localKube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						secret, ok := obj.(*corev1.Secret)
						if !ok {
							return errors.New("object is not a Secret")
						}

						*secret = *createSpecificSecret("name", "namespace", "key", "value")
						return nil
					},
				},
				headers: map[string][]string{
					"Authorization": {"Bearer {{invalid-placeholder}}"},
				},
			},
			want: want{
				result: map[string][]string{
					"Authorization": {"Bearer {{invalid-placeholder}}"},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got, gotErr := PatchSecretsIntoHeaders(tc.args.ctx, tc.args.localKube, tc.args.headers)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("isUpToDate(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("isUpToDate(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func TestPatchResponseToSecret(t *testing.T) {
	type args struct {
		ctx             context.Context
		localKube       client.Client
		logger          logging.Logger
		data            interface{}
		path            string
		secretKey       string
		secretName      string
		secretNamespace string
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		// Add test cases
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			gotErr := PatchResponseToSecret(tc.args.ctx, tc.args.localKube, tc.args.logger, tc.args.data, tc.args.path, tc.args.secretKey, tc.args.secretName, tc.args.secretNamespace)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("isUpToDate(...): -want error, +got error: %s", diff)
			}
		})
	}
}
