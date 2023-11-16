package utils

import (
	"context"

	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errFailedToSetStatusCode = "failed to update status code"
	errFailedToSetHeaders    = "failed to update headers"
	errFailedToSetBody       = "failed to update body"
)

type SetRequestStatusFunc func() error

type RequestResource struct {
	Resource       client.Object
	RequestContext context.Context
	HttpResponse   httpClient.HttpResponse
	LocalClient    client.Client
}

func (rr *RequestResource) SetStatusCode() SetRequestStatusFunc {
	return func() error {
		if resp, ok := rr.Resource.(ResponseSetter); ok {
			if rr.HttpResponse.StatusCode != 0 {
				resp.SetStatusCode(rr.HttpResponse.StatusCode)
				return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)
			}
		}
		return nil
	}
}

func (rr *RequestResource) SetHeaders() SetRequestStatusFunc {
	return func() error {
		if resp, ok := rr.Resource.(ResponseSetter); ok {
			if rr.HttpResponse.Headers != nil {
				resp.SetHeaders(rr.HttpResponse.Headers)
				return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)
			}
		}
		return nil
	}
}

func (rr *RequestResource) SetBody() SetRequestStatusFunc {
	return func() error {
		if resp, ok := rr.Resource.(ResponseSetter); ok {
			if rr.HttpResponse.ResponseBody != "" {
				resp.SetBody(rr.HttpResponse.ResponseBody)
				return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)
			}
		}
		return nil
	}
}

func (rr *RequestResource) SetSynced() SetRequestStatusFunc {
	return func() error {
		if synced, ok := rr.Resource.(SyncedSetter); ok {
			synced.SetSynced(true)
			return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)
		}
		return nil
	}
}

func (rr *RequestResource) SetError(err error) SetRequestStatusFunc {
	return func() error {
		if resourceSetErr, ok := rr.Resource.(ErrorSetter); ok {
			resourceSetErr.SetError(err)
			return rr.LocalClient.Status().Update(rr.RequestContext, rr.Resource)

		}
		return nil
	}
}

type ResponseSetter interface {
	SetStatusCode(statusCode int)
	SetHeaders(headers map[string][]string)
	SetBody(body string)
}

type SyncedSetter interface {
	SetSynced(synced bool)
}

type ErrorSetter interface {
	SetError(err error)
}

func SetRequestResourceStatus(rr RequestResource, statusFuncs ...SetRequestStatusFunc) error {
	for _, fn := range statusFuncs {
		if err := fn(); err != nil {
			return errors.Wrap(err, errFailedToSetStatusCode)
		}
	}

	return nil
}
