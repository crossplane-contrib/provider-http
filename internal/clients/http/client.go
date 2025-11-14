package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const (
	authKey = "Authorization"
)

// TLSConfigData contains the TLS configuration data loaded from secrets or inline.
type TLSConfigData struct {
	// CABundle contains PEM encoded CA certificates
	CABundle []byte
	// ClientCert contains PEM encoded client certificate
	ClientCert []byte
	// ClientKey contains PEM encoded client private key
	ClientKey []byte
	// InsecureSkipVerify controls whether to skip TLS verification
	InsecureSkipVerify bool
}

// Client is the interface to interact with Http
type Client interface {
	SendRequest(ctx context.Context, method string, url string, body Data, headers Data, tlsConfig *TLSConfigData) (resp HttpDetails, err error)
}

type client struct {
	log                logging.Logger
	timeout            time.Duration
	authorizationToken string
}

type HttpResponse struct {
	Body       string              `json:"body"`
	Headers    map[string][]string `json:"headers"`
	StatusCode int                 `json:"statusCode"`
}

// Ensure HttpResponse implements interfaces.HTTPResponse
var _ interfaces.HTTPResponse = (*HttpResponse)(nil)

// GetStatusCode returns the HTTP status code.
func (r *HttpResponse) GetStatusCode() int {
	return r.StatusCode
}

// GetBody returns the response body.
func (r *HttpResponse) GetBody() string {
	return r.Body
}

// GetHeaders returns the response headers.
func (r *HttpResponse) GetHeaders() map[string][]string {
	return r.Headers
}

type Data struct {
	Encrypted interface{} // Data containing encrypted data -> to be shown at the status
	Decrypted interface{} // Data containing sensitive data -> to be sent
}

type HttpRequest struct {
	Method  string              `json:"method"`
	Body    string              `json:"body,omitempty"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type HttpDetails struct {
	HttpResponse HttpResponse
	HttpRequest  HttpRequest
}

// SendRequest sends an HTTP request with optional TLS configuration.
func (hc *client) SendRequest(ctx context.Context, method string, url string, body Data, headers Data, tlsConfigData *TLSConfigData) (details HttpDetails, err error) {
	requestBody := []byte(body.Decrypted.(string))

	// request contains the HTTP request that will be sent.
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(requestBody))

	// requestDetails contains the request details that will be logged.
	requestDetails := HttpRequest{
		URL:     url,
		Body:    body.Encrypted.(string),
		Headers: headers.Encrypted.(map[string][]string),
		Method:  method,
	}

	if err != nil {
		return HttpDetails{
			HttpRequest: requestDetails,
		}, err
	}

	for key, values := range headers.Decrypted.(map[string][]string) {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	// Add the authorization token to the request if it doesn't already exist.
	if _, exists := request.Header[authKey]; !exists && hc.authorizationToken != "" {
		request.Header[authKey] = []string{hc.authorizationToken}
	}

	// Build TLS configuration
	tlsConfig, err := buildTLSConfig(tlsConfigData)
	if err != nil {
		return HttpDetails{
			HttpRequest: requestDetails,
		}, fmt.Errorf("failed to build TLS config: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment, // Use proxy settings from environment
		},
		Timeout: hc.timeout,
	}

	response, err := client.Do(request)
	if err != nil {
		return HttpDetails{
			HttpRequest: requestDetails,
		}, err
	}

	responsebody, err := io.ReadAll(response.Body)
	if err != nil {
		return HttpDetails{
			HttpRequest: requestDetails,
		}, err
	}

	beautifiedResponse := HttpResponse{
		Body:       string(responsebody),
		Headers:    response.Header,
		StatusCode: response.StatusCode,
	}

	err = response.Body.Close()
	if err != nil {
		return HttpDetails{
			HttpRequest: requestDetails,
		}, err
	}

	hc.log.Info(fmt.Sprint("http request sent: ", toJSON(requestDetails)))

	return HttpDetails{
		HttpResponse: beautifiedResponse,
		HttpRequest:  requestDetails,
	}, nil
}

// NewClient returns a new Http Client
func NewClient(log logging.Logger, timeout time.Duration, authorizationToken string) (Client, error) {
	return &client{
		log:                log,
		timeout:            timeout,
		authorizationToken: authorizationToken,
	}, nil
}

// toJSON converts the request to a JSON string.
func toJSON(request HttpRequest) string {
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

// buildTLSConfig builds a tls.Config from TLSConfigData
func buildTLSConfig(data *TLSConfigData) (*tls.Config, error) {
	if data == nil {
		// #nosec G402 -- Empty TLS config is valid when no TLS configuration is provided
		return &tls.Config{}, nil
	}

	tlsConfig := &tls.Config{
		// #nosec G402 - InsecureSkipVerify is configurable by the user
		InsecureSkipVerify: data.InsecureSkipVerify,
	}

	// Load CA bundle if provided
	if len(data.CABundle) > 0 {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(data.CABundle) {
			return nil, fmt.Errorf("failed to parse CA bundle")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate and key if provided
	if len(data.ClientCert) > 0 && len(data.ClientKey) > 0 {
		cert, err := tls.X509KeyPair(data.ClientCert, data.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
