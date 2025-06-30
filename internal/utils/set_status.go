package utils

import (
	"context"

	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ErrFailedToSetStatus = "failed to update status"
)

// SetRequestStatusFunc is a function that sets the status of a resource.
type SetRequestStatusFunc func()

// RequestResource is a struct that holds the resource, request context, http response, http request, and local client.
type RequestResource struct {
	Resource               client.Object
	RequestContext         context.Context
	AuthenticationResponse httpClient.HttpResponse
	HttpResponse           httpClient.HttpResponse
	HttpRequest            httpClient.HttpRequest
	LocalClient            client.Client
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

func (rr *RequestResource) SetAuthenticationResponse() SetRequestStatusFunc {
	return func() {
		if resp, ok := rr.Resource.(AuthenticationResponseSetter); ok {
			if rr.AuthenticationResponse.Body != "" {
				resp.SetBody(rr.AuthenticationResponse.Body)
			}
			if len(rr.AuthenticationResponse.Headers) > 0 {
				resp.SetHeaders(rr.AuthenticationResponse.Headers)
			}
			if rr.AuthenticationResponse.StatusCode != 0 {
				resp.SetStatusCode(rr.AuthenticationResponse.StatusCode)
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

func (rr *RequestResource) SetLastReconcileTime() SetRequestStatusFunc {
	return func() {
		if lastReconcileTimeSetter, ok := rr.Resource.(LastReconcileTimeSetter); ok {
			lastReconcileTimeSetter.SetLastReconcileTime()
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

// ResponseSetter is an interface that defines the methods to set the status code, headers, and body of a resource.
type ResponseSetter interface {
	SetStatusCode(statusCode int)
	SetHeaders(headers map[string][]string)
	SetBody(body string)
}

// CacheSetter is an interface that defines the method to set the cache of a resource.
type CacheSetter interface {
	SetCache(statusCode int, headers map[string][]string, body string)
}

// SyncedSetter is an interface that defines the method to set the synced status of a resource.
type SyncedSetter interface {
	SetSynced(synced bool)
}

// ErrorSetter is an interface that defines the method to set the error of a resource.
type ErrorSetter interface {
	SetError(err error)
}

// ResetFailures is an interface that defines the method to reset the failures of a resource.
type ResetFailures interface {
	ResetFailures()
}

// LastReconcileTimeSetter is an interface that defines the method to set the last reconcile time of a resource.
type LastReconcileTimeSetter interface {
	SetLastReconcileTime()
}

// RequestDetailsSetter is an interface that defines the method to set the request details of a resource.
type RequestDetailsSetter interface {
	SetRequestDetails(url, method, body string, headers map[string][]string)
}

type AuthenticationResponseSetter interface {
	SetStatusCode(statusCode int)
	SetHeaders(headers map[string][]string)
	SetBody(body string)
}

// SetRequestResourceStatus sets the status of a resource.
func SetRequestResourceStatus(rr RequestResource, statusFuncs ...SetRequestStatusFunc) error {
	for _, updateStatusFunc := range statusFuncs {
		updateStatusFunc()
	}

	return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)
}
