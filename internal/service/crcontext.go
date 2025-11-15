package service

import (
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RequestCRContext wraps a Request CR and provides convenient access to its interfaces.
// This reduces parameter counts by bundling spec, status, and cached response accessors.
type RequestCRContext struct {
	cr interfaces.RequestResource
}

// NewRequestCRContext creates a new context for a Request custom resource.
func NewRequestCRContext(cr interfaces.RequestResource) *RequestCRContext {
	return &RequestCRContext{cr: cr}
}

// GetCR returns the underlying custom resource as a client.Object.
func (c *RequestCRContext) GetCR() client.Object {
	return c.cr
}

// GetRequestResource returns the underlying custom resource with full interface access.
func (c *RequestCRContext) GetRequestResource() interfaces.RequestResource {
	return c.cr
}

// Spec returns the request specification (ForProvider parameters).
func (c *RequestCRContext) Spec() interfaces.MappedHTTPRequestSpec {
	return c.cr.GetSpec()
}

// Status provides access to status reading methods.
func (c *RequestCRContext) Status() interfaces.RequestStatusReader {
	return c.cr
}

// StatusWriter provides access to status modification methods.
func (c *RequestCRContext) StatusWriter() interfaces.RequestStatusWriter {
	return c.cr
}

// CachedResponse provides access to cached response data.
func (c *RequestCRContext) CachedResponse() interfaces.CachedResponse {
	return c.cr
}

// DisposableRequestCRContext wraps a DisposableRequest CR and provides convenient access to its spec, status, and object.
// This reduces parameter counts by bundling spec, rollback policy, status, and object accessors.
type DisposableRequestCRContext struct {
	cr interfaces.DisposableRequestResource
}

// NewDisposableRequestCRContext creates a new context for a DisposableRequest custom resource.
func NewDisposableRequestCRContext(cr interfaces.DisposableRequestResource) *DisposableRequestCRContext {
	return &DisposableRequestCRContext{
		cr: cr,
	}
}

// Spec returns the request specification.
func (c *DisposableRequestCRContext) Spec() interfaces.SimpleHTTPRequestSpec {
	return c.cr.GetSpec()
}

// RollbackPolicy returns the rollback policy configuration.
// The spec also implements RollbackAware for DisposableRequest.
func (c *DisposableRequestCRContext) RollbackPolicy() interfaces.RollbackAware {
	spec := c.cr.GetSpec()

	if rollbackAware, ok := spec.(interfaces.RollbackAware); ok {
		return rollbackAware
	}

	return nil
}

// Status returns the status reader.
func (c *DisposableRequestCRContext) Status() interfaces.DisposableRequestStatusReader {
	return c.cr
}

// StatusWriter provides access to status modification methods.
func (c *DisposableRequestCRContext) StatusWriter() interfaces.DisposableRequestStatusWriter {
	return c.cr
}

// GetCR returns the underlying custom resource as a client.Object.
func (c *DisposableRequestCRContext) GetCR() client.Object {
	return c.cr
}

// GetDisposableRequestResource returns the underlying custom resource with full interface access.
func (c *DisposableRequestCRContext) GetDisposableRequestResource() interfaces.DisposableRequestResource {
	return c.cr
}
