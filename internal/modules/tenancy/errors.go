package tenancy

import "errors"

var (
	ErrInvalidOrganizationInput = errors.New("tenancy invalid organization input")
	ErrOrganizationSlugExists   = errors.New("tenancy organization slug exists")
	ErrInvalidAuthorization     = errors.New("tenancy invalid authorization")
	ErrTenantAccessDenied       = errors.New("tenancy access denied")
)
