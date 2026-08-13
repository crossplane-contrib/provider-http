package requestschedule

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	requestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
)

func TestRateLimitDeadline(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	obj := &requestv1alpha2.Request{ObjectMeta: metav1.ObjectMeta{Name: "example", UID: types.UID("stable")}}
	tests := []struct {
		name       string
		headers    map[string][]string
		failures   int32
		wantSource RateLimitSource
		minimum    time.Time
		maximum    time.Time
	}{
		{
			name: "RetryAfterDeltaTakesPrecedence",
			headers: map[string][]string{
				"retry-after":        {"120"},
				"X-Rate-Limit-Reset": {strconv.FormatInt(now.Add(10*time.Minute).Unix(), 10)},
			},
			wantSource: RateLimitSourceRetryAfter,
			minimum:    now.Add(2 * time.Minute),
			maximum:    now.Add(2*time.Minute + resetSafetyWindow),
		},
		{
			name:       "RetryAfterHTTPDate",
			headers:    map[string][]string{"Retry-After": {now.Add(5 * time.Minute).Format(http.TimeFormat)}},
			wantSource: RateLimitSourceRetryAfter,
			minimum:    now.Add(5 * time.Minute),
			maximum:    now.Add(5*time.Minute + resetSafetyWindow),
		},
		{
			name:       "ResetFallback",
			headers:    map[string][]string{"Retry-After": {"invalid"}, "x-rate-limit-reset": {strconv.FormatInt(now.Add(10*time.Minute).Unix(), 10)}},
			wantSource: RateLimitSourceReset,
			minimum:    now.Add(10 * time.Minute),
			maximum:    now.Add(10*time.Minute + resetSafetyWindow),
		},
		{
			name:       "MalformedFallback",
			headers:    map[string][]string{"Retry-After": {"invalid"}, "X-Rate-Limit-Reset": {"invalid"}},
			failures:   3,
			wantSource: RateLimitSourceFallback,
			minimum:    now.Add(4 * time.Minute),
			maximum:    now.Add(4 * time.Minute),
		},
		{
			name:       "PastFallback",
			headers:    map[string][]string{"X-Rate-Limit-Reset": {strconv.FormatInt(now.Add(-time.Minute).Unix(), 10)}},
			wantSource: RateLimitSourceFallback,
			minimum:    now.Add(time.Minute),
			maximum:    now.Add(time.Minute),
		},
		{
			name:       "ExcessiveFallback",
			headers:    map[string][]string{"Retry-After": {strconv.FormatInt(int64((25*time.Hour)/time.Second), 10)}},
			wantSource: RateLimitSourceFallback,
			minimum:    now.Add(time.Minute),
			maximum:    now.Add(time.Minute),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, source := RateLimitDeadline(now, tt.headers, tt.failures, obj)
			if source != tt.wantSource {
				t.Fatalf("source = %q, want %q", source, tt.wantSource)
			}
			if got.Before(tt.minimum) || got.After(tt.maximum) {
				t.Fatalf("deadline %s outside [%s,%s]", got, tt.minimum, tt.maximum)
			}
		})
	}
}

func TestRateLimitSafetyOffsetIsStable(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(time.Minute)
	obj := &requestv1alpha2.Request{ObjectMeta: metav1.ObjectMeta{Name: "example", UID: types.UID("stable")}}
	first := rateLimitSafetyOffset(obj, deadline)
	second := rateLimitSafetyOffset(obj, deadline)
	if first != second {
		t.Fatalf("offset changed: %s != %s", first, second)
	}
	if first < 0 || first > resetSafetyWindow {
		t.Fatalf("offset %s outside [0,%s]", first, resetSafetyWindow)
	}
}

func TestFallbackBackoff(t *testing.T) {
	tests := []struct {
		failures int32
		want     time.Duration
	}{
		{failures: 0, want: time.Minute},
		{failures: 1, want: time.Minute},
		{failures: 2, want: 2 * time.Minute},
		{failures: 3, want: 4 * time.Minute},
		{failures: 7, want: time.Hour},
		{failures: 100, want: time.Hour},
	}
	for _, tt := range tests {
		if got := FallbackBackoff(tt.failures); got != tt.want {
			t.Errorf("FallbackBackoff(%d) = %s, want %s", tt.failures, got, tt.want)
		}
	}
}
