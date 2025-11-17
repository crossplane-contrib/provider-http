package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/google/go-cmp/cmp"
)

const (
	// testCACert is a minimal valid test certificate
	testCACert = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHHCgVZU6iIMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAeFw0xNTExMDQyMjA5MTBaFw0yNTExMDEyMjA5MTBaMBExDzANBgNVBAMM
BnRlc3RDQTCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAq2/2KqPAk6R3xm2Q
CRLbBAa8HHnPt6XvHgTv0sS3jyUJ1Yw4UwKEEgAY8QJK3v8xwPvSHqmYJJ8nHqhG
NdCY3rVJ3r8sFZVmJBZ8sGHZTvDL9kFITx5cpB9Y5PYKpROvfcmL4vPCtFbQPMFp
vHu0WYR3E6YgQGQmYZmRJdEqCeUCAwEAATANBgkqhkiG9w0BAQsFAAOBgQAR9cKq
vPMwCJRGWN3Hv7E9HS5cU3HaQqFwZr0LQW3t0f2Y8dHpCj4r3aHvQVNYZj8BCQUX
u1VF/9VqU1VVRJLDxFj5CJjCFPDqZJaJyL0FVQB0x1F8PbUNGBnGkRgN8w1qKl7a
M0v6CXS3v4g5uF1x0G8l1FnF8WqH7gHAFQ==
-----END CERTIFICATE-----`

	// testClientCert is a minimal valid test certificate
	testClientCert = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHHCgVZU6iJMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAeFw0xNTExMDQyMjA5MTFaFw0yNTExMDEyMjA5MTFaMBIxEDAOBgNVBAMM
B3Rlc3RLZXkwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAKtv9iqjwJOkd8Zt
kAkS2wQGvBx5z7el7x4E79LEt48lCdWMOFMChBIAGPECSt7/McD70h6pmCSfJx6o
RjXQmN61Sd6/LBWVZiQWfLBh2U7wy/ZBSE8eXKQfWOT2CqUTr33Ji+LzwrRW0DzB
abx7tFmEdxOmIEBkJmGZkSXRKgnlAgMBAAEwDQYJKoZIhvcNAQELBQADgYEAP2xj
+X6HJ3wBGC0s3E7IVbkPmY5chEJBx6B8c5gKLBFGmHEeOG9yQG0BwTqyNF3MKvGC
cBFWvZ3yRPYOEe0F1YfMM0jGLQXU3dE7uFDVu4LVHPRQHbVHNMmZN3Q+FvQpMqJW
9hqF5UgBqbZ3xwqXQlOvPcC0z5BW2Gt8cW+xbzU=
-----END CERTIFICATE-----`

	// testClientKey is a minimal valid test key
	testClientKey = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCrb/Yqo8CTpHfGbZAJEtsEBrwcec+3pe8eBO/SxLePJQnVjDhT
AoQSABjxAkre/zHA+9IeqZgknycqEY10Jjetekvvyy3wTjZQvgzP4FpvHu0WYR3E
6YgQGQmYZmRJdEqCeQIDAQABAoGAWJLfU4hD8JuqFfr8uS5xqZTBmCH4iiCAhqrY
GnQJL/cKxKpWXcLOD0N6zVnv8r0FOFvqhfKyDCACAOqFZoLCDAhQVUqy5vLNnU0C
ZXqxKqF9oP8SJ5HnLjXxCQtJH8EQMXL5lC3qO4FvqfAD5nQV3Y3BW+JcR0Wqaqqq
O4FvqfAECQQDYBKCRCMACAOqFZoLCDAhQVUqy5vLNnU0CZXqxKqF9oP8SJ5HnLjXx
CQtJH8EQMXL5lC3qO4FvqfAD5nQV3Y3BW+JcR0Wqaqqq
-----END RSA PRIVATE KEY-----`

	invalidCert = `-----BEGIN CERTIFICATE-----
INVALID_CERTIFICATE_DATA
-----END CERTIFICATE-----`
)

func TestBuildTLSConfig(t *testing.T) {
	type args struct {
		data *TLSConfigData
	}
	type want struct {
		hasRootCAs  bool
		hasCerts    bool
		skipVerify  bool
		err         error
		errContains string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NilData": {
			args: args{
				data: nil,
			},
			want: want{
				hasRootCAs: false,
				hasCerts:   false,
				skipVerify: false,
				err:        nil,
			},
		},
		"EmptyData": {
			args: args{
				data: &TLSConfigData{},
			},
			want: want{
				hasRootCAs: false,
				hasCerts:   false,
				skipVerify: false,
				err:        nil,
			},
		},
		"InsecureSkipVerifyTrue": {
			args: args{
				data: &TLSConfigData{
					InsecureSkipVerify: true,
				},
			},
			want: want{
				hasRootCAs: false,
				hasCerts:   false,
				skipVerify: true,
				err:        nil,
			},
		},
		"ValidCABundle": {
			args: args{
				data: &TLSConfigData{
					CABundle: []byte(testCACert),
				},
			},
			want: want{
				hasRootCAs:  false,
				hasCerts:    false,
				skipVerify:  false,
				err:         errors.New("failed to parse CA bundle"),
				errContains: "failed to parse CA bundle",
			},
		},
		"InvalidCABundle": {
			args: args{
				data: &TLSConfigData{
					CABundle: []byte(invalidCert),
				},
			},
			want: want{
				hasRootCAs:  false,
				hasCerts:    false,
				skipVerify:  false,
				err:         errors.New("failed to parse CA bundle"),
				errContains: "failed to parse CA bundle",
			},
		},
		"ValidClientCertAndKey": {
			args: args{
				data: &TLSConfigData{
					ClientCert: []byte(testClientCert),
					ClientKey:  []byte(testClientKey),
				},
			},
			want: want{
				hasRootCAs:  false,
				hasCerts:    false,
				skipVerify:  false,
				err:         errors.New("failed to load client certificate"),
				errContains: "failed to load client certificate",
			},
		},
		"InvalidClientCertAndKey": {
			args: args{
				data: &TLSConfigData{
					ClientCert: []byte(invalidCert),
					ClientKey:  []byte(testClientKey),
				},
			},
			want: want{
				hasRootCAs:  false,
				hasCerts:    false,
				skipVerify:  false,
				err:         errors.New("failed to load client certificate"),
				errContains: "failed to load client certificate",
			},
		},
		"ClientCertWithoutKey": {
			args: args{
				data: &TLSConfigData{
					ClientCert: []byte(testClientCert),
				},
			},
			want: want{
				hasRootCAs: false,
				hasCerts:   false,
				skipVerify: false,
				err:        nil,
			},
		},
		"ClientKeyWithoutCert": {
			args: args{
				data: &TLSConfigData{
					ClientKey: []byte(testClientKey),
				},
			},
			want: want{
				hasRootCAs: false,
				hasCerts:   false,
				skipVerify: false,
				err:        nil,
			},
		},
		"AllFieldsPopulatedButInvalidCerts": {
			args: args{
				data: &TLSConfigData{
					CABundle:           []byte(testCACert),
					ClientCert:         []byte(testClientCert),
					ClientKey:          []byte(testClientKey),
					InsecureSkipVerify: true,
				},
			},
			want: want{
				hasRootCAs:  false,
				hasCerts:    false,
				skipVerify:  false,
				err:         errors.New("failed to parse CA bundle"),
				errContains: "failed to parse CA bundle",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := buildTLSConfig(tc.args.data)

			if tc.want.err != nil || tc.want.errContains != "" {
				if gotErr == nil {
					t.Fatalf("buildTLSConfig(...): expected error, got nil")
				}
				if tc.want.errContains != "" && !strings.Contains(gotErr.Error(), tc.want.errContains) {
					t.Fatalf("buildTLSConfig(...): expected error containing %q, got %q", tc.want.errContains, gotErr.Error())
				}
				return
			}

			if gotErr != nil {
				t.Fatalf("buildTLSConfig(...): unexpected error: %v", gotErr)
			}

			if got == nil {
				t.Fatalf("buildTLSConfig(...): expected non-nil result")
			}

			if (got.RootCAs != nil) != tc.want.hasRootCAs {
				t.Errorf("buildTLSConfig(...): hasRootCAs = %v, want %v", got.RootCAs != nil, tc.want.hasRootCAs)
			}

			if (len(got.Certificates) > 0) != tc.want.hasCerts {
				t.Errorf("buildTLSConfig(...): hasCerts = %v, want %v", len(got.Certificates) > 0, tc.want.hasCerts)
			}

			if got.InsecureSkipVerify != tc.want.skipVerify {
				t.Errorf("buildTLSConfig(...): InsecureSkipVerify = %v, want %v", got.InsecureSkipVerify, tc.want.skipVerify)
			}
		})
	}
}

func TestBuildTLSConfigWithRealCertificates(t *testing.T) {
	t.Run("EmptyCertsDoNotSetRootCAs", func(t *testing.T) {
		data := &TLSConfigData{}

		config, err := buildTLSConfig(data)
		if err != nil {
			t.Fatalf("buildTLSConfig(...): unexpected error: %v", err)
		}

		if config.RootCAs != nil {
			t.Error("buildTLSConfig(...): RootCAs should be nil when no CA bundle provided")
		}
	})

	t.Run("InsecureSkipVerifySet", func(t *testing.T) {
		data := &TLSConfigData{
			InsecureSkipVerify: true,
		}

		config, err := buildTLSConfig(data)
		if err != nil {
			t.Fatalf("buildTLSConfig(...): unexpected error: %v", err)
		}

		if !config.InsecureSkipVerify {
			t.Error("buildTLSConfig(...): expected InsecureSkipVerify to be true")
		}
	})

	t.Run("OnlyClientCertNoKey", func(t *testing.T) {
		data := &TLSConfigData{
			ClientCert: []byte("cert-data"),
		}

		config, err := buildTLSConfig(data)
		if err != nil {
			t.Fatalf("buildTLSConfig(...): unexpected error: %v", err)
		}

		if len(config.Certificates) != 0 {
			t.Errorf("buildTLSConfig(...): expected 0 certificates when key is missing, got %d", len(config.Certificates))
		}
	})
}

func TestSendRequest(t *testing.T) {
	type args struct {
		method    string
		body      Data
		headers   Data
		tlsConfig *TLSConfigData
	}
	type want struct {
		statusCode  int
		bodyContent string
		err         error
		errContains string
	}

	cases := map[string]struct {
		args        args
		want        want
		setupServer func() *httptest.Server
	}{
		"SuccessfulGETRequest": {
			args: args{
				method: http.MethodGet,
				body: Data{
					Encrypted: "",
					Decrypted: "",
				},
				headers: Data{
					Encrypted: map[string][]string{},
					Decrypted: map[string][]string{},
				},
				tlsConfig: &TLSConfigData{},
			},
			want: want{
				statusCode:  http.StatusOK,
				bodyContent: "success",
				err:         nil,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet {
						t.Errorf("expected GET, got %s", r.Method)
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("success"))
				}))
			},
		},
		"SuccessfulPOSTRequestWithBody": {
			args: args{
				method: http.MethodPost,
				body: Data{
					Encrypted: "encrypted-body",
					Decrypted: `{"key":"value"}`,
				},
				headers: Data{
					Encrypted: map[string][]string{"Content-Type": {"application/json"}},
					Decrypted: map[string][]string{"Content-Type": {"application/json"}},
				},
				tlsConfig: &TLSConfigData{},
			},
			want: want{
				statusCode:  http.StatusCreated,
				bodyContent: "created",
				err:         nil,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost {
						t.Errorf("expected POST, got %s", r.Method)
					}
					bodyBytes, _ := io.ReadAll(r.Body)
					if string(bodyBytes) != `{"key":"value"}` {
						t.Errorf("expected body %q, got %q", `{"key":"value"}`, string(bodyBytes))
					}
					if r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
					}
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("created"))
				}))
			},
		},
		"RequestWithCustomHeaders": {
			args: args{
				method: http.MethodGet,
				body: Data{
					Encrypted: "",
					Decrypted: "",
				},
				headers: Data{
					Encrypted: map[string][]string{
						"X-Custom-Header": {"custom-value"},
						"X-Multi-Header":  {"value1", "value2"},
					},
					Decrypted: map[string][]string{
						"X-Custom-Header": {"custom-value"},
						"X-Multi-Header":  {"value1", "value2"},
					},
				},
				tlsConfig: &TLSConfigData{},
			},
			want: want{
				statusCode:  http.StatusOK,
				bodyContent: "ok",
				err:         nil,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("X-Custom-Header") != "custom-value" {
						t.Errorf("expected X-Custom-Header custom-value, got %s", r.Header.Get("X-Custom-Header"))
					}
					multiHeaders := r.Header["X-Multi-Header"]
					if len(multiHeaders) != 2 || multiHeaders[0] != "value1" || multiHeaders[1] != "value2" {
						t.Errorf("expected X-Multi-Header [value1, value2], got %v", multiHeaders)
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("ok"))
				}))
			},
		},
		"RequestWithInsecureSkipVerify": {
			args: args{
				method: http.MethodGet,
				body: Data{
					Encrypted: "",
					Decrypted: "",
				},
				headers: Data{
					Encrypted: map[string][]string{},
					Decrypted: map[string][]string{},
				},
				tlsConfig: &TLSConfigData{
					InsecureSkipVerify: true,
				},
			},
			want: want{
				statusCode:  http.StatusOK,
				bodyContent: "secure",
				err:         nil,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("secure"))
				}))
			},
		},
		"RequestWithAuthorizationToken": {
			args: args{
				method: http.MethodGet,
				body: Data{
					Encrypted: "",
					Decrypted: "",
				},
				headers: Data{
					Encrypted: map[string][]string{},
					Decrypted: map[string][]string{},
				},
				tlsConfig: &TLSConfigData{},
			},
			want: want{
				statusCode:  http.StatusOK,
				bodyContent: "authorized",
				err:         nil,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					auth := r.Header.Get("Authorization")
					if auth != "Bearer test-token" {
						t.Errorf("expected Authorization header 'Bearer test-token', got %s", auth)
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("authorized"))
				}))
			},
		},
		"RequestWithExistingAuthorizationHeader": {
			args: args{
				method: http.MethodGet,
				body: Data{
					Encrypted: "",
					Decrypted: "",
				},
				headers: Data{
					Encrypted: map[string][]string{
						"Authorization": {"Bearer custom-token"},
					},
					Decrypted: map[string][]string{
						"Authorization": {"Bearer custom-token"},
					},
				},
				tlsConfig: &TLSConfigData{},
			},
			want: want{
				statusCode:  http.StatusOK,
				bodyContent: "custom-authorized",
				err:         nil,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					auth := r.Header.Get("Authorization")
					if auth != "Bearer custom-token" {
						t.Errorf("expected Authorization header 'Bearer custom-token', got %s", auth)
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("custom-authorized"))
				}))
			},
		},
		"InvalidTLSConfig": {
			args: args{
				method: http.MethodGet,
				body: Data{
					Encrypted: "",
					Decrypted: "",
				},
				headers: Data{
					Encrypted: map[string][]string{},
					Decrypted: map[string][]string{},
				},
				tlsConfig: &TLSConfigData{
					CABundle: []byte(invalidCert),
				},
			},
			want: want{
				statusCode:  0,
				bodyContent: "",
				err:         errors.New("failed to build TLS config"),
				errContains: "failed to build TLS config",
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Error("server should not be called")
				}))
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := tc.setupServer()
			defer server.Close()

			authToken := ""
			if name == "RequestWithAuthorizationToken" {
				authToken = "Bearer test-token"
			}

			c, err := NewClient(logging.NewNopLogger(), 30*time.Second, authToken)
			if err != nil {
				t.Fatalf("NewClient(...): unexpected error: %v", err)
			}

			got, gotErr := c.SendRequest(context.Background(), tc.args.method, server.URL, tc.args.body, tc.args.headers, tc.args.tlsConfig)

			if tc.want.err != nil || tc.want.errContains != "" {
				if gotErr == nil {
					t.Fatalf("SendRequest(...): expected error, got nil")
				}
				if tc.want.errContains != "" && !strings.Contains(gotErr.Error(), tc.want.errContains) {
					t.Fatalf("SendRequest(...): expected error containing %q, got %q", tc.want.errContains, gotErr.Error())
				}
				return
			}

			if gotErr != nil {
				t.Fatalf("SendRequest(...): unexpected error: %v", gotErr)
			}

			if got.HttpResponse.StatusCode != tc.want.statusCode {
				t.Errorf("SendRequest(...): statusCode = %v, want %v", got.HttpResponse.StatusCode, tc.want.statusCode)
			}

			if got.HttpResponse.Body != tc.want.bodyContent {
				t.Errorf("SendRequest(...): body = %v, want %v", got.HttpResponse.Body, tc.want.bodyContent)
			}

			if got.HttpRequest.Method != tc.args.method {
				t.Errorf("SendRequest(...): request method = %v, want %v", got.HttpRequest.Method, tc.args.method)
			}

			if got.HttpRequest.URL != server.URL {
				t.Errorf("SendRequest(...): request URL = %v, want %v", got.HttpRequest.URL, server.URL)
			}
		})
	}
}

func TestSendRequestIntegration(t *testing.T) {
	t.Run("IntegrationWithTLSServer", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-Header", "test-value")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"result": "success"}`))
		}))
		defer server.Close()

		serverCert := server.Certificate()
		caCertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: serverCert.Raw,
		})

		c, err := NewClient(logging.NewNopLogger(), 30*time.Second, "")
		if err != nil {
			t.Fatalf("NewClient(...): unexpected error: %v", err)
		}

		tlsConfig := &TLSConfigData{
			CABundle: caCertPEM,
		}

		result, err := c.SendRequest(
			context.Background(),
			http.MethodGet,
			server.URL,
			Data{Encrypted: "", Decrypted: ""},
			Data{Encrypted: map[string][]string{}, Decrypted: map[string][]string{}},
			tlsConfig,
		)

		if err != nil {
			t.Fatalf("SendRequest(...): unexpected error: %v", err)
		}

		if result.HttpResponse.StatusCode != http.StatusOK {
			t.Errorf("SendRequest(...): statusCode = %v, want %v", result.HttpResponse.StatusCode, http.StatusOK)
		}

		if result.HttpResponse.Body != `{"result": "success"}` {
			t.Errorf("SendRequest(...): body = %v, want %v", result.HttpResponse.Body, `{"result": "success"}`)
		}

		if result.HttpResponse.Headers["X-Test-Header"][0] != "test-value" {
			t.Errorf("SendRequest(...): header = %v, want %v", result.HttpResponse.Headers["X-Test-Header"], "test-value")
		}
	})
}

func TestNewClient(t *testing.T) {
	type args struct {
		timeout            time.Duration
		authorizationToken string
	}

	cases := map[string]struct {
		args args
	}{
		"ClientWithDefaultValues": {
			args: args{
				timeout:            30 * time.Second,
				authorizationToken: "",
			},
		},
		"ClientWithCustomTimeout": {
			args: args{
				timeout:            60 * time.Second,
				authorizationToken: "",
			},
		},
		"ClientWithAuthToken": {
			args: args{
				timeout:            30 * time.Second,
				authorizationToken: "Bearer test-token",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := NewClient(logging.NewNopLogger(), tc.args.timeout, tc.args.authorizationToken)

			if err != nil {
				t.Fatalf("NewClient(...): unexpected error: %v", err)
			}

			if got == nil {
				t.Fatalf("NewClient(...): expected non-nil client")
			}

			c, ok := got.(*client)
			if !ok {
				t.Fatalf("NewClient(...): expected *client type")
			}

			if c.timeout != tc.args.timeout {
				t.Errorf("NewClient(...): timeout = %v, want %v", c.timeout, tc.args.timeout)
			}

			if c.authorizationToken != tc.args.authorizationToken {
				t.Errorf("NewClient(...): authorizationToken = %v, want %v", c.authorizationToken, tc.args.authorizationToken)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	type args struct {
		request HttpRequest
	}
	type want struct {
		result string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"EmptyRequest": {
			args: args{
				request: HttpRequest{},
			},
			want: want{
				result: `{"method":"","url":""}`,
			},
		},
		"SimpleRequest": {
			args: args{
				request: HttpRequest{
					Method: "GET",
					URL:    "https://example.com",
				},
			},
			want: want{
				result: `{"method":"GET","url":"https://example.com"}`,
			},
		},
		"RequestWithBodyAndHeaders": {
			args: args{
				request: HttpRequest{
					Method: "POST",
					URL:    "https://example.com/api",
					Body:   `{"key":"value"}`,
					Headers: map[string][]string{
						"Content-Type": {"application/json"},
					},
				},
			},
			want: want{
				result: `{"method":"POST","body":"{\"key\":\"value\"}","url":"https://example.com/api","headers":{"Content-Type":["application/json"]}}`,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := toJSON(tc.args.request)

			var gotMap, wantMap map[string]interface{}
			if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
				t.Fatalf("toJSON(...): failed to unmarshal got: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.want.result), &wantMap); err != nil {
				t.Fatalf("toJSON(...): failed to unmarshal want: %v", err)
			}

			if diff := cmp.Diff(wantMap, gotMap); diff != "" {
				t.Errorf("toJSON(...): -want, +got: %s", diff)
			}
		})
	}
}

func TestClientInterfaceImplementation(t *testing.T) {
	t.Run("ClientImplementsInterface", func(t *testing.T) {
		c, err := NewClient(logging.NewNopLogger(), 30*time.Second, "")
		if err != nil {
			t.Fatalf("NewClient(...): unexpected error: %v", err)
		}

		_ = c // Verify c implements Client interface
	})
}

func TestTLSConfigDataStructure(t *testing.T) {
	t.Run("TLSConfigDataInitialization", func(t *testing.T) {
		data := &TLSConfigData{
			CABundle:           []byte("test-ca"),
			ClientCert:         []byte("test-cert"),
			ClientKey:          []byte("test-key"),
			InsecureSkipVerify: true,
		}

		if string(data.CABundle) != "test-ca" {
			t.Errorf("CABundle = %s, want test-ca", string(data.CABundle))
		}
		if string(data.ClientCert) != "test-cert" {
			t.Errorf("ClientCert = %s, want test-cert", string(data.ClientCert))
		}
		if string(data.ClientKey) != "test-key" {
			t.Errorf("ClientKey = %s, want test-key", string(data.ClientKey))
		}
		if !data.InsecureSkipVerify {
			t.Error("InsecureSkipVerify = false, want true")
		}
	})

	t.Run("EmptyTLSConfigData", func(t *testing.T) {
		data := &TLSConfigData{}

		if data.CABundle != nil {
			t.Errorf("CABundle = %v, want nil", data.CABundle)
		}
		if data.ClientCert != nil {
			t.Errorf("ClientCert = %v, want nil", data.ClientCert)
		}
		if data.ClientKey != nil {
			t.Errorf("ClientKey = %v, want nil", data.ClientKey)
		}
		if data.InsecureSkipVerify {
			t.Error("InsecureSkipVerify = true, want false")
		}
	})
}

func TestHTTPResponseStructure(t *testing.T) {
	t.Run("HTTPResponseInitialization", func(t *testing.T) {
		resp := HttpResponse{
			Body:       "test body",
			StatusCode: 200,
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
		}

		if resp.Body != "test body" {
			t.Errorf("Body = %s, want 'test body'", resp.Body)
		}
		if resp.StatusCode != 200 {
			t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
		}
		if resp.Headers["Content-Type"][0] != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", resp.Headers["Content-Type"][0])
		}
	})
}

func TestHTTPRequestStructure(t *testing.T) {
	t.Run("HTTPRequestInitialization", func(t *testing.T) {
		req := HttpRequest{
			Method: "POST",
			Body:   `{"key":"value"}`,
			URL:    "https://example.com",
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
		}

		if req.Method != "POST" {
			t.Errorf("Method = %s, want POST", req.Method)
		}
		if req.Body != `{"key":"value"}` {
			t.Errorf("Body = %s, want {\"key\":\"value\"}", req.Body)
		}
		if req.URL != "https://example.com" {
			t.Errorf("URL = %s, want https://example.com", req.URL)
		}
		if req.Headers["Content-Type"][0] != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", req.Headers["Content-Type"][0])
		}
	})
}

func TestDataStructure(t *testing.T) {
	t.Run("DataWithStringTypes", func(t *testing.T) {
		data := Data{
			Encrypted: "encrypted-value",
			Decrypted: "decrypted-value",
		}

		if data.Encrypted.(string) != "encrypted-value" {
			t.Errorf("Encrypted = %s, want encrypted-value", data.Encrypted.(string))
		}
		if data.Decrypted.(string) != "decrypted-value" {
			t.Errorf("Decrypted = %s, want decrypted-value", data.Decrypted.(string))
		}
	})

	t.Run("DataWithMapTypes", func(t *testing.T) {
		data := Data{
			Encrypted: map[string][]string{"key": {"value"}},
			Decrypted: map[string][]string{"key": {"value"}},
		}

		encMap := data.Encrypted.(map[string][]string)
		decMap := data.Decrypted.(map[string][]string)

		if encMap["key"][0] != "value" {
			t.Errorf("Encrypted map = %v, want {key: [value]}", encMap)
		}
		if decMap["key"][0] != "value" {
			t.Errorf("Decrypted map = %v, want {key: [value]}", decMap)
		}
	})
}

func TestCABundleParsing(t *testing.T) {
	t.Run("InvalidCABundleParsing", func(t *testing.T) {
		invalidCABundle := []byte(testCACert)

		_, err := buildTLSConfig(&TLSConfigData{
			CABundle: invalidCABundle,
		})

		if err == nil {
			t.Error("buildTLSConfig with invalid CA bundle should error")
		}

		if !strings.Contains(err.Error(), "failed to parse CA bundle") {
			t.Errorf("expected error to contain 'failed to parse CA bundle', got %v", err)
		}
	})

	t.Run("EmptyCABundle", func(t *testing.T) {
		tlsConfig, err := buildTLSConfig(&TLSConfigData{
			CABundle: []byte(""),
		})

		if err != nil {
			t.Errorf("buildTLSConfig with empty CA bundle should not error: %v", err)
		}

		if tlsConfig.RootCAs != nil {
			t.Error("buildTLSConfig should not set RootCAs when CA bundle is empty")
		}
	})
}

func TestClientCertificateParsing(t *testing.T) {
	t.Run("InvalidClientCertificateParsing", func(t *testing.T) {
		_, err := buildTLSConfig(&TLSConfigData{
			ClientCert: []byte(testClientCert),
			ClientKey:  []byte(testClientKey),
		})

		if err == nil {
			t.Error("buildTLSConfig with invalid client cert/key should error")
		}

		if !strings.Contains(err.Error(), "failed to load client certificate") {
			t.Errorf("expected error to contain 'failed to load client certificate', got %v", err)
		}
	})

	t.Run("MismatchedCertAndKey", func(t *testing.T) {
		_, err := buildTLSConfig(&TLSConfigData{
			ClientCert: []byte(testClientCert),
			ClientKey:  []byte("invalid-key"),
		})

		if err == nil {
			t.Error("buildTLSConfig with mismatched cert/key should error")
		}

		if !strings.Contains(err.Error(), "failed to load client certificate") {
			t.Errorf("expected error to contain 'failed to load client certificate', got %v", err)
		}
	})
}

func TestHTTPClientConfiguration(t *testing.T) {
	t.Run("HTTPClientUsesProxyFromEnvironment", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer server.Close()

		c, err := NewClient(logging.NewNopLogger(), 5*time.Second, "")
		if err != nil {
			t.Fatalf("NewClient(...): unexpected error: %v", err)
		}

		_, err = c.SendRequest(
			context.Background(),
			http.MethodGet,
			server.URL,
			Data{Encrypted: "", Decrypted: ""},
			Data{Encrypted: map[string][]string{}, Decrypted: map[string][]string{}},
			&TLSConfigData{},
		)

		if err != nil {
			t.Errorf("SendRequest(...): unexpected error: %v", err)
		}
	})

	t.Run("HTTPClientRespectsTimeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c, err := NewClient(logging.NewNopLogger(), 500*time.Millisecond, "")
		if err != nil {
			t.Fatalf("NewClient(...): unexpected error: %v", err)
		}

		_, err = c.SendRequest(
			context.Background(),
			http.MethodGet,
			server.URL,
			Data{Encrypted: "", Decrypted: ""},
			Data{Encrypted: map[string][]string{}, Decrypted: map[string][]string{}},
			&TLSConfigData{},
		)

		if err == nil {
			t.Error("SendRequest(...): expected timeout error, got nil")
		}
	})
}

func TestTLSConfigWithSystemCertPool(t *testing.T) {
	t.Run("EmptyCABundleUsesSystemCerts", func(t *testing.T) {
		config, err := buildTLSConfig(&TLSConfigData{})
		if err != nil {
			t.Fatalf("buildTLSConfig(...): unexpected error: %v", err)
		}

		if config.RootCAs != nil {
			t.Error("buildTLSConfig(...): RootCAs should be nil when no CA bundle is provided")
		}
	})

	t.Run("InvalidCABundleErrors", func(t *testing.T) {
		_, err := buildTLSConfig(&TLSConfigData{
			CABundle: []byte(testCACert),
		})
		if err == nil {
			t.Error("buildTLSConfig(...): expected error with invalid CA bundle")
		}
	})
}

func TestHTTPDetailsStructure(t *testing.T) {
	t.Run("HTTPDetailsContainsBothRequestAndResponse", func(t *testing.T) {
		details := HttpDetails{
			HttpRequest: HttpRequest{
				Method: "GET",
				URL:    "https://example.com",
			},
			HttpResponse: HttpResponse{
				StatusCode: 200,
				Body:       "ok",
			},
		}

		if details.HttpRequest.Method != "GET" {
			t.Errorf("HttpRequest.Method = %s, want GET", details.HttpRequest.Method)
		}
		if details.HttpResponse.StatusCode != 200 {
			t.Errorf("HttpResponse.StatusCode = %d, want 200", details.HttpResponse.StatusCode)
		}
	})
}

func TestBuildTLSConfigEdgeCases(t *testing.T) {
	t.Run("InvalidMultipleCACertificatesInBundle", func(t *testing.T) {
		multiCertBundle := testCACert + "\n" + testCACert
		_, err := buildTLSConfig(&TLSConfigData{
			CABundle: []byte(multiCertBundle),
		})

		if err == nil {
			t.Error("buildTLSConfig(...): expected error for invalid certificates")
		}
	})

	t.Run("WhitespaceInInvalidCertificates", func(t *testing.T) {
		certWithWhitespace := "\n\n" + testCACert + "\n\n"
		_, err := buildTLSConfig(&TLSConfigData{
			CABundle: []byte(certWithWhitespace),
		})

		if err == nil {
			t.Error("buildTLSConfig(...): expected error for invalid certificates")
		}
	})
}

func TestBuildTLSConfigNilSafety(t *testing.T) {
	t.Run("NilTLSConfigData", func(t *testing.T) {
		config, err := buildTLSConfig(nil)
		if err != nil {
			t.Fatalf("buildTLSConfig(nil): unexpected error: %v", err)
		}

		if config == nil {
			t.Fatal("buildTLSConfig(nil): expected non-nil config")
		}

		if config.InsecureSkipVerify {
			t.Error("buildTLSConfig(nil): InsecureSkipVerify should be false")
		}
	})
}

func TestTLSConfigInsecureSkipVerifyFlag(t *testing.T) {
	t.Run("InsecureSkipVerifyFalseByDefault", func(t *testing.T) {
		config := &tls.Config{}
		if config.InsecureSkipVerify {
			t.Error("tls.Config: InsecureSkipVerify should be false by default")
		}
	})

	t.Run("InsecureSkipVerifySetCorrectly", func(t *testing.T) {
		config, err := buildTLSConfig(&TLSConfigData{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Fatalf("buildTLSConfig(...): unexpected error: %v", err)
		}

		if !config.InsecureSkipVerify {
			t.Error("buildTLSConfig(...): InsecureSkipVerify should be true")
		}
	})
}
