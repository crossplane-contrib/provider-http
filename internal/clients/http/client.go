package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Client is the interface to interact with Http
type Client interface {
	SendRequest(ctx context.Context, method string, url string, body Data, headers Data, skipTLSVerify bool) (resp HttpDetails, err error)
}

type client struct {
	log     logging.Logger
	timeout time.Duration
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

func (hc *client) SendRequest(ctx context.Context, method string, url string, body Data, headers Data, skipTLSVerify bool) (details HttpDetails, err error) {
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

	client := &http.Client{
		Transport: &http.Transport{
			// #nosec G402
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify},
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
func NewClient(log logging.Logger, timeout time.Duration) (Client, error) {
	return &client{
		log:     log,
		timeout: timeout,
	}, nil
}

func toJSON(request HttpRequest) string {
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return ""
	}

	return string(jsonBytes)
}
