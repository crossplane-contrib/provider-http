package datapatcher

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createSpecificSecret(name, namespace, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key:         []byte(value),
			"other-key": []byte("otherSecretValue"),
		},
	}
}

func Test_findPlaceholders(t *testing.T) {
	type args struct {
		value string
	}
	type want struct {
		result []string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldFindPlaceholders": {
			args: args{
				value: "data -> {{name:namespace:key}} {{name:namespace:key}} {{name-second:namespace-second:key-second}}",
			},
			want: want{
				result: []string{"{{name:namespace:key}}", "{{name:namespace:key}}", "{{name-second:namespace-second:key-second}}"},
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got := findPlaceholders(tc.args.value)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("findPlaceholders(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_removeDuplicates(t *testing.T) {
	type args struct {
		value []string
	}
	type want struct {
		result []string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldRemoveDuplicates": {
			args: args{
				value: []string{"{{name:namespace:key}}", "{{name:namespace:key}}", "{{name-second:namespace-second:key-second}}"},
			},
			want: want{
				result: []string{"{{name:namespace:key}}", "{{name-second:namespace-second:key-second}}"},
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got := removeDuplicates(tc.args.value)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("removeDuplicates(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_parsePlaceholder(t *testing.T) {
	type args struct {
		placeholder string
	}
	type want struct {
		name      string
		namespace string
		key       string
		ok        bool
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldParsePlaceholder": {
			args: args{
				placeholder: "{{name:namespace:key}}",
			},
			want: want{
				name:      "name",
				namespace: "namespace",
				key:       "key",
				ok:        true,
			},
		},
		"ShouldFailDueToInvalidSyntax": {
			args: args{
				placeholder: "{{name::namespace:key}}",
			},
			want: want{
				name:      "",
				namespace: "",
				key:       "",
				ok:        false,
			},
		},
		"ShouldFailDueToLessArguments": {
			args: args{
				placeholder: "{{name:key}}",
			},
			want: want{
				name:      "",
				namespace: "",
				key:       "",
				ok:        false,
			},
		},
		"ShouldFailDueToMoreArguments": {
			args: args{
				placeholder: "{{name:key:namespace:try}}",
			},
			want: want{
				name:      "",
				namespace: "",
				key:       "",
				ok:        false,
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			name, namespace, key, ok := parsePlaceholder(tc.args.placeholder)
			if diff := cmp.Diff(tc.want.name, name); diff != "" {
				t.Errorf("parsePlaceholder(...): -want name, +got name: %s", diff)
			}
			if diff := cmp.Diff(tc.want.namespace, namespace); diff != "" {
				t.Errorf("parsePlaceholder(...): -want namespace, +got namespace: %s", diff)
			}
			if diff := cmp.Diff(tc.want.key, key); diff != "" {
				t.Errorf("parsePlaceholder(...): -want key, +got key: %s", diff)
			}
			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Errorf("parsePlaceholder(...): -want ok, +got ok: %s", diff)
			}
		})
	}
}

func Test_replacePlaceholderWithSecretValue(t *testing.T) {
	type args struct {
		originalString string
		old            string
		secret         *corev1.Secret
		key            string
	}
	type want struct {
		result string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldGetSecret": {
			args: args{
				originalString: "this is the test string",
				old:            "test",
				secret: &corev1.Secret{
					Data: map[string][]byte{
						"key": []byte("changed"),
					},
				},
				key: "key",
			},
			want: want{
				result: "this is the changed string",
			},
		},
	}
	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got := replacePlaceholderWithSecretValue(tc.args.originalString, tc.args.old, tc.args.secret, tc.args.key)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("replacePlaceholderWithSecretValue(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_patchSecretsToValue(t *testing.T) {
	placeholderData := "{{name:namespace:key}}"

	type args struct {
		valueToHandle string
		localKube     client.Client
	}

	type want struct {
		result string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ShouldPatchSingleSecret": {
			args: args{
				valueToHandle: "data -> " + placeholderData,
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
			},
			want: want{
				result: "data -> value",
			},
		},
		"ShouldPatchMultipleSecrets": {
			args: args{
				valueToHandle: "data -> " + placeholderData + " " + placeholderData,
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
			},
			want: want{
				result: "data -> value value",
			},
		},
		"ShouldNotPatchInvalidPlaceholder": {
			args: args{
				valueToHandle: "data -> {{invalid-placeholder}}",
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
			},
			want: want{
				result: "data -> {{invalid-placeholder}}",
			},
		},
	}

	for name, tc := range cases {
		tc := tc // Create local copies of loop variables

		t.Run(name, func(t *testing.T) {
			got, err := patchSecretsToValue(context.Background(), tc.args.localKube, tc.args.valueToHandle, logging.NewNopLogger())
			if err != nil {
				t.Fatalf("patchSecretsToValue(...): unexpected error: %v", err)
			}
			if got != tc.want.result {
				t.Errorf("patchSecretsToValue(...): want result %q, got %q", tc.want.result, got)
			}
		})
	}
}
