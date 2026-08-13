package v1alpha2

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (d *Request) SetStatusCode(statusCode int) {
	d.Status.Response.StatusCode = statusCode
}

func (d *Request) SetHeaders(headers map[string][]string) {
	d.Status.Response.Headers = headers
}

func (d *Request) SetBody(body string) {
	d.Status.Response.Body = body
}

func (d *Request) SetError(err error) {
	d.Status.Failed++
	if err != nil {
		d.Status.Error = err.Error()
	}
}

func (d *Request) ResetFailures() {
	d.Status.Failed = 0
	d.Status.Error = ""
}

func (d *Request) SetRequestDetails(url, method, body string, headers map[string][]string) {
	d.Status.RequestDetails.Body = body
	d.Status.RequestDetails.URL = url
	d.Status.RequestDetails.Headers = headers
	d.Status.RequestDetails.Method = method
}

func (d *Request) SetCache(statusCode int, headers map[string][]string, body string) {
	d.Status.Cache.Response.StatusCode = statusCode
	d.Status.Cache.Response.Headers = headers
	d.Status.Cache.Response.Body = body
	d.Status.Cache.LastUpdated = time.Now().UTC().Format(time.RFC3339)
}

func (d *Request) SetLastRequestTime(t *metav1.Time) {
	d.Status.LastRequestTime = t
}

func (d *Request) SetNextPollTime(t *metav1.Time) {
	d.Status.NextPollTime = t
}

func (d *Request) SetRateLimitUntil(t *metav1.Time) {
	d.Status.RateLimitUntil = t
}

func (d *Request) SetObservedDesiredStateHash(hash string) {
	d.Status.ObservedDesiredStateHash = hash
}
