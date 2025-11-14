package observe

import (
	"context"
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ErrObjectNotFound = "object wasn't found"
)

// isDeletedCheck is an interface for performing isDeleted checks.
type isDeletedCheck interface {
	Check(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) error
}

// defaultIsRemovedResponseCheck performs a default comparison between the response and desired state.
type defaultIsRemovedResponseCheck struct {
	localKube client.Client
	logger    logging.Logger
	http      httpClient.Client
}

// Check performs a default comparison between the response and desired state.
func (d *defaultIsRemovedResponseCheck) Check(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) error {
	if details.HttpResponse.StatusCode == http.StatusNotFound {
		return errors.New(ErrObjectNotFound)
	}

	return nil
}

// // customIsRemovedResponseCheck performs a custom response check using JQ logic.
type customIsRemovedResponseCheck struct {
	localKube client.Client
	logger    logging.Logger
	http      httpClient.Client
}

// Check performs a custom response check using JQ logic.
func (c *customIsRemovedResponseCheck) Check(ctx context.Context, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) error {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return errors.New("spec does not support custom response checks")
	}

	logic := responseCheckAware.GetIsRemovedCheck().GetLogic()
	customCheck := &customCheck{localKube: c.localKube, logger: c.logger, http: c.http}

	isRemoved, err := customCheck.check(ctx, spec, details, logic)
	if err != nil {
		return errors.Errorf(errExpectedFormat, "isRemovedCheck", err.Error())
	} else if isRemoved {
		return errors.New(ErrObjectNotFound)
	}

	return nil
}

// isRemovedCheckFactoryMap is a map that associates each check type with its corresponding factory function.
var isRemovedCheckFactoryMap = map[string]func(localKube client.Client, logger logging.Logger, http httpClient.Client) isDeletedCheck{
	common.ExpectedResponseCheckTypeDefault: func(localKube client.Client, logger logging.Logger, http httpClient.Client) isDeletedCheck {
		return &defaultIsRemovedResponseCheck{localKube: localKube, logger: logger, http: http}
	},
	common.ExpectedResponseCheckTypeCustom: func(localKube client.Client, logger logging.Logger, http httpClient.Client) isDeletedCheck {
		return &customIsRemovedResponseCheck{localKube: localKube, logger: logger, http: http}
	},
}

// GetIsRemovedResponseCheck uses a map to select and return the appropriate ResponseCheck.
func GetIsRemovedResponseCheck(spec interfaces.MappedHTTPRequestSpec, localKube client.Client, logger logging.Logger, http httpClient.Client) isDeletedCheck {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return isRemovedCheckFactoryMap[common.ExpectedResponseCheckTypeDefault](localKube, logger, http)
	}

	if factory, ok := isRemovedCheckFactoryMap[responseCheckAware.GetIsRemovedCheck().GetType()]; ok {
		return factory(localKube, logger, http)
	}
	return isRemovedCheckFactoryMap[common.ExpectedResponseCheckTypeDefault](localKube, logger, http)
}
