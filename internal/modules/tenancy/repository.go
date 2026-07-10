package tenancy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"corebe.local/api/internal/platform/database"
)

var (
	ErrOrganizationNotFound = errors.New("tenancy organization not found")
	ErrMembershipNotFound   = errors.New("tenancy membership not found")
)

type Repository struct{}

func NewRepository() Repository {
	return Repository{}
}

func (r Repository) CreateOrganization(ctx context.Context, db database.DBTX, params CreateOrganizationParams) (Organization, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO organizations (id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING id, name, slug, status, created_at, updated_at
	`, params.ID, params.Name, params.Slug)

	organization, err := scanOrganization(row)
	if err != nil {
		return Organization{}, fmt.Errorf("create organization: %w", err)
	}
	return organization, nil
}

func (r Repository) GetOrganizationByID(ctx context.Context, db database.DBTX, id string) (Organization, error) {
	row := db.QueryRow(ctx, `
		SELECT id, name, slug, status, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`, id)

	organization, err := scanOrganization(row)
	if err != nil {
		return Organization{}, mapOrganizationReadError(err)
	}
	return organization, nil
}

func (r Repository) GetOrganizationBySlug(ctx context.Context, db database.DBTX, slug string) (Organization, error) {
	row := db.QueryRow(ctx, `
		SELECT id, name, slug, status, created_at, updated_at
		FROM organizations
		WHERE slug = $1
	`, slug)

	organization, err := scanOrganization(row)
	if err != nil {
		return Organization{}, mapOrganizationReadError(err)
	}
	return organization, nil
}

func (r Repository) CreateMembership(ctx context.Context, db database.DBTX, params CreateMembershipParams) (Membership, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING organization_id, user_id, role, status, created_at, updated_at
	`, params.OrganizationID, params.UserID, params.Role)

	membership, err := scanMembership(row)
	if err != nil {
		return Membership{}, fmt.Errorf("create membership: %w", err)
	}
	return membership, nil
}

func (r Repository) GetMembership(ctx context.Context, db database.DBTX, organizationID string, userID string) (Membership, error) {
	row := db.QueryRow(ctx, `
		SELECT organization_id, user_id, role, status, created_at, updated_at
		FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2
	`, organizationID, userID)

	membership, err := scanMembership(row)
	if err != nil {
		return Membership{}, mapMembershipReadError(err)
	}
	return membership, nil
}

func scanOrganization(row pgx.Row) (Organization, error) {
	var organization Organization
	if err := row.Scan(
		&organization.ID,
		&organization.Name,
		&organization.Slug,
		&organization.Status,
		&organization.CreatedAt,
		&organization.UpdatedAt,
	); err != nil {
		return Organization{}, err
	}
	return organization, nil
}

func scanMembership(row pgx.Row) (Membership, error) {
	var membership Membership
	if err := row.Scan(
		&membership.OrganizationID,
		&membership.UserID,
		&membership.Role,
		&membership.Status,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	); err != nil {
		return Membership{}, err
	}
	return membership, nil
}

func mapOrganizationReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOrganizationNotFound
	}
	return fmt.Errorf("read organization: %w", err)
}

func mapMembershipReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMembershipNotFound
	}
	return fmt.Errorf("read membership: %w", err)
}
