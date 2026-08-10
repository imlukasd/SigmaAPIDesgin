package tenancy

import (
	"context"

	"corebe.local/api/internal/platform/database"
)

type TransactionManager interface {
	WithinTx(ctx context.Context, fn func(context.Context, database.DBTX) error) error
}

type Authorizer interface {
	Authorize(ctx context.Context, request AuthorizeRequest) (AuthorizationResult, error)
}

type OrganizationService interface {
	CreateOrganization(ctx context.Context, request CreateOrganizationRequest) (CreateOrganizationResult, error)
}

type Store interface {
	CreateOrganization(ctx context.Context, db database.DBTX, params CreateOrganizationParams) (Organization, error)
	GetOrganizationByID(ctx context.Context, db database.DBTX, id string) (Organization, error)
	CreateMembership(ctx context.Context, db database.DBTX, params CreateMembershipParams) (Membership, error)
	GetMembership(ctx context.Context, db database.DBTX, organizationID string, userID string) (Membership, error)
}
