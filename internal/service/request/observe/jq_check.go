package observe

import (
	"fmt"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/jq"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/service/request/requestgen"
	"github.com/crossplane-contrib/provider-http/internal/utils"
)

// responseCheck is an interface for performing response checks.
type responseCheck interface {
	Check(svcCtx *service.ServiceContext, crCtx *service.RequestCRContext, details httpClient.HttpDetails, responseErr error) (bool, error)
}

// customCheck performs a custom response check using JQ logic.
type customCheck struct{}

// Check performs a custom response check using JQ logic.
func (c *customCheck) check(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, details httpClient.HttpDetails, logic string) (bool, error) {
	// Convert response to a map and apply JQ logic
	sensitiveResponse, err := datapatcher.PatchSecretsIntoResponse(svcCtx.Ctx, svcCtx.LocalKube, &details.HttpResponse, svcCtx.Logger)
	if err != nil {
		return false, err
	}

	sensitiveRequestContext := requestgen.GenerateRequestContext(spec, sensitiveResponse)

	jqQuery := utils.NormalizeWhitespace(logic)
	sensitiveJQQuery, err := datapatcher.PatchSecretsIntoString(svcCtx.Ctx, svcCtx.LocalKube, jqQuery, svcCtx.Logger)
	if err != nil {
		return false, err
	}

	isExpected, err := jq.ParseBool(sensitiveJQQuery, sensitiveRequestContext)

	svcCtx.Logger.Debug(fmt.Sprintf("Applying JQ filter %s, result is %v", jqQuery, isExpected))
	if err != nil {
		return false, err
	}

	return isExpected, nil
}
