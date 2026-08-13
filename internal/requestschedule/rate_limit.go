package requestschedule

import (
	"crypto/sha256"
	"encoding/binary"
	"net/http"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	fallbackBackoffMinimum = time.Minute
	fallbackBackoffMaximum = time.Hour
	maximumServerDeferral  = 24 * time.Hour
	resetSafetyWindow      = 5 * time.Second
)

// RateLimitSource identifies how an HTTP 429 deferral deadline was selected.
type RateLimitSource string

const (
	RateLimitSourceRetryAfter RateLimitSource = "Retry-After"
	RateLimitSourceReset      RateLimitSource = "X-Rate-Limit-Reset"
	RateLimitSourceFallback   RateLimitSource = "Fallback"
)

// RateLimitDeadline returns the next permitted request time for an HTTP 429.
func RateLimitDeadline(now time.Time, headers map[string][]string, failures int32, obj metav1.Object) (time.Time, RateLimitSource) {
	if deadline, ok := parseRetryAfter(now, headerValue(headers, "Retry-After")); ok {
		return deadline.Add(rateLimitSafetyOffset(obj, deadline)), RateLimitSourceRetryAfter
	}
	if deadline, ok := parseRateLimitReset(now, headerValue(headers, "X-Rate-Limit-Reset")); ok {
		return deadline.Add(rateLimitSafetyOffset(obj, deadline)), RateLimitSourceReset
	}
	return now.Add(FallbackBackoff(failures)), RateLimitSourceFallback
}

// FallbackBackoff returns exponential backoff from one minute capped at one hour.
func FallbackBackoff(failures int32) time.Duration {
	if failures <= 1 {
		return fallbackBackoffMinimum
	}
	backoff := fallbackBackoffMinimum
	for attempt := int32(1); attempt < failures && backoff < fallbackBackoffMaximum; attempt++ {
		backoff *= 2
		if backoff >= fallbackBackoffMaximum {
			return fallbackBackoffMaximum
		}
	}
	return backoff
}

func parseRetryAfter(now time.Time, value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
		if seconds <= 0 {
			return time.Time{}, false
		}
		return validateServerDeadline(now, now.Add(time.Duration(seconds)*time.Second))
	}
	parsed, err := http.ParseTime(value)
	if err != nil {
		return time.Time{}, false
	}
	return validateServerDeadline(now, parsed)
}

func parseRateLimitReset(now time.Time, value string) (time.Time, bool) {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	return validateServerDeadline(now, time.Unix(seconds, 0))
}

func validateServerDeadline(now, deadline time.Time) (time.Time, bool) {
	if !deadline.After(now) || deadline.Sub(now) > maximumServerDeferral {
		return time.Time{}, false
	}
	return deadline, true
}

func headerValue(headers map[string][]string, name string) string {
	for key, values := range headers {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func rateLimitSafetyOffset(obj metav1.Object, deadline time.Time) time.Duration {
	identity := obj.GetNamespace() + "\x00" + obj.GetName() + "\x00" + string(obj.GetUID()) + "\x00" + deadline.UTC().Format(time.RFC3339Nano)
	digest := sha256.Sum256([]byte(identity))
	return time.Duration(binary.BigEndian.Uint64(digest[:8]) % (uint64(resetSafetyWindow) + 1))
}
