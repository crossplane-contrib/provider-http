package request

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/requestschedule"
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

	details, sendErr := svcCtx.HTTP.SendRequest(svcCtx.Ctx, requestmapping.GetEffectiveMethod(mapping), requestDetails.Url, requestDetails.Body, requestDetails.Headers, svcCtx.TLSConfigData)

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

	// Persist rate-limit deferral for create and update actions too, so the next
	// managed reconcile cannot immediately repeat a server-throttled request.
	if details.HttpResponse.StatusCode == 429 {
		now := time.Now().UTC()
		lastRequest := metav1.NewTime(now)
		deadline, _ := requestschedule.RateLimitDeadline(now, details.HttpResponse.Headers, crCtx.Status().GetFailed()+1, crCtx.GetCR())
		rateLimitUntil := metav1.NewTime(deadline)
		desiredHash, hashErr := requestschedule.DesiredStateHash(crCtx.GetCR())
		if hashErr != nil {
			return hashErr
		}
		statusHandler.SetScheduling(&lastRequest, nil, &rateLimitUntil, desiredHash)
	}

	return statusHandler.SetRequestStatus()
}
