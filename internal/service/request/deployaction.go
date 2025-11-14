package request

import (
	"context"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/service/request/statushandler"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeployAction executes the action based on the given Request resource and Mapping configuration.
func DeployAction(ctx context.Context, cr interfaces.RequestResource, action string, localKube client.Client, logger logging.Logger, httpClient httpClient.Client) error {
	spec := cr.GetSpec()
	mapping, err := requestmapping.GetMapping(spec, action, logger)
	if err != nil {
		logger.Info(err.Error())
		return nil
	}

	requestDetails, err := requestgen.GenerateValidRequestDetails(ctx, spec, mapping, cr.GetResponse(), cr.GetCachedResponse(), localKube, logger)
	if err != nil {
		return err
	}

	details, sendErr := httpClient.SendRequest(ctx, requestmapping.GetEffectiveMethod(mapping), requestDetails.Url, requestDetails.Body, requestDetails.Headers, spec.GetInsecureSkipTLSVerify())

	// Apply response data to secrets and update CR status
	secretConfigs := spec.GetSecretInjectionConfigs()
	datapatcher.ApplyResponseDataToSecrets(ctx, localKube, logger, &details.HttpResponse, secretConfigs, cr)

	statusHandler, err := statushandler.NewStatusHandler(ctx, cr, spec, details, sendErr, localKube, logger)
	if err != nil {
		return err
	}

	return statusHandler.SetRequestStatus()
}
