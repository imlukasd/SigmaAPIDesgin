package tenancy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"corebe.local/api/internal/platform/database"
)

type fakeTransactionManager struct {
	db database.DBTX
}

func (f fakeTransactionManager) WithinTx(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, f.db)
}

type fakeStore struct {
	createOrganizationParams CreateOrganizationParams
	createOrganizationErr    error
	createMembershipParams   CreateMembershipParams
	createMembershipErr      error
	getOrganizationID        string
	getOrganization          Organization
	getOrganizationErr       error
	getMembershipOrgID       string
	getMembershipUserID      string
	getMembership            Membership
	getMembershipErr         error
}

func (f *fakeStore) CreateOrganization(ctx context.Context, db database.DBTX, params CreateOrganizationParams) (Organization, error) {
	f.createOrganizationParams = params
	if f.createOrganizationErr != nil {
		return Organization{}, f.createOrganizationErr
	}
	return Organization{
		ID:     params.ID,
		Name:   params.Name,
		Slug:   params.Slug,
		Status: OrganizationStatusActive,
	}, nil
}

func (f *fakeStore) GetOrganizationByID(ctx context.Context, db database.DBTX, id string) (Organization, error) {
	f.getOrganizationID = id
	if f.getOrganizationErr != nil {
		return Organization{}, f.getOrganizationErr
	}
	if f.getOrganization.ID == "" {
		return Organization{}, ErrOrganizationNotFound
	}
	return f.getOrganization, nil
}

func (f *fakeStore) CreateMembership(ctx context.Context, db database.DBTX, params CreateMembershipParams) (Membership, error) {
	f.createMembershipParams = params
	if f.createMembershipErr != nil {
		return Membership{}, f.createMembershipErr
	}
	return Membership{
		OrganizationID: params.OrganizationID,
		UserID:         params.UserID,
		Role:           params.Role,
		Status:         MembershipStatusActive,
	}, nil
}

func (f *fakeStore) GetMembership(ctx context.Context, db database.DBTX, organizationID string, userID string) (Membership, error) {
	f.getMembershipOrgID = organizationID
	f.getMembershipUserID = userID
	if f.getMembershipErr != nil {
		return Membership{}, f.getMembershipErr
	}
	if f.getMembership.OrganizationID == "" {
		return Membership{}, ErrMembershipNotFound
	}
	return f.getMembership, nil
}

func TestServiceCreateOrganization(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Store:      store,
	})

	result, err := service.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name:        " Test Organization ",
		Slug:        "test-organization",
		OwnerUserID: " user_123 ",
	})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	if result.Organization.ID == "" {
		t.Fatal("expected generated organization id")
	}
	if store.createOrganizationParams.Name != "Test Organization" {
		t.Fatalf("expected trimmed organization name, got %s", store.createOrganizationParams.Name)
	}
	if store.createOrganizationParams.Slug != "test-organization" {
		t.Fatalf("expected slug, got %s", store.createOrganizationParams.Slug)
	}
	if store.createMembershipParams.OrganizationID != result.Organization.ID {
		t.Fatalf("expected owner membership organization id %s, got %s", result.Organization.ID, store.createMembershipParams.OrganizationID)
	}
	if store.createMembershipParams.UserID != "user_123" {
		t.Fatalf("expected trimmed owner user id, got %s", store.createMembershipParams.UserID)
	}
	if store.createMembershipParams.Role != MembershipRoleOwner {
		t.Fatalf("expected owner role, got %s", store.createMembershipParams.Role)
	}
	if result.Owner.Role != MembershipRoleOwner {
		t.Fatalf("expected owner role result, got %s", result.Owner.Role)
	}
}

func TestServiceCreateOrganizationRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Store:      store,
	})

	_, err := service.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name:        "Test Organization",
		Slug:        "Invalid Slug",
		OwnerUserID: "user_123",
	})
	if !errors.Is(err, ErrInvalidOrganizationInput) {
		t.Fatalf("expected ErrInvalidOrganizationInput, got %v", err)
	}
	if store.createOrganizationParams.ID != "" {
		t.Fatalf("expected no repository write, got %+v", store.createOrganizationParams)
	}
}

func TestServiceCreateOrganizationMapsDuplicateSlug(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		createOrganizationErr: fmt.Errorf("create organization: %w", &pgconn.PgError{
			ConstraintName: organizationsSlugUniqueConstraint,
		}),
	}
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Store:      store,
	})

	_, err := service.CreateOrganization(context.Background(), CreateOrganizationRequest{
		Name:        "Test Organization",
		Slug:        "test-organization",
		OwnerUserID: "user_123",
	})
	if !errors.Is(err, ErrOrganizationSlugExists) {
		t.Fatalf("expected ErrOrganizationSlugExists, got %v", err)
	}
	if store.createMembershipParams.OrganizationID != "" {
		t.Fatalf("expected no membership write, got %+v", store.createMembershipParams)
	}
}

func TestServiceAuthorize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		role       MembershipRole
		permission Permission
	}{
		{name: "owner manage", role: MembershipRoleOwner, permission: PermissionOrganizationManage},
		{name: "admin manage", role: MembershipRoleAdmin, permission: PermissionOrganizationManage},
		{name: "member read", role: MembershipRoleMember, permission: PermissionOrganizationRead},
		{name: "admin product manage", role: MembershipRoleAdmin, permission: PermissionProductManage},
		{name: "member product read", role: MembershipRoleMember, permission: PermissionProductRead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeStore{
				getOrganization: Organization{
					ID:     "org_123",
					Status: OrganizationStatusActive,
				},
				getMembership: Membership{
					OrganizationID: "org_123",
					UserID:         "user_123",
					Role:           tt.role,
					Status:         MembershipStatusActive,
				},
			}
			service := NewService(ServiceConfig{Store: store})

			result, err := service.Authorize(context.Background(), AuthorizeRequest{
				OrganizationID: " org_123 ",
				UserID:         " user_123 ",
				Permission:     tt.permission,
			})
			if err != nil {
				t.Fatalf("authorize: %v", err)
			}
			if store.getOrganizationID != "org_123" {
				t.Fatalf("expected organization lookup org_123, got %s", store.getOrganizationID)
			}
			if store.getMembershipOrgID != "org_123" || store.getMembershipUserID != "user_123" {
				t.Fatalf("expected membership lookup org_123/user_123, got %s/%s", store.getMembershipOrgID, store.getMembershipUserID)
			}
			if result.Membership.Role != tt.role {
				t.Fatalf("expected role %s, got %s", tt.role, result.Membership.Role)
			}
		})
	}
}

func TestServiceAuthorizeDeniesAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		organization     Organization
		organizationErr  error
		membership       Membership
		membershipErr    error
		permission       Permission
		expectMembership bool
	}{
		{
			name:            "missing organization",
			organizationErr: ErrOrganizationNotFound,
			permission:      PermissionOrganizationRead,
		},
		{
			name: "disabled organization",
			organization: Organization{
				ID:     "org_123",
				Status: OrganizationStatusDisabled,
			},
			permission: PermissionOrganizationRead,
		},
		{
			name: "missing membership",
			organization: Organization{
				ID:     "org_123",
				Status: OrganizationStatusActive,
			},
			membershipErr:    ErrMembershipNotFound,
			permission:       PermissionOrganizationRead,
			expectMembership: true,
		},
		{
			name: "disabled membership",
			organization: Organization{
				ID:     "org_123",
				Status: OrganizationStatusActive,
			},
			membership: Membership{
				OrganizationID: "org_123",
				UserID:         "user_123",
				Role:           MembershipRoleOwner,
				Status:         MembershipStatusDisabled,
			},
			permission:       PermissionOrganizationRead,
			expectMembership: true,
		},
		{
			name: "member cannot manage",
			organization: Organization{
				ID:     "org_123",
				Status: OrganizationStatusActive,
			},
			membership: Membership{
				OrganizationID: "org_123",
				UserID:         "user_123",
				Role:           MembershipRoleMember,
				Status:         MembershipStatusActive,
			},
			permission:       PermissionOrganizationManage,
			expectMembership: true,
		},
		{
			name: "member cannot manage product",
			organization: Organization{
				ID:     "org_123",
				Status: OrganizationStatusActive,
			},
			membership: Membership{
				OrganizationID: "org_123",
				UserID:         "user_123",
				Role:           MembershipRoleMember,
				Status:         MembershipStatusActive,
			},
			permission:       PermissionProductManage,
			expectMembership: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeStore{
				getOrganization:    tt.organization,
				getOrganizationErr: tt.organizationErr,
				getMembership:      tt.membership,
				getMembershipErr:   tt.membershipErr,
			}
			service := NewService(ServiceConfig{Store: store})

			_, err := service.Authorize(context.Background(), AuthorizeRequest{
				OrganizationID: "org_123",
				UserID:         "user_123",
				Permission:     tt.permission,
			})
			if !errors.Is(err, ErrTenantAccessDenied) {
				t.Fatalf("expected ErrTenantAccessDenied, got %v", err)
			}
			if !tt.expectMembership && store.getMembershipOrgID != "" {
				t.Fatalf("expected no membership lookup, got %s/%s", store.getMembershipOrgID, store.getMembershipUserID)
			}
		})
	}
}

func TestServiceAuthorizeRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{Store: &fakeStore{}})

	_, err := service.Authorize(context.Background(), AuthorizeRequest{
		OrganizationID: "",
		UserID:         "user_123",
		Permission:     PermissionOrganizationRead,
	})
	if !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("expected ErrInvalidAuthorization, got %v", err)
	}
}
