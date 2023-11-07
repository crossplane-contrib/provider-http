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
}

func (d *DesposibleRequest) SetError(err error) {
	d.Status.Failed++
	d.Status.Error = err.Error()
	d.SetSynced(true)
}
