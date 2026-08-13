// Package requestschedule provides durable scheduling decisions shared by Request controllers.
package requestschedule

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
)

// Decision describes whether an external request is allowed and when to reconcile again.
type Decision struct {
	AllowHTTP bool
	WakeAt    *time.Time
}

// EffectiveInterval returns a Request's configured interval or the provider default.
func EffectiveInterval(spec interfaces.MappedHTTPRequestSpec, defaultInterval time.Duration) time.Duration {
	if scheduling, ok := spec.(interfaces.RequestSchedulingAware); ok {
		if configured := scheduling.GetPollInterval(); configured != nil && configured.Duration > 0 {
			return configured.Duration
		}
	}
	return defaultInterval
}

// PollJitter returns the configured jitter window, or zero when absent or invalid.
func PollJitter(spec interfaces.MappedHTTPRequestSpec) time.Duration {
	if scheduling, ok := spec.(interfaces.RequestSchedulingAware); ok {
		if configured := scheduling.GetPollJitter(); configured != nil && configured.Duration > 0 {
			return configured.Duration
		}
	}
	return 0
}

// JitterOffset deterministically maps resource identity and schedule into [0, window].
func JitterOffset(obj metav1.Object, interval, window time.Duration) time.Duration {
	if window <= 0 {
		return 0
	}
	identity := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%d",
		obj.GetNamespace(), obj.GetName(), obj.GetUID(), obj.GetGenerateName(), interval, window)
	digest := sha256.Sum256([]byte(identity))
	value := binary.BigEndian.Uint64(digest[:8])
	// window is a non-negative time.Duration, so converting after reducing by
	// its int64-bounded modulus is safe.
	return time.Duration(value % uint64(window+1)) //nolint:gosec
}

// DesiredStateHash hashes the fields observed by resource.DesiredStateChanged.
func DesiredStateHash(obj client.Object) (string, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	var resource map[string]any
	if err := json.Unmarshal(raw, &resource); err != nil {
		return "", fmt.Errorf("decode request: %w", err)
	}

	annotations := copyStringMap(obj.GetAnnotations())
	delete(annotations, meta.AnnotationKeyExternalCreateFailed)
	delete(annotations, meta.AnnotationKeyExternalCreatePending)
	desired := struct {
		Spec        any               `json:"spec"`
		Labels      map[string]string `json:"labels,omitempty"`
		Annotations map[string]string `json:"annotations,omitempty"`
	}{
		Spec:        resource["spec"],
		Labels:      copyStringMap(obj.GetLabels()),
		Annotations: annotations,
	}
	canonical, err := json.Marshal(desired)
	if err != nil {
		return "", fmt.Errorf("marshal desired state: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

// Evaluate determines whether HTTP is allowed for the current desired state.
func Evaluate(now time.Time, desiredHash string, status interfaces.RequestStatusReader) Decision {
	if until := status.GetRateLimitUntil(); until != nil && now.Before(until.Time) {
		return Decision{WakeAt: timePointer(until.Time)}
	}
	if status.GetObservedDesiredStateHash() == "" || status.GetObservedDesiredStateHash() != desiredHash {
		return Decision{AllowHTTP: true}
	}
	if next := status.GetNextPollTime(); next != nil && now.Before(next.Time) {
		return Decision{WakeAt: timePointer(next.Time)}
	}
	return Decision{AllowHTTP: true}
}

// NextPollTime computes the next periodic deadline after a successful observation.
func NextPollTime(now time.Time, obj metav1.Object, spec interfaces.MappedHTTPRequestSpec, defaultInterval time.Duration) time.Time {
	interval := EffectiveInterval(spec, defaultInterval)
	return now.Add(interval + JitterOffset(obj, interval, PollJitter(spec)))
}

func copyStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func timePointer(value time.Time) *time.Time {
	return &value
}
