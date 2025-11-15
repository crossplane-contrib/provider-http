package observe

import (
	"net/http"

	"github.com/crossplane-contrib/provider-http/apis/common"
	"github.com/crossplane-contrib/provider-http/apis/interfaces"
	httpClient "github.com/crossplane-contrib/provider-http/internal/clients/http"
	"github.com/crossplane-contrib/provider-http/internal/service"
	"github.com/pkg/errors"
)

const (
	ErrObjectNotFound = "object wasn't found"
)

// isDeletedCheck is an interface for performing isDeleted checks.
type isDeletedCheck interface {
	Check(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) error
}

// defaultIsRemovedResponseCheck performs a default comparison between the response and desired state.
type defaultIsRemovedResponseCheck struct{}

// Check performs a default comparison between the response and desired state.
func (d *defaultIsRemovedResponseCheck) Check(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) error {
	if details.HttpResponse.StatusCode == http.StatusNotFound {
		return errors.New(ErrObjectNotFound)
	}

	return nil
}

// customIsRemovedResponseCheck performs a custom response check using JQ logic.
type customIsRemovedResponseCheck struct{}

// Check performs a custom response check using JQ logic.
func (c *customIsRemovedResponseCheck) Check(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec, statusReader interfaces.RequestStatusReader, cachedReader interfaces.CachedResponse, details httpClient.HttpDetails, responseErr error) error {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return errors.New("spec does not support custom response checks")
	}

	logic := responseCheckAware.GetIsRemovedCheck().GetLogic()
	customCheck := &customCheck{}

	isRemoved, err := customCheck.check(svcCtx, spec, details, logic)
	if err != nil {
		return errors.Errorf(errExpectedFormat, "isRemovedCheck", err.Error())
	} else if isRemoved {
		return errors.New(ErrObjectNotFound)
	}

	return nil
}

// isRemovedCheckFactoryMap is a map that associates each check type with its corresponding factory function.
var isRemovedCheckFactoryMap = map[string]func() isDeletedCheck{
	common.ExpectedResponseCheckTypeDefault: func() isDeletedCheck {
		return &defaultIsRemovedResponseCheck{}
	},
	common.ExpectedResponseCheckTypeCustom: func() isDeletedCheck {
		return &customIsRemovedResponseCheck{}
	},
}

// GetIsRemovedResponseCheck uses a map to select and return the appropriate ResponseCheck.
func GetIsRemovedResponseCheck(svcCtx *service.ServiceContext, spec interfaces.MappedHTTPRequestSpec) isDeletedCheck {
	responseCheckAware, ok := spec.(interfaces.ResponseCheckAware)
	if !ok {
		return isRemovedCheckFactoryMap[common.ExpectedResponseCheckTypeDefault]()
	}

	if factory, ok := isRemovedCheckFactoryMap[responseCheckAware.GetIsRemovedCheck().GetType()]; ok {
		return factory()
	}
	return isRemovedCheckFactoryMap[common.ExpectedResponseCheckTypeDefault]()
}
