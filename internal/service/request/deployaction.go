package request

import (
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestmapping"
	"github.com/crossplane-contrib/provider-http/internal/service/request/statushandler"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
)

// DeployAction executes the action based on the given Request resource and Mapping configuration.
func DeployAction(svcCtx *service.ServiceContext, crCtx *service.RequestCRContext, action string) error {
	spec := crCtx.Spec()
	mapping, err := requestmapping.GetMapping(spec, action, svcCtx.Logger)
	if err != nil {
		svcCtx.Logger.Info(err.Error())
		return nil
	}

	requestDetails, err := requestgen.GenerateValidRequestDetails(svcCtx, crCtx, mapping)
	if err != nil {
		return err
	}

	details, sendErr := svcCtx.HTTP.SendRequest(svcCtx.Ctx, requestmapping.GetEffectiveMethod(mapping), requestDetails.Url, requestDetails.Body, requestDetails.Headers, spec.GetInsecureSkipTLSVerify())

	// Skip secret injection during deletion to avoid cross-namespace owner reference issues
	if !meta.WasDeleted(crCtx.GetCR()) {
		// Apply response data to secrets and update CR status
		secretConfigs := spec.GetSecretInjectionConfigs()
		datapatcher.ApplyResponseDataToSecrets(svcCtx.Ctx, svcCtx.LocalKube, svcCtx.Logger, &details.HttpResponse, secretConfigs, crCtx.GetCR())
	} else {
		svcCtx.Logger.Debug("Request is being deleted, skipping secret injection")
	}

	statusHandler, err := statushandler.NewStatusHandler(svcCtx, crCtx, details, sendErr)
	if err != nil {
		return err
	}

	return statusHandler.SetRequestStatus()
}
