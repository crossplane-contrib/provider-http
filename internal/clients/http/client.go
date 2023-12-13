package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Client is the interface to interact with Http
type Client interface {
	SendRequest(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp HttpResponse, err error)
}

type client struct {
	log     logging.Logger
	timeout time.Duration
}

type HttpResponse struct {
	Body       string
	Headers    map[string][]string
	StatusCode int
	Method     string
}

func (hc *client) SendRequest(ctx context.Context, method string, url string, body string, headers map[string][]string, skipTLSVerify bool) (resp HttpResponse, err error) {
	requestBody := []byte(body)
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(requestBody))

	if err != nil {
		return HttpResponse{}, err
	}

	for key, values := range headers {
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
		return HttpResponse{}, err
	}

	responsebody, err := io.ReadAll(response.Body)
	if err != nil {
		return HttpResponse{}, err
	}

	beautifiedResponse := HttpResponse{
		Body:       string(responsebody),
		Headers:    response.Header,
		StatusCode: response.StatusCode,
		Method:     response.Request.Method,
	}

	err = response.Body.Close()
	if err != nil {
		return HttpResponse{}, err
	}

	return beautifiedResponse, nil
}

// NewClient returns a new Http Client
func NewClient(log logging.Logger, timeout time.Duration) (Client, error) {
	return &client{
		log:     log,
		timeout: timeout,
	}, nil
}
