package utils

import (
	"context"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ErrFailedToSetStatus = "failed to update status"
)

// SetRequestStatusFunc is a function that sets the status of a resource.
type SetRequestStatusFunc func()

// RequestResource is a struct that holds the status writer, resource object, request context, http response, http request, and local client.
type RequestResource struct {
	StatusWriter   interfaces.BaseStatusWriter // Common status writer interface
	Resource       client.Object               // Underlying resource for status updates
	RequestContext context.Context
	HttpResponse   httpClient.HttpResponse
	HttpRequest    httpClient.HttpRequest
	LocalClient    client.Client
}

func (rr *RequestResource) SetStatusCode() SetRequestStatusFunc {
	return func() {
		if rr.HttpResponse.StatusCode != 0 {
			rr.StatusWriter.SetStatusCode(rr.HttpResponse.StatusCode)
		}
	}
}

func (rr *RequestResource) SetHeaders() SetRequestStatusFunc {
	return func() {
		if rr.HttpResponse.Headers != nil {
			rr.StatusWriter.SetHeaders(rr.HttpResponse.Headers)
		}
	}
}

func (rr *RequestResource) SetBody() SetRequestStatusFunc {
	return func() {
		if rr.HttpResponse.Body != "" {
			rr.StatusWriter.SetBody(rr.HttpResponse.Body)
		}
	}
}

func (rr *RequestResource) SetRequestDetails() SetRequestStatusFunc {
	return func() {
		if rr.HttpRequest.Method != "" {
			rr.StatusWriter.SetRequestDetails(rr.HttpRequest.URL, rr.HttpRequest.Method, rr.HttpRequest.Body, rr.HttpRequest.Headers)
		}
	}
}

func (rr *RequestResource) SetSynced() SetRequestStatusFunc {
	return func() {
		if synced, ok := rr.StatusWriter.(interfaces.DisposableRequestStatusWriter); ok {
			synced.SetSynced(true)
		}
	}
}

func (rr *RequestResource) SetLastReconcileTime() SetRequestStatusFunc {
	return func() {
		if lastReconcileTimeSetter, ok := rr.StatusWriter.(interfaces.DisposableRequestStatusWriter); ok {
			lastReconcileTimeSetter.SetLastReconcileTime()
		}
	}
}

func (rr *RequestResource) SetCache() SetRequestStatusFunc {
	return func() {
		if cached, ok := rr.StatusWriter.(interfaces.RequestStatusWriter); ok {
			cached.SetCache(rr.HttpResponse.StatusCode, rr.HttpResponse.Headers, rr.HttpResponse.Body)
		}
	}
}

func (rr *RequestResource) SetError(err error) SetRequestStatusFunc {
	return func() {
		rr.StatusWriter.SetError(err)
	}
}

func (rr *RequestResource) ResetFailures() SetRequestStatusFunc {
	return func() {
		if resetter, ok := rr.StatusWriter.(interfaces.RequestStatusWriter); ok {
			resetter.ResetFailures()
		}
	}
}

// SetRequestResourceStatus sets the status of a resource.
func SetRequestResourceStatus(rr RequestResource, statusFuncs ...SetRequestStatusFunc) error {
	for _, updateStatusFunc := range statusFuncs {
		updateStatusFunc()
	}

	return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)
}
