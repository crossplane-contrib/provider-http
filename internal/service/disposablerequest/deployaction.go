package disposablerequest

import (
	"context"
	"fmt"
	"strconv"

	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	datapatcher "github.com/crossplane-contrib/provider-http/internal/data-patcher"
	"github.com/crossplane-contrib/provider-http/internal/utils"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errResponseFormat = "Response does not match the expected format, retries limit "
)

// DeployAction sends the HTTP request defined in the DisposableRequest resource and updates its status based on the response.
func DeployAction(ctx context.Context, spec interfaces.SimpleHTTPRequestSpec, rollbackPolicy interfaces.RollbackAware, status interfaces.DisposableRequestStatus, obj client.Object, localKube client.Client, logger logging.Logger, httpCli httpClient.Client) error {
	if status.GetSynced() {
		logger.Debug("Resource is already synced, skipping deployment action")
		return nil
	}

	// Check if retries limit has been reached
	if utils.RollBackEnabled(rollbackPolicy.GetRollbackRetriesLimit()) && utils.RetriesLimitReached(status.GetFailed(), rollbackPolicy.GetRollbackRetriesLimit()) {
		logger.Debug("Retries limit reached, not retrying anymore")
		return nil
	}

	details, httpRequestErr := sendHttpRequest(ctx, spec, localKube, logger, httpCli)

	resource, err := prepareRequestResource(ctx, obj, details, localKube)
	if err != nil {
		return err
	}

	// Handle HTTP request errors first
	if httpRequestErr != nil {
		return handleHttpRequestError(resource, httpRequestErr)
	}

	return handleHttpResponse(ctx, spec, rollbackPolicy, details.HttpResponse, resource, obj.(metav1.Object), localKube, logger)
}

// sendHttpRequest sends the HTTP request with sensitive data patched
func sendHttpRequest(ctx context.Context, spec interfaces.SimpleHTTPRequestSpec, localKube client.Client, logger logging.Logger, httpCli httpClient.Client) (httpClient.HttpDetails, error) {
	sensitiveBody, err := datapatcher.PatchSecretsIntoString(ctx, localKube, spec.GetBody(), logger)
	if err != nil {
		return httpClient.HttpDetails{}, err
	}

	sensitiveHeaders, err := datapatcher.PatchSecretsIntoHeaders(ctx, localKube, spec.GetHeaders(), logger)
	if err != nil {
		return httpClient.HttpDetails{}, err
	}

	bodyData := httpClient.Data{Encrypted: spec.GetBody(), Decrypted: sensitiveBody}
	headersData := httpClient.Data{Encrypted: spec.GetHeaders(), Decrypted: sensitiveHeaders}
	details, err := httpCli.SendRequest(ctx, spec.GetMethod(), spec.GetURL(), bodyData, headersData, spec.GetInsecureSkipTLSVerify())

	return details, err
}

// prepareRequestResource creates and initializes the RequestResource
func prepareRequestResource(ctx context.Context, obj client.Object, details httpClient.HttpDetails, localKube client.Client) (*utils.RequestResource, error) {
	resource := &utils.RequestResource{
		Resource:       obj,
		RequestContext: ctx,
		HttpResponse:   details.HttpResponse,
		LocalClient:    localKube,
		HttpRequest:    details.HttpRequest,
	}

	// Get the latest version of the resource before updating
	if err := localKube.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj); err != nil {
		return nil, errors.Wrap(err, "failed to get the latest version of the resource")
	}

	return resource, nil
}

// handleHttpResponse processes the HTTP response and updates resource status accordingly
func handleHttpResponse(ctx context.Context, spec interfaces.SimpleHTTPRequestSpec, rollbackPolicy interfaces.RollbackAware, sensitiveResponse httpClient.HttpResponse, resource *utils.RequestResource, obj metav1.Object, localKube client.Client, logger logging.Logger) error {
	// Handle HTTP error status codes
	if utils.IsHTTPError(resource.HttpResponse.StatusCode) {
		return handleHttpErrorStatus(spec, resource)
	}

	// Handle response validation
	return handleResponseValidation(ctx, spec, rollbackPolicy, sensitiveResponse, resource, obj, localKube, logger)
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
func handleResponseValidation(ctx context.Context, spec interfaces.SimpleHTTPRequestSpec, rollbackPolicy interfaces.RollbackAware, sensitiveResponse httpClient.HttpResponse, resource *utils.RequestResource, obj metav1.Object, localKube client.Client, logger logging.Logger) error {
	isExpectedResponse, err := IsResponseAsExpected(spec, sensitiveResponse)
	if err != nil {
		return err
	}

	if isExpectedResponse {
		datapatcher.ApplyResponseDataToSecrets(ctx, localKube, logger, &resource.HttpResponse, spec.GetSecretInjectionConfigs(), obj)
		return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(), resource.SetSynced(), resource.SetRequestDetails())
	}

	limit := utils.GetRollbackRetriesLimit(rollbackPolicy.GetRollbackRetriesLimit())
	return utils.SetRequestResourceStatus(*resource, resource.SetStatusCode(), resource.SetLastReconcileTime(), resource.SetHeaders(), resource.SetBody(),
		resource.SetError(errors.New(errResponseFormat+fmt.Sprint(limit))), resource.SetRequestDetails())
}
