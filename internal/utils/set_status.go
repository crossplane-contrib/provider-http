package utils

import (
	"context"

	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ErrFailedToSetStatus = "failed to update status"
)

type SetRequestStatusFunc func()

type RequestResource struct {
	Resource       client.Object
	RequestContext context.Context
	HttpResponse   httpClient.HttpResponse
	HttpRequest    httpClient.HttpRequest
	LocalClient    client.Client
}

func (rr *RequestResource) SetStatusCode() SetRequestStatusFunc {
	return func() {
		if resp, ok := rr.Resource.(ResponseSetter); ok {
			if rr.HttpResponse.StatusCode != 0 {
				resp.SetStatusCode(rr.HttpResponse.StatusCode)
			}
		}
	}
}

func (rr *RequestResource) SetHeaders() SetRequestStatusFunc {
	return func() {
		if resp, ok := rr.Resource.(ResponseSetter); ok {
			if rr.HttpResponse.Headers != nil {
				resp.SetHeaders(rr.HttpResponse.Headers)
			}
		}
	}
}

func (rr *RequestResource) SetBody() SetRequestStatusFunc {
	return func() {
		if resp, ok := rr.Resource.(ResponseSetter); ok {
			if rr.HttpResponse.Body != "" {
				resp.SetBody(rr.HttpResponse.Body)
			}
		}
	}
}

func (rr *RequestResource) SetRequestDetails() SetRequestStatusFunc {
	return func() {
		if resp, ok := rr.Resource.(RequestDetailsSetter); ok {
			if rr.HttpRequest.Method != "" {
				resp.SetRequestDetails(rr.HttpRequest.URL, rr.HttpRequest.Method, rr.HttpRequest.Body, rr.HttpRequest.Headers)
			}
		}
	}
}

func (rr *RequestResource) SetSynced() SetRequestStatusFunc {
	return func() {
		if synced, ok := rr.Resource.(SyncedSetter); ok {
			synced.SetSynced(true)
		}
	}
}

func (rr *RequestResource) SetCache() SetRequestStatusFunc {
	return func() {
		if cached, ok := rr.Resource.(CacheSetter); ok {
			cached.SetCache(rr.HttpResponse.StatusCode, rr.HttpResponse.Headers, rr.HttpResponse.Body)
		}
	}
}

func (rr *RequestResource) SetError(err error) SetRequestStatusFunc {
	return func() {
		if resourceSetErr, ok := rr.Resource.(ErrorSetter); ok {
			resourceSetErr.SetError(err)
		}
	}
}

func (rr *RequestResource) ResetFailures() SetRequestStatusFunc {
	return func() {
		if resetter, ok := rr.Resource.(ResetFailures); ok {
			resetter.ResetFailures()
		}
	}
}

type ResponseSetter interface {
	SetStatusCode(statusCode int)
	SetHeaders(headers map[string][]string)
	SetBody(body string)
}

type CacheSetter interface {
	SetCache(statusCode int, headers map[string][]string, body string)
}

type SyncedSetter interface {
	SetSynced(synced bool)
}

type ErrorSetter interface {
	SetError(err error)
}

type ResetFailures interface {
	ResetFailures()
}

type RequestDetailsSetter interface {
	SetRequestDetails(url, method, body string, headers map[string][]string)
}

func SetRequestResourceStatus(rr RequestResource, statusFuncs ...SetRequestStatusFunc) error {
	for _, updateStatusFunc := range statusFuncs {
		updateStatusFunc()
	}

	return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)
}
