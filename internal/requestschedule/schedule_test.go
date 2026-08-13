package requestschedule

import (
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	requestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
)

func TestEffectiveInterval(t *testing.T) {
	fallback := 10 * time.Minute
	tests := []struct {
		name string
		spec *requestv1alpha2.RequestParameters
		want time.Duration
	}{
		{name: "GlobalFallback", spec: &requestv1alpha2.RequestParameters{}, want: fallback},
		{name: "Configured", spec: &requestv1alpha2.RequestParameters{PollInterval: duration(24 * time.Hour)}, want: 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveInterval(tt.spec, fallback); got != tt.want {
				t.Fatalf("EffectiveInterval() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestJitterOffset(t *testing.T) {
	obj := &requestv1alpha2.Request{ObjectMeta: metav1.ObjectMeta{Name: "example", UID: types.UID("stable-uid")}}
	interval := 24 * time.Hour
	window := time.Hour
	first := JitterOffset(obj, interval, window)
	second := JitterOffset(obj, interval, window)
	if first != second {
		t.Fatalf("offset changed: %s != %s", first, second)
	}
	if first < 0 || first > window {
		t.Fatalf("offset %s outside [0,%s]", first, window)
	}
	if got := JitterOffset(obj, interval, 0); got != 0 {
		t.Fatalf("zero-window offset = %s, want 0", got)
	}

	other := obj.DeepCopy()
	other.UID = types.UID("different-uid")
	if got := JitterOffset(other, interval, window); got == first {
		t.Fatalf("different identity unexpectedly produced same offset %s", got)
	}
}

func TestDesiredStateHash(t *testing.T) {
	obj := &requestv1alpha2.Request{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "example",
			Labels:      map[string]string{"team": "platform"},
			Annotations: map[string]string{"example.org/reconcile-at": "one"},
		},
		Spec: requestv1alpha2.RequestSpec{ForProvider: requestv1alpha2.RequestParameters{
			Mappings: []requestv1alpha2.Mapping{{Method: "GET", URL: ".payload.baseUrl"}},
			Payload:  requestv1alpha2.Payload{BaseUrl: "https://example.org"},
		}},
	}
	first := mustHash(t, obj)
	if got := mustHash(t, obj.DeepCopy()); got != first {
		t.Fatalf("unchanged hash = %s, want %s", got, first)
	}

	changed := obj.DeepCopy()
	changed.Annotations["example.org/reconcile-at"] = "two"
	if got := mustHash(t, changed); got == first {
		t.Fatal("annotation change did not change hash")
	}

	ignored := obj.DeepCopy()
	ignored.Annotations[meta.AnnotationKeyExternalCreatePending] = "pending"
	if got := mustHash(t, ignored); got != first {
		t.Fatalf("ignored annotation changed hash: %s != %s", got, first)
	}

	statusOnly := obj.DeepCopy()
	statusOnly.Status.Error = "changed"
	if got := mustHash(t, statusOnly); got != first {
		t.Fatalf("status changed hash: %s != %s", got, first)
	}
}

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	future := metav1.NewTime(now.Add(time.Hour))
	past := metav1.NewTime(now.Add(-time.Hour))
	tests := []struct {
		name   string
		status requestv1alpha2.RequestStatus
		hash   string
		allow  bool
		wakeAt *time.Time
	}{
		{name: "FirstObservation", hash: "new", allow: true},
		{name: "ChangedDesiredState", status: requestv1alpha2.RequestStatus{ObservedDesiredStateHash: "old", NextPollTime: &future}, hash: "new", allow: true},
		{name: "FuturePoll", status: requestv1alpha2.RequestStatus{ObservedDesiredStateHash: "same", NextPollTime: &future}, hash: "same", wakeAt: &future.Time},
		{name: "DuePoll", status: requestv1alpha2.RequestStatus{ObservedDesiredStateHash: "same", NextPollTime: &past}, hash: "same", allow: true},
		{name: "RateLimitedChangedState", status: requestv1alpha2.RequestStatus{ObservedDesiredStateHash: "old", RateLimitUntil: &future}, hash: "new", wakeAt: &future.Time},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &requestv1alpha2.Request{Status: tt.status}
			got := Evaluate(now, tt.hash, req)
			if got.AllowHTTP != tt.allow {
				t.Fatalf("AllowHTTP = %v, want %v", got.AllowHTTP, tt.allow)
			}
			if !sameTime(got.WakeAt, tt.wakeAt) {
				t.Fatalf("WakeAt = %v, want %v", got.WakeAt, tt.wakeAt)
			}
		})
	}
}

func TestEveryDueRequestIsEligibleRegardlessOfOrder(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	past := metav1.NewTime(now.Add(-time.Minute))
	const requestCount = 100

	for index := requestCount - 1; index >= 0; index-- {
		req := &requestv1alpha2.Request{Status: requestv1alpha2.RequestStatus{
			ObservedDesiredStateHash: "same",
			NextPollTime:             &past,
		}}
		decision := Evaluate(now, "same", req)
		if !decision.AllowHTTP {
			t.Fatalf("due request at ordered position %d was not eligible", index)
		}
	}
}

func TestJitterDistributesFleet(t *testing.T) {
	interval := 24 * time.Hour
	window := time.Hour
	offsets := map[time.Duration]struct{}{}
	for index := 0; index < 100; index++ {
		obj := &requestv1alpha2.Request{ObjectMeta: metav1.ObjectMeta{
			Name: "example",
			UID:  types.UID(time.Unix(int64(index), 0).Format(time.RFC3339Nano)),
		}}
		offsets[JitterOffset(obj, interval, window)] = struct{}{}
	}
	if len(offsets) < 90 {
		t.Fatalf("100 resources produced only %d distinct jitter offsets", len(offsets))
	}
}

func TestNextPollTime(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	obj := &requestv1alpha2.Request{ObjectMeta: metav1.ObjectMeta{Name: "example", UID: types.UID("stable")}}
	spec := &requestv1alpha2.RequestParameters{PollInterval: duration(24 * time.Hour), PollJitter: duration(time.Hour)}
	got := NextPollTime(now, obj, spec, time.Minute)
	minimum := now.Add(24 * time.Hour)
	maximum := minimum.Add(time.Hour)
	if got.Before(minimum) || got.After(maximum) {
		t.Fatalf("next poll %s outside [%s,%s]", got, minimum, maximum)
	}
}

func duration(value time.Duration) *metav1.Duration {
	return &metav1.Duration{Duration: value}
}

func mustHash(t *testing.T, obj *requestv1alpha2.Request) string {
	t.Helper()
	value, err := DesiredStateHash(obj)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func sameTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}
