package disposablerequest

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha2 "github.com/crossplane-contrib/provider-http/apis/namespaced/disposablerequest/v1alpha2"
)

func TestCustomPollIntervalHook_Namespaced(t *testing.T) {
	defaultPoll := 30 * time.Second

	now := time.Now()

	cases := map[string]struct {
		cr   *v1alpha2.DisposableRequest
		want time.Duration
	}{
		"nil nextReconcile": {
			cr:   &v1alpha2.DisposableRequest{},
			want: defaultPoll,
		},
		"zero lastReconcile + nextReconcile set": {
			cr: &v1alpha2.DisposableRequest{
				Spec: v1alpha2.DisposableRequestSpec{
					ForProvider: v1alpha2.DisposableRequestParameters{
						NextReconcile: &metav1.Duration{Duration: time.Hour},
					},
				},
			},
			want: time.Hour,
		},
		"past lastReconcile with remaining time": {
			cr: &v1alpha2.DisposableRequest{
				Spec: v1alpha2.DisposableRequestSpec{
					ForProvider: v1alpha2.DisposableRequestParameters{
						NextReconcile: &metav1.Duration{Duration: time.Hour},
					},
				},
				Status: v1alpha2.DisposableRequestStatus{
					LastReconcileTime: metav1.NewTime(now.Add(-30 * time.Minute)),
				},
			},
			want: 30 * time.Minute,
		},
		"nextReconcile already passed": {
			cr: &v1alpha2.DisposableRequest{
				Spec: v1alpha2.DisposableRequestSpec{
					ForProvider: v1alpha2.DisposableRequestParameters{
						NextReconcile: &metav1.Duration{Duration: time.Second},
					},
				},
				Status: v1alpha2.DisposableRequestStatus{
					LastReconcileTime: metav1.NewTime(now.Add(-time.Hour)),
				},
			},
			want: defaultPoll,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := customPollIntervalHook(tc.cr, 0)
			// Allow a 2-second tolerance for the durations that are computed from now
			if diff := got - tc.want; diff > 2*time.Second || diff < -2*time.Second {
				t.Fatalf("%s: want ~%v, got %v", name, tc.want, got)
			}
		})
	}
}
