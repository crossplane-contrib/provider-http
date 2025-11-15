package disposablerequest

import (
	"context"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/service"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errPatchFromReferencedSecret    = "cannot patch from referenced secret"
	errCheckExpectedResponse        = "failed to check if response is as expected"
	errGetLatestVersion             = "failed to get the latest version of the resource"
	errFailedUpdateStatusConditions = "failed updating status conditions"
)

// ValidateStoredResponse validates the stored response against expected criteria
func ValidateStoredResponse(svcCtx *service.ServiceContext, spec interfaces.SimpleHTTPRequestSpec, status interfaces.DisposableRequestStatusReader, obj metav1.Object) (bool, httpClient.HttpResponse, error) {
	response := status.GetResponse()
	if response == nil || response.GetStatusCode() == 0 {
		return false, httpClient.HttpResponse{}, nil
	}

	sensitiveBody, err := datapatcher.PatchSecretsIntoString(svcCtx.Ctx, svcCtx.LocalKube, response.GetBody(), svcCtx.Logger)
	if err != nil {
		return false, httpClient.HttpResponse{}, errors.Wrap(err, errPatchFromReferencedSecret)
	}

	storedResponse := httpClient.HttpResponse{
		StatusCode: response.GetStatusCode(),
		Headers:    response.GetHeaders(),
		Body:       sensitiveBody,
	}

	isExpected, err := IsResponseAsExpected(spec, storedResponse)
	if err != nil {
		svcCtx.Logger.Debug("Setting error condition due to validation error", "error", err)
		return false, httpClient.HttpResponse{}, errors.Wrap(err, errCheckExpectedResponse)
	}
	if !isExpected {
		svcCtx.Logger.Debug("Response does not match expected criteria")
		return false, httpClient.HttpResponse{}, nil
	}

	return true, storedResponse, nil
}

// CalculateUpToDateStatus determines if the resource should be considered up-to-date
func CalculateUpToDateStatus(reconciliationPolicy interfaces.ReconciliationPolicyAware, rollbackPolicy interfaces.RollbackAware, currentStatus bool) bool {
	// If shouldLoopInfinitely is true, the resource should never be considered up-to-date
	if reconciliationPolicy.GetShouldLoopInfinitely() {
		if rollbackPolicy.GetRollbackRetriesLimit() == nil {
			return false
		}
	}
	return currentStatus
}

// UpdateResourceStatus updates the resource status to Available
func UpdateResourceStatus(ctx context.Context, obj client.Object, localKube client.Client) error {
	if err := localKube.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj); err != nil {
		return errors.Wrap(err, errGetLatestVersion)
	}

	// Type assert to set conditions
	if statusWriter, ok := obj.(interface{ SetConditions(...xpv1.Condition) }); ok {
		statusWriter.SetConditions(xpv1.Available())
		if err := localKube.Status().Update(ctx, obj); err != nil {
			return errors.New(errFailedUpdateStatusConditions)
		}
	}
	return nil
}

// ApplySecretInjectionsFromStoredResponse applies secret injection configurations using the stored response
// This is used when the resource is already synced but secret injection configs may have been updated
func ApplySecretInjectionsFromStoredResponse(svcCtx *service.ServiceContext, spec interfaces.SimpleHTTPRequestSpec, storedResponse httpClient.HttpResponse, obj metav1.Object) {
	svcCtx.Logger.Debug("Applying secret injections from stored response")
	datapatcher.ApplyResponseDataToSecrets(svcCtx.Ctx, svcCtx.LocalKube, svcCtx.Logger, &storedResponse, spec.GetSecretInjectionConfigs(), obj)
}
