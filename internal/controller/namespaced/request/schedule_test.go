package request

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane-contrib/provider-http/apis/namespaced/request/v1alpha2"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/requestschedule"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

func TestCustomPollIntervalHook(t *testing.T) {
	fallback := 15 * time.Minute
	now := time.Now()
	futurePoll := metav1.NewTime(now.Add(2 * time.Hour))
	futureLimit := metav1.NewTime(now.Add(time.Hour))
	tests := []struct {
		name    string
		request *v1alpha2.Request
		minimum time.Duration
		maximum time.Duration
	}{
		{name: "GlobalFallback", request: &v1alpha2.Request{}, minimum: fallback, maximum: fallback},
		{name: "ConfiguredFallback", request: &v1alpha2.Request{Spec: v1alpha2.RequestSpec{ForProvider: v1alpha2.RequestParameters{PollInterval: &metav1.Duration{Duration: 24 * time.Hour}}}}, minimum: 24 * time.Hour, maximum: 24 * time.Hour},
		{name: "PersistedPoll", request: &v1alpha2.Request{Status: v1alpha2.RequestStatus{NextPollTime: &futurePoll}}, minimum: 2*time.Hour - time.Second, maximum: 2 * time.Hour},
		{name: "RateLimitPrecedence", request: &v1alpha2.Request{Status: v1alpha2.RequestStatus{NextPollTime: &futurePoll, RateLimitUntil: &futureLimit}}, minimum: time.Hour - time.Second, maximum: time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := customPollIntervalHook(tt.request, fallback)
			if got < tt.minimum || got > tt.maximum {
				t.Fatalf("interval %s outside [%s,%s]", got, tt.minimum, tt.maximum)
			}
		})
	}
}

func TestObserveSkipsHTTPBeforePersistedPoll(t *testing.T) {
	req := httpNamespacedRequest()
	hash, err := requestschedule.DesiredStateHash(req)
	if err != nil {
		t.Fatal(err)
	}
	next := metav1.NewTime(time.Now().Add(time.Hour))
	req.Status.ObservedDesiredStateHash = hash
	req.Status.NextPollTime = &next

	e := &external{
		logger: logging.NewNopLogger(),
		http: &MockHttpClient{MockSendRequest: func(_ context.Context, _, _ string, _, _ httpClient.Data, _ *httpClient.TLSConfigData) (httpClient.HttpDetails, error) {
			panic("HTTP must not be called before persisted poll")
		}},
	}
	got, err := e.Observe(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ResourceExists || !got.ResourceUpToDate {
		t.Fatalf("observation = %+v, want existing and up to date", got)
	}
}
