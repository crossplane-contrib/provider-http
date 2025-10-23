package v1alpha2

import "time"

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
