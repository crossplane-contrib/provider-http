package v1alpha1

func (d *DesposibleRequest) SetStatusCode(statusCode int) {
	d.Status.Response.StatusCode = statusCode
}

func (d *DesposibleRequest) SetHeaders(headers map[string][]string) {
	d.Status.Response.Headers = headers
}

func (d *DesposibleRequest) SetBody(body string) {
	d.Status.Response.Body = body
}

func (d *DesposibleRequest) SetSynced(synced bool) {
	d.Status.Synced = synced
	d.Status.Failed = 0
	d.Status.Error = ""
}

func (d *DesposibleRequest) SetError(err error) {
	d.Status.Failed++
	d.Status.Synced = true
	if err != nil {
		d.Status.Error = err.Error()
	}
}

func (d *DesposibleRequest) SetRequestDetails(url, method, body string, headers map[string][]string) {
	d.Status.RequestDetails.Body = body
	d.Status.RequestDetails.URL = url
	d.Status.RequestDetails.Headers = headers
	d.Status.RequestDetails.Method = method
}
