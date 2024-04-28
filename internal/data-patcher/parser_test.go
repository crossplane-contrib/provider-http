package datapatcher

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

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
