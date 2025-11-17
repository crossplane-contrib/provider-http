package v1alpha2

import (
	"testing"
	"time"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDisposableRequestParameters_Accessors(t *testing.T) {
	timeout := &metav1.Duration{Duration: 5 * time.Minute}
	nextReconcile := &metav1.Duration{Duration: 10 * time.Minute}
	rollbackLimit := int32(3)

	params := &DisposableRequestParameters{
		URL:                   "https://api.example.com/resource",
		Method:                "POST",
		Body:                  `{"key":"value"}`,
		Headers:               map[string][]string{"Content-Type": {"application/json"}},
		WaitTimeout:           timeout,
		InsecureSkipTLSVerify: true,
		ExpectedResponse:      ".status == 'success'",
		NextReconcile:         nextReconcile,
		ShouldLoopInfinitely:  true,
		RollbackRetriesLimit:  &rollbackLimit,
		SecretInjectionConfigs: []common.SecretInjectionConfig{
			{
				SecretRef: common.SecretRef{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
		},
	}

	if got := params.GetURL(); got != "https://api.example.com/resource" {
		t.Errorf("GetURL() = %v, want https://api.example.com/resource", got)
	}

	if got := params.GetMethod(); got != "POST" {
		t.Errorf("GetMethod() = %v, want POST", got)
	}

	if got := params.GetBody(); got != `{"key":"value"}` {
		t.Errorf("GetBody() = %v, want {\"key\":\"value\"}", got)
	}

	if got := params.GetHeaders(); !cmp.Equal(got, params.Headers) {
		t.Errorf("GetHeaders() mismatch: %v", cmp.Diff(params.Headers, got))
	}

	if got := params.GetWaitTimeout(); got != timeout {
		t.Errorf("GetWaitTimeout() = %v, want %v", got, timeout)
	}

	if got := params.GetInsecureSkipTLSVerify(); got != true {
		t.Errorf("GetInsecureSkipTLSVerify() = %v, want true", got)
	}

	if got := params.GetExpectedResponse(); got != ".status == 'success'" {
		t.Errorf("GetExpectedResponse() = %v, want .status == 'success'", got)
	}

	if got := params.GetNextReconcile(); got != nextReconcile {
		t.Errorf("GetNextReconcile() = %v, want %v", got, nextReconcile)
	}

	if got := params.GetShouldLoopInfinitely(); got != true {
		t.Errorf("GetShouldLoopInfinitely() = %v, want true", got)
	}

	if got := params.GetRollbackRetriesLimit(); *got != rollbackLimit {
		t.Errorf("GetRollbackRetriesLimit() = %v, want %v", *got, rollbackLimit)
	}

	if got := params.GetSecretInjectionConfigs(); len(got) != 1 {
		t.Errorf("GetSecretInjectionConfigs() length = %v, want 1", len(got))
	}
}

func TestDisposableResponse_Accessors(t *testing.T) {
	response := &Response{
		StatusCode: 200,
		Body:       `{"result":"success"}`,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}

	if got := response.GetStatusCode(); got != 200 {
		t.Errorf("GetStatusCode() = %v, want 200", got)
	}

	if got := response.GetBody(); got != `{"result":"success"}` {
		t.Errorf("GetBody() = %v, want {\"result\":\"success\"}", got)
	}

	if got := response.GetHeaders(); len(got) != 1 {
		t.Errorf("GetHeaders() length = %v, want 1", len(got))
	}
}

func TestDisposableRequest_CachedResponse(t *testing.T) {
	// Test with cached response
	req := &DisposableRequest{
		Status: DisposableRequestStatus{
			Response: Response{
				StatusCode: 200,
				Body:       "cached",
			},
		},
	}

	cached := req.GetCachedResponse()
	if cached == nil {
		t.Error("GetCachedResponse() returned nil for valid response")
	}
	if cached.GetStatusCode() != 200 {
		t.Errorf("Cached response StatusCode = %v, want 200", cached.GetStatusCode())
	}

	// Test with no cached response
	req2 := &DisposableRequest{
		Status: DisposableRequestStatus{
			Response: Response{
				StatusCode: 0,
			},
		},
	}

	cached2 := req2.GetCachedResponse()
	if cached2 != nil {
		t.Error("GetCachedResponse() should return nil when StatusCode is 0")
	}
}

func TestDisposableRequest_StatusAccessors(t *testing.T) {
	req := &DisposableRequest{
		Status: DisposableRequestStatus{
			Synced: true,
			Failed: 2,
			Response: Response{
				StatusCode: 201,
				Body:       "test response",
			},
		},
	}

	if got := req.GetSynced(); got != true {
		t.Errorf("GetSynced() = %v, want true", got)
	}

	if got := req.GetFailed(); got != 2 {
		t.Errorf("GetFailed() = %v, want 2", got)
	}

	resp := req.GetResponse()
	if resp == nil {
		t.Error("GetResponse() returned nil")
	}
	if resp.GetStatusCode() != 201 {
		t.Errorf("Response StatusCode = %v, want 201", resp.GetStatusCode())
	}

	// Test SetFailed
	req.SetFailed(5)
	if got := req.GetFailed(); got != 5 {
		t.Errorf("After SetFailed(5), GetFailed() = %v, want 5", got)
	}
}