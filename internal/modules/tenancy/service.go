package tenancy

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"corebe.local/api/internal/platform/database"
	"corebe.local/api/internal/platform/id"
)

const organizationsSlugUniqueConstraint = "organizations_slug_unique"

var organizationSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Service struct {
	db         *pgxpool.Pool
	transactor TransactionManager
	store      Store
}

type ServiceConfig struct {
	DB         *pgxpool.Pool
	Transactor TransactionManager
	Store      Store
}

type CreateOrganizationRequest struct {
	Name        string
	Slug        string
	OwnerUserID string
}

type CreateOrganizationResult struct {
	Organization Organization
	Owner        Membership
}

type AuthorizeRequest struct {
	OrganizationID string
	UserID         string
	Permission     Permission
}

type AuthorizationResult struct {
	Organization Organization
	Membership   Membership
}

func NewService(cfg ServiceConfig) Service {
	return Service{
		db:         cfg.DB,
		transactor: cfg.Transactor,
		store:      cfg.Store,
	}
}

func (s Service) CreateOrganization(ctx context.Context, request CreateOrganizationRequest) (CreateOrganizationResult, error) {
	name := strings.TrimSpace(request.Name)
	slug := strings.TrimSpace(request.Slug)
	ownerUserID := strings.TrimSpace(request.OwnerUserID)
	if !validOrganizationInput(name, slug, ownerUserID) {
		return CreateOrganizationResult{}, ErrInvalidOrganizationInput
	}

	organizationID, err := id.NewUUID()
	if err != nil {
		return CreateOrganizationResult{}, err
	}

	var result CreateOrganizationResult
	err = s.transactor.WithinTx(ctx, func(ctx context.Context, tx database.DBTX) error {
		organization, err := s.store.CreateOrganization(ctx, tx, CreateOrganizationParams{
			ID:   organizationID,
			Name: name,
			Slug: slug,
		})
		if err != nil {
			return mapCreateOrganizationError(err)
		}

		owner, err := s.store.CreateMembership(ctx, tx, CreateMembershipParams{
			OrganizationID: organization.ID,
			UserID:         ownerUserID,
			Role:           MembershipRoleOwner,
		})
		if err != nil {
			return err
		}

		result = CreateOrganizationResult{
			Organization: organization,
			Owner:        owner,
		}
		return nil
	})
	if err != nil {
		return CreateOrganizationResult{}, err
	}

	return result, nil
}

func (s Service) Authorize(ctx context.Context, request AuthorizeRequest) (AuthorizationResult, error) {
	organizationID := strings.TrimSpace(request.OrganizationID)
	userID := strings.TrimSpace(request.UserID)
	if organizationID == "" || userID == "" || request.Permission == "" {
		return AuthorizationResult{}, ErrInvalidAuthorization
	}

	organization, err := s.store.GetOrganizationByID(ctx, s.db, organizationID)
	if err != nil {
		return AuthorizationResult{}, mapAuthorizationReadError(err)
	}
	if organization.Status != OrganizationStatusActive {
		return AuthorizationResult{}, ErrTenantAccessDenied
	}

	membership, err := s.store.GetMembership(ctx, s.db, organizationID, userID)
	if err != nil {
		return AuthorizationResult{}, mapAuthorizationReadError(err)
	}
	if membership.Status != MembershipStatusActive {
		return AuthorizationResult{}, ErrTenantAccessDenied
	}
	if !roleAllowsPermission(membership.Role, request.Permission) {
		return AuthorizationResult{}, ErrTenantAccessDenied
	}

	return AuthorizationResult{
		Organization: organization,
		Membership:   membership,
	}, nil
}

func validOrganizationInput(name string, slug string, ownerUserID string) bool {
	return name != "" &&
		organizationSlugPattern.MatchString(slug) &&
		ownerUserID != ""
}

func mapCreateOrganizationError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == organizationsSlugUniqueConstraint {
		return ErrOrganizationSlugExists
	}
	return err
}

func mapAuthorizationReadError(err error) error {
	if errors.Is(err, ErrOrganizationNotFound) || errors.Is(err, ErrMembershipNotFound) {
		return ErrTenantAccessDenied
	}
	return err
}
