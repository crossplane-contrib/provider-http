package v1alpha1

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
	// TODO (REL): make sure it gets incremented
	d.Status.Failed++
	d.Status.Error = err.Error()
}
