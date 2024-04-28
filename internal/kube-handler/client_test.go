package kubehandler

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errBoom = errors.New("boom")
)

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
					MockGet: test.NewMockGetFn(nil),
				},
				name:      "",
				namespace: "",
			},
			want: want{
				result: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "",
						Name:      "",
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
				result: &corev1.Secret{},
				err:    errBoom,
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
