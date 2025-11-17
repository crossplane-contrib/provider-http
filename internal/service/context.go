package service

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
)

// ServiceContext wraps common dependencies passed to service layer functions.
// This reduces parameter count and makes function signatures more maintainable.
type ServiceContext struct {
	Ctx           context.Context
	LocalKube     client.Client
	Logger        logging.Logger
	HTTP          httpClient.Client
	TLSConfigData *httpClient.TLSConfigData
}

// NewServiceContext creates a new ServiceContext with the provided dependencies.
func NewServiceContext(ctx context.Context, localKube client.Client, logger logging.Logger, httpClient httpClient.Client, tlsConfigData *httpClient.TLSConfigData) *ServiceContext {
	return &ServiceContext{
		Ctx:           ctx,
		LocalKube:     localKube,
		Logger:        logger,
		HTTP:          httpClient,
		TLSConfigData: tlsConfigData,
	}
}
