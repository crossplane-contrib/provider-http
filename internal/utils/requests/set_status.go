package requests

import (
	"context"

	"github.com/arielsepton/provider-http/apis/desposiblerequest/v1alpha1"
	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errFailedToSetStatusCode = "failed to update status code"
	errFailedToSetHeaders    = "failed to update headers"
	errFailedToSetBody       = "failed to update body"
)

func setStatusCode(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse, localKube client.Client) error {
	if res.StatusCode != 0 {
		cr.Status.Response.StatusCode = res.StatusCode
		if err := localKube.Status().Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetStatusCode)
		}
	}

	return nil
}

func setHeaders(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse, localKube client.Client) error {
	if res.Headers != nil {
		cr.Status.Response.Headers = res.Headers
		if err := localKube.Status().Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetHeaders)
		}
	}

	return nil
}

func setBody(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse, localKube client.Client) error {
	if res.ResponseBody != "" {
		cr.Status.Response.Body = res.ResponseBody
		if err := localKube.Status().Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetBody)
		}
	}

	return nil
}

func SetDesposibleRequestStatus(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse, localKube client.Client) error {
	if err := setStatusCode(ctx, cr, res, localKube); err != nil {
		return err
	}

	if err := setHeaders(ctx, cr, res, localKube); err != nil {
		return err
	}

	if err := setBody(ctx, cr, res, localKube); err != nil {
		return err
	}

	cr.Status.Synced = true
	if err := localKube.Status().Update(ctx, cr); err != nil {
		return errors.Wrap(err, errFailedToSetStatusCode)
	}

	return nil
}
