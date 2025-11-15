package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/google/go-cmp/cmp"
)

func TestHttpResponse_Implements_Interfaces(t *testing.T) {
	// Test that HttpResponse properly implements the interface methods
	resp := &HttpResponse{
		StatusCode: 200,
		Body:       "test body",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
	}

	if got := resp.GetStatusCode(); got != 200 {
		t.Errorf("GetStatusCode() = %v, want %v", got, 200)
	}

	if got := resp.GetBody(); got != "test body" {
		t.Errorf("GetBody() = %v, want %v", got, "test body")
	}

	if got := resp.GetHeaders(); len(got) != 1 {
		t.Errorf("GetHeaders() length = %v, want %v", len(got), 1)
	}
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name    string
		request HttpRequest
		want    string
	}{
		{
			name: "ValidRequest",
			request: HttpRequest{
				Method: "GET",
				URL:    "http://example.com",
				Body:   "test body",
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
			want: `{"method":"GET","body":"test body","url":"http://example.com","headers":{"Content-Type":["application/json"]}}`,
		},
		{
			name: "RequestWithoutBody",
			request: HttpRequest{
				Method: "GET",
				URL:    "http://example.com",
			},
			want: `{"method":"GET","url":"http://example.com"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSON(tt.request)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("toJSON() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClient_SendRequest(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		requestBody    string
		requestHeaders map[string][]string
		serverHandler  http.HandlerFunc
		skipTLSVerify  bool
		authToken      string
		wantErr        bool
		wantStatusCode int
		wantBody       string
	}{
		{
			name:        "SuccessfulGETRequest",
			method:      "GET",
			requestBody: "",
			requestHeaders: map[string][]string{
				"Accept": {"application/json"},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			},
			wantErr:        false,
			wantStatusCode: 200,
			wantBody:       `{"status":"ok"}`,
		},
		{
			name:        "SuccessfulPOSTRequest",
			method:      "POST",
			requestBody: `{"key":"value"}`,
			requestHeaders: map[string][]string{
				"Content-Type": {"application/json"},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), "key") {
					t.Errorf("Expected body to contain 'key', got %s", string(body))
				}
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"created":true}`))
			},
			wantErr:        false,
			wantStatusCode: 201,
			wantBody:       `{"created":true}`,
		},
		{
			name:        "RequestWithAuthToken",
			method:      "GET",
			requestBody: "",
			requestHeaders: map[string][]string{
				"Accept": {"application/json"},
			},
			authToken: "Bearer test-token",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-token" {
					t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", authHeader)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"authenticated":true}`))
			},
			wantErr:        false,
			wantStatusCode: 200,
			wantBody:       `{"authenticated":true}`,
		},
		{
			name:        "RequestWithExistingAuthHeader",
			method:      "GET",
			requestBody: "",
			requestHeaders: map[string][]string{
				"Authorization": {"Bearer custom-token"},
			},
			authToken: "Bearer default-token",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				// Should use the custom token from headers, not the default
				if authHeader != "Bearer custom-token" {
					t.Errorf("Expected Authorization header 'Bearer custom-token', got '%s'", authHeader)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ok":true}`))
			},
			wantErr:        false,
			wantStatusCode: 200,
			wantBody:       `{"ok":true}`,
		},
		{
			name:        "ServerError",
			method:      "GET",
			requestBody: "",
			requestHeaders: map[string][]string{
				"Accept": {"application/json"},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal server error"}`))
			},
			wantErr:        false,
			wantStatusCode: 500,
			wantBody:       `{"error":"internal server error"}`,
		},
		{
			name:        "MultipleHeaderValues",
			method:      "GET",
			requestBody: "",
			requestHeaders: map[string][]string{
				"X-Custom": {"value1", "value2"},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				customHeaders := r.Header["X-Custom"]
				if len(customHeaders) != 2 {
					t.Errorf("Expected 2 X-Custom headers, got %d", len(customHeaders))
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ok":true}`))
			},
			wantErr:        false,
			wantStatusCode: 200,
			wantBody:       `{"ok":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			// Create client
			log := logging.NewNopLogger()
			c, err := NewClient(log, 10*time.Second, tt.authToken)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			// Prepare request data
			bodyData := Data{
				Encrypted: tt.requestBody,
				Decrypted: tt.requestBody,
			}
			headerData := Data{
				Encrypted: tt.requestHeaders,
				Decrypted: tt.requestHeaders,
			}

			// Send request
			ctx := context.Background()
			got, err := c.SendRequest(ctx, tt.method, server.URL, bodyData, headerData, tt.skipTLSVerify)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if got.HttpResponse.StatusCode != tt.wantStatusCode {
					t.Errorf("SendRequest() StatusCode = %v, want %v", got.HttpResponse.StatusCode, tt.wantStatusCode)
				}

				if got.HttpResponse.Body != tt.wantBody {
					t.Errorf("SendRequest() Body = %v, want %v", got.HttpResponse.Body, tt.wantBody)
				}

				// Verify request details are captured
				if got.HttpRequest.Method != tt.method {
					t.Errorf("HttpRequest.Method = %v, want %v", got.HttpRequest.Method, tt.method)
				}

				if got.HttpRequest.URL != server.URL {
					t.Errorf("HttpRequest.URL = %v, want %v", got.HttpRequest.URL, server.URL)
				}
			}
		})
	}
}

func TestClient_SendRequest_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with very short timeout
	log := logging.NewNopLogger()
	c, err := NewClient(log, 50*time.Millisecond, "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	bodyData := Data{
		Encrypted: "",
		Decrypted: "",
	}
	headerData := Data{
		Encrypted: map[string][]string{},
		Decrypted: map[string][]string{},
	}

	ctx := context.Background()
	_, err = c.SendRequest(ctx, "GET", server.URL, bodyData, headerData, false)

	if err == nil {
		t.Error("SendRequest() expected timeout error, got nil")
	}
}

func TestClient_SendRequest_InvalidURL(t *testing.T) {
	log := logging.NewNopLogger()
	c, err := NewClient(log, 10*time.Second, "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	bodyData := Data{
		Encrypted: "",
		Decrypted: "",
	}
	headerData := Data{
		Encrypted: map[string][]string{},
		Decrypted: map[string][]string{},
	}

	ctx := context.Background()
	_, err = c.SendRequest(ctx, "GET", "://invalid-url", bodyData, headerData, false)

	if err == nil {
		t.Error("SendRequest() expected error for invalid URL, got nil")
	}
}

func TestClient_SendRequest_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logging.NewNopLogger()
	c, err := NewClient(log, 10*time.Second, "")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	bodyData := Data{
		Encrypted: "",
		Decrypted: "",
	}
	headerData := Data{
		Encrypted: map[string][]string{},
		Decrypted: map[string][]string{},
	}

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = c.SendRequest(ctx, "GET", server.URL, bodyData, headerData, false)

	if err == nil {
		t.Error("SendRequest() expected context cancellation error, got nil")
	}
}

func TestNewClient(t *testing.T) {
	log := logging.NewNopLogger()
	timeout := 30 * time.Second
	authToken := "Bearer test-token"

	client, err := NewClient(log, timeout, authToken)
	if err != nil {
		t.Errorf("NewClient() error = %v, want nil", err)
	}

	if client == nil {
		t.Error("NewClient() returned nil client")
	}

	// Verify client is not nil and is usable (we can't inspect internal fields)
	// Just verify it was created successfully by checking it's not nil
	_ = client
}

func TestClient_SendRequest_ResponseHeadersAndJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.Header().Add("X-Multi", "value1")
		w.Header().Add("X-Multi", "value2")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"message": "success",
			"data":    map[string]string{"key": "value"},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	log := logging.NewNopLogger()
	c, _ := NewClient(log, 10*time.Second, "")

	bodyData := Data{
		Encrypted: "",
		Decrypted: "",
	}
	headerData := Data{
		Encrypted: map[string][]string{},
		Decrypted: map[string][]string{},
	}

	ctx := context.Background()
	got, err := c.SendRequest(ctx, "GET", server.URL, bodyData, headerData, false)

	if err != nil {
		t.Fatalf("SendRequest() error = %v", err)
	}

	// Check custom header
	customHeaders := got.HttpResponse.Headers["X-Custom-Header"]
	if len(customHeaders) == 0 || customHeaders[0] != "custom-value" {
		t.Errorf("Expected X-Custom-Header = 'custom-value', got '%v'", customHeaders)
	}

	// Check multi-value header
	multiHeaders := got.HttpResponse.Headers["X-Multi"]
	if len(multiHeaders) != 2 {
		t.Errorf("Expected 2 X-Multi headers, got %d", len(multiHeaders))
	}

	// Verify body is valid JSON
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(got.HttpResponse.Body), &response); err != nil {
		t.Errorf("Response body is not valid JSON: %v", err)
	}
}

func TestClient_SendRequest_DifferentEncryptedDecrypted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and verify the actual request body (decrypted)
		body, _ := io.ReadAll(r.Body)
		if string(body) != "decrypted-data" {
			t.Errorf("Expected body 'decrypted-data', got '%s'", string(body))
		}

		// Verify the actual headers (decrypted)
		if r.Header.Get("X-Secret") != "actual-secret" {
			t.Errorf("Expected header X-Secret = 'actual-secret', got '%s'", r.Header.Get("X-Secret"))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	log := logging.NewNopLogger()
	c, _ := NewClient(log, 10*time.Second, "")

	// Body data: encrypted for status/logging, decrypted for actual request
	bodyData := Data{
		Encrypted: "***encrypted***",
		Decrypted: "decrypted-data",
	}

	// Header data: encrypted for status/logging, decrypted for actual request
	headerData := Data{
		Encrypted: map[string][]string{
			"X-Secret": {"***hidden***"},
		},
		Decrypted: map[string][]string{
			"X-Secret": {"actual-secret"},
		},
	}

	ctx := context.Background()
	got, err := c.SendRequest(ctx, "POST", server.URL, bodyData, headerData, false)

	if err != nil {
		t.Fatalf("SendRequest() error = %v", err)
	}

	// Verify that the request details contain encrypted data (for logging/status)
	if got.HttpRequest.Body != "***encrypted***" {
		t.Errorf("HttpRequest.Body = %v, want '***encrypted***'", got.HttpRequest.Body)
	}

	if got.HttpRequest.Headers["X-Secret"][0] != "***hidden***" {
		t.Errorf("HttpRequest.Headers[X-Secret] = %v, want '***hidden***'", got.HttpRequest.Headers["X-Secret"][0])
	}
}
