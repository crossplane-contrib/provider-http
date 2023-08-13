package desposiblerequest

import (
	"context"

	"github.com/arielsepton/provider-http/apis/desposiblerequest/v1alpha1"
	httpClient "github.com/arielsepton/provider-http/internal/clients/http"
	"github.com/pkg/errors"
)

func (c *external) setStatusCode(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse) error {
	if res.StatusCode != 0 {
		cr.Status.Response.StatusCode = res.StatusCode
		if err := c.localKube.Status().Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetStatusCode)
		}
	}

	return nil
}

func (c *external) setHeaders(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse) error {
	if res.Headers != nil {
		cr.Status.Response.Headers = res.Headers
		if err := c.localKube.Status().Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetHeaders)
		}
	}

	return nil
}

func (c *external) setBody(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse) error {
	if res.ResponseBody != "" {
		cr.Status.Response.Body = res.ResponseBody
		if err := c.localKube.Status().Update(ctx, cr); err != nil {
			return errors.Wrap(err, errFailedToSetBody)
		}
	}

	return nil
}

func (c *external) setDesposibleRequestStatus(ctx context.Context, cr *v1alpha1.DesposibleRequest, res httpClient.HttpResponse) error {
	if err := c.setStatusCode(ctx, cr, res); err != nil {
		return err
	}

	if err := c.setHeaders(ctx, cr, res); err != nil {
		return err
	}

	if err := c.setBody(ctx, cr, res); err != nil {
		return err
	}

	cr.Status.Synced = true
	if err := c.localKube.Status().Update(ctx, cr); err != nil {
		return errors.Wrap(err, errFailedToSetStatusCode)
	}

	return nil
}
