package disposablerequest

import (
	"fmt"
	"strconv"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	errResponseFormat = "Response does not match the expected format, retries limit "
)

// DeployAction sends the HTTP request defined in the DisposableRequest resource and updates its status based on the response.
func DeployAction(svcCtx *service.ServiceContext, crCtx *service.DisposableRequestCRContext) error {
	spec := crCtx.Spec()
	status := crCtx.Status()
	rollbackPolicy := crCtx.RollbackPolicy()
	reconciliationPolicy := crCtx.ReconciliationPolicy()

	if status.GetSynced() && (reconciliationPolicy == nil || !reconciliationPolicy.GetShouldLoopInfinitely()) {
		svcCtx.Logger.Debug("Resource is already synced, skipping deployment action")
		return nil
	}

	// Check if retries limit has been reached
	if utils.RollBackEnabled(rollbackPolicy.GetRollbackRetriesLimit()) && utils.RetriesLimitReached(status.GetFailed(), rollbackPolicy.GetRollbackRetriesLimit()) {
		svcCtx.Logger.Debug("Retries limit reached, not retrying anymore")
		return nil
	}

	details, httpRequestErr := sendHttpRequest(svcCtx, spec)

	resource, err := prepareRequestResource(svcCtx, crCtx, details)
	if err != nil {
		return err
	}

	// Handle HTTP request errors first
	if httpRequestErr != nil {
		return handleHttpRequestError(resource, httpRequestErr)
	}

	return handleHttpResponse(svcCtx, crCtx, details.HttpResponse, resource)
}

// sendHttpRequest sends the HTTP request with sensitive data patched
func sendHttpRequest(svcCtx *service.ServiceContext, spec interfaces.SimpleHTTPRequestSpec) (httpClient.HttpDetails, error) {
	sensitiveBody, err := datapatcher.PatchSecretsIntoString(svcCtx.Ctx, svcCtx.LocalKube, spec.GetBody(), svcCtx.Logger)
	if err != nil {
		return httpClient.HttpDetails{}, err
	}

	sensitiveHeaders, err := datapatcher.PatchSecretsIntoHeaders(svcCtx.Ctx, svcCtx.LocalKube, spec.GetHeaders(), svcCtx.Logger)
	if err != nil {
		return httpClient.HttpDetails{}, err
	}

	bodyData := httpClient.Data{Encrypted: spec.GetBody(), Decrypted: sensitiveBody}
	headersData := httpClient.Data{Encrypted: spec.GetHeaders(), Decrypted: sensitiveHeaders}
	details, err := svcCtx.HTTP.SendRequest(svcCtx.Ctx, spec.GetMethod(), spec.GetURL(), bodyData, headersData, svcCtx.TLSConfigData)

	return details, err
}

// prepareRequestResource creates and initializes the RequestResource
func prepareRequestResource(svcCtx *service.ServiceContext, crCtx *service.DisposableRequestCRContext, details httpClient.HttpDetails) (*utils.RequestResource, error) {
	obj := crCtx.GetCR()
	resource := &utils.RequestResource{
		StatusWriter:   crCtx.StatusWriter(),
		Resource:       obj,
		RequestContext: svcCtx.Ctx,
		HttpResponse:   details.HttpResponse,
		LocalClient:    svcCtx.LocalKube,
		HttpRequest:    details.HttpRequest,
	}

	// Get the latest version of the resource before updating
	if err := svcCtx.LocalKube.Get(svcCtx.Ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj); err != nil {
		return nil, errors.Wrap(err, "failed to get the latest version of the resource")
	}

	return resource, nil
}

// handleHttpResponse processes the HTTP response and updates resource status accordingly
func handleHttpResponse(svcCtx *service.ServiceContext, crCtx *service.DisposableRequestCRContext, sensitiveResponse httpClient.HttpResponse, resource *utils.RequestResource) error {
	spec := crCtx.Spec()
	rollbackPolicy := crCtx.RollbackPolicy()
	obj := crCtx.GetCR()

	// Handle HTTP error status codes
	if utils.IsHTTPError(resource.HttpResponse.StatusCode) {
		return handleHttpErrorStatus(spec, resource)
	}

	// Handle response validation
	return handleResponseValidation(svcCtx, spec, rollbackPolicy, sensitiveResponse, resource, obj.(metav1.Object))
}

// handleHttpRequestError handles cases where the HTTP request itself failed
func handleHttpRequestError(resource *utils.RequestResource, httpRequestErr error) error {
	setErr := resource.SetError(httpRequestErr)
	if settingError := utils.SetRequestResourceStatus(*resource, setErr, resource.SetLastReconcileTime(), resource.SetRequestDetails()); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}
	return httpRequestErr
}

// handleHttpErrorStatus handles HTTP error status codes
func handleHttpErrorStatus(spec interfaces.SimpleHTTPRequestSpec, resource *utils.RequestResource) error {
	if settingError := utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(), resource.SetRequestDetails(), resource.SetError(nil)); settingError != nil {
		return errors.Wrap(settingError, utils.ErrFailedToSetStatus)
	}

	return errors.Errorf(utils.ErrStatusCode, spec.GetMethod(), strconv.Itoa(resource.HttpResponse.StatusCode))
}

// handleResponseValidation validates the response and updates status accordingly
func handleResponseValidation(svcCtx *service.ServiceContext, spec interfaces.SimpleHTTPRequestSpec, rollbackPolicy interfaces.RollbackAware, sensitiveResponse httpClient.HttpResponse, resource *utils.RequestResource, obj metav1.Object) error {
	isExpectedResponse, err := IsResponseAsExpected(spec, sensitiveResponse)
	if err != nil {
		return err
	}

	if isExpectedResponse {
		datapatcher.ApplyResponseDataToSecrets(svcCtx.Ctx, svcCtx.LocalKube, svcCtx.Logger, &resource.HttpResponse, spec.GetSecretInjectionConfigs(), obj)
		return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(), resource.SetSynced(), resource.SetRequestDetails())
	}

	limit := utils.GetRollbackRetriesLimit(rollbackPolicy.GetRollbackRetriesLimit())
	return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(),
		resource.SetError(errors.New(errResponseFormat+fmt.Sprint(limit))), resource.SetRequestDetails())
}
