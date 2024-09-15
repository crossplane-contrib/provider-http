package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Client is the interface to interact with Http
type Client interface {
	SendRequest(ctx context.Context, method string, url string, body Data, headers Data) (resp HttpDetails, err error)
}

type client struct {
	client http.Client
	log    logging.Logger
}

type HttpResponse struct {
	Body       string              `json:"body"`
	Headers    map[string][]string `json:"headers"`
	StatusCode int                 `json:"statusCode"`
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

func (hc *client) SendRequest(ctx context.Context, method string, url string, body Data, headers Data) (details HttpDetails, err error) {
	requestBody := []byte(body.Decrypted.(string))
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(requestBody))
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

	response, err := hc.client.Do(request)
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
func NewClient(log logging.Logger, timeout time.Duration, certPEMBlock, keyPEMBlock, caPEMBlock []byte, insecureSkipVerify bool) (Client, error) {
	tlsConfig, err := tlsConfig(certPEMBlock, keyPEMBlock, caPEMBlock, insecureSkipVerify)
	if err != nil {
		return nil, err
	}
	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment, // Use proxy settings from environment
		},
		Timeout: timeout,
	}
	return &client{
		client: httpClient,
		log:    log,
	}, nil
}

func toJSON(request HttpRequest) string {
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}

func tlsConfig(certPEMBlock, keyPEMBlock, caPEMBlock []byte, insecureSkipVerify bool) (*tls.Config, error) {
	// #nosec G402
	tlsConfig := &tls.Config{}
	if len(certPEMBlock) > 0 && len(keyPEMBlock) > 0 {
		certificate, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	if len(caPEMBlock) > 0 {
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEMBlock) {
			return nil, errors.New("some error appending the ca.crt")
		}
		tlsConfig.RootCAs = caPool
	}

	tlsConfig.InsecureSkipVerify = insecureSkipVerify

	return tlsConfig, nil
}
