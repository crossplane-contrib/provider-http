package http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Client is the interface to interact with Http
type Client interface {
	SendRequest(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp HttpResponse, err error)
}

type client struct {
	log     logging.Logger
	timeout time.Duration
}

type HttpResponse struct {
	ResponseBody string
	Headers      map[string][]string
	StatusCode   int
}

func (hc *client) SendRequest(ctx context.Context, method string, url string, body string, headers map[string][]string) (resp HttpResponse, err error) {
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
		ResponseBody: string(responsebody),
		Headers:      response.Header,
		StatusCode:   response.StatusCode,
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
