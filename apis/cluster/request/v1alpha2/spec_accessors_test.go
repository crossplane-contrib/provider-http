package v1alpha2

import (
	"testing"
	"time"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRequestParameters_Accessors(t *testing.T) {
	timeout := &metav1.Duration{Duration: 5 * time.Minute}
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	secretConfigs := []common.SecretInjectionConfig{
		{
			SecretRef: common.SecretRef{
				Name:      "test-secret",
				Namespace: "default",
			},
		},
	}

	params := &RequestParameters{
		WaitTimeout:            timeout,
		InsecureSkipTLSVerify:  true,
		Headers:                headers,
		SecretInjectionConfigs: secretConfigs,
		Payload: Payload{
			BaseUrl: "https://api.example.com",
			Body:    `{"key":"value"}`,
		},
		Mappings: []Mapping{
			{
				Method: "POST",
				URL:    ".payload.baseUrl",
				Body:   ".payload.body",
			},
		},
		ExpectedResponseCheck: ExpectedResponseCheck{
			Type:  "jq",
			Logic: ".status == 'success'",
		},
		IsRemovedCheck: ExpectedResponseCheck{
			Type:  "statusCode",
			Logic: "404",
		},
	}

	if got := params.GetWaitTimeout(); got != timeout {
		t.Errorf("GetWaitTimeout() = %v, want %v", got, timeout)
	}

	if got := params.GetInsecureSkipTLSVerify(); got != true {
		t.Errorf("GetInsecureSkipTLSVerify() = %v, want true", got)
	}

	if got := params.GetHeaders(); !cmp.Equal(got, headers) {
		t.Errorf("GetHeaders() mismatch: %v", cmp.Diff(headers, got))
	}

	if got := params.GetSecretInjectionConfigs(); len(got) != 1 {
		t.Errorf("GetSecretInjectionConfigs() length = %v, want 1", len(got))
	}

	if got := params.GetPayload(); got == nil {
		t.Error("GetPayload() returned nil")
	}

	if got := params.GetMappings(); len(got) != 1 {
		t.Errorf("GetMappings() length = %v, want 1", len(got))
	}

	if got := params.GetExpectedResponseCheck(); got == nil {
		t.Error("GetExpectedResponseCheck() returned nil")
	}

	if got := params.GetIsRemovedCheck(); got == nil {
		t.Error("GetIsRemovedCheck() returned nil")
	}
}

func TestMapping_Accessors(t *testing.T) {
	mapping := &Mapping{
		Method: "POST",
		Action: "create",
		Body:   `{"key":"value"}`,
		URL:    "https://api.example.com/resource",
		Headers: map[string][]string{
			"Authorization": {"Bearer token"},
		},
	}

	if got := mapping.GetMethod(); got != "POST" {
		t.Errorf("GetMethod() = %v, want POST", got)
	}

	if got := mapping.GetAction(); got != "create" {
		t.Errorf("GetAction() = %v, want create", got)
	}

	if got := mapping.GetBody(); got != `{"key":"value"}` {
		t.Errorf("GetBody() = %v, want {\"key\":\"value\"}", got)
	}

	if got := mapping.GetURL(); got != "https://api.example.com/resource" {
		t.Errorf("GetURL() = %v, want https://api.example.com/resource", got)
	}

	if got := mapping.GetHeaders(); len(got) != 1 {
		t.Errorf("GetHeaders() length = %v, want 1", len(got))
	}

	// Test SetMethod
	mapping.SetMethod("GET")
	if got := mapping.GetMethod(); got != "GET" {
		t.Errorf("After SetMethod(GET), GetMethod() = %v, want GET", got)
	}
}

func TestPayload_Accessors(t *testing.T) {
	payload := &Payload{
		BaseUrl: "https://api.example.com",
		Body:    `{"data":"test"}`,
	}

	if got := payload.GetBaseURL(); got != "https://api.example.com" {
		t.Errorf("GetBaseURL() = %v, want https://api.example.com", got)
	}

	if got := payload.GetBody(); got != `{"data":"test"}` {
		t.Errorf("GetBody() = %v, want {\"data\":\"test\"}", got)
	}
}

func TestExpectedResponseCheck_Accessors(t *testing.T) {
	check := &ExpectedResponseCheck{
		Type:  "jq",
		Logic: ".status == 'ok'",
	}

	if got := check.GetType(); got != "jq" {
		t.Errorf("GetType() = %v, want jq", got)
	}

	if got := check.GetLogic(); got != ".status == 'ok'" {
		t.Errorf("GetLogic() = %v, want .status == 'ok'", got)
	}
}

func TestResponse_Accessors(t *testing.T) {
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

func TestRequest_CachedResponse(t *testing.T) {
	// Test with cached response
	req := &Request{
		Status: RequestStatus{
			Cache: Cache{
				Response: Response{
					StatusCode: 200,
					Body:       "cached",
				},
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
	req2 := &Request{
		Status: RequestStatus{
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

func TestRequest_StatusReader(t *testing.T) {
	req := &Request{
		Status: RequestStatus{
			Response: Response{
				StatusCode: 200,
				Body:       "test",
			},
			Failed: 3,
			RequestDetails: Mapping{
				Method: "POST",
				URL:    "https://example.com",
			},
		},
	}

	resp := req.GetResponse()
	if resp == nil {
		t.Error("GetResponse() returned nil")
	}
	if resp.GetStatusCode() != 200 {
		t.Errorf("Response StatusCode = %v, want 200", resp.GetStatusCode())
	}

	if got := req.GetFailed(); got != 3 {
		t.Errorf("GetFailed() = %v, want 3", got)
	}

	details := req.GetRequestDetails()
	if details == nil {
		t.Error("GetRequestDetails() returned nil")
	}
	if details.GetMethod() != "POST" {
		t.Errorf("RequestDetails Method = %v, want POST", details.GetMethod())
	}
}

func TestRequest_RequestResource(t *testing.T) {
	req := &Request{
		Spec: RequestSpec{
			ForProvider: RequestParameters{
				Payload: Payload{
					BaseUrl: "https://api.example.com",
				},
			},
		},
	}

	spec := req.GetSpec()
	if spec == nil {
		t.Error("GetSpec() returned nil")
	}

	payload := spec.GetPayload()
	if payload == nil {
		t.Error("Spec.GetPayload() returned nil")
	}
	if payload.GetBaseURL() != "https://api.example.com" {
		t.Errorf("Payload BaseURL = %v, want https://api.example.com", payload.GetBaseURL())
	}
}
