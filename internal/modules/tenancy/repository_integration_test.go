package tenancy

import (
	"context"
	"errors"
	"testing"

	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/platform/testdb"
)

func TestRepositoryCreateAndReadOrganizationMembershipIntegration(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	identityRepo := identity.NewRepository()
	tenancyRepo := NewRepository()

	suffix := testdb.UniqueSuffix(t)
	userID := testdb.NewUUID(t)
	organizationID := testdb.NewUUID(t)
	email := "tenant-user-" + suffix + "@example.com"
	slug := "org-" + suffix[:24]

	_, err := identityRepo.CreateUser(ctx, db, identity.CreateUserParams{
		ID:              userID,
		Email:           email,
		EmailNormalized: email,
		PasswordHash:    "hashed-password-for-test",
		DisplayName:     "Tenant User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	organization, err := tenancyRepo.CreateOrganization(ctx, db, CreateOrganizationParams{
		ID:   organizationID,
		Name: "Test Organization",
		Slug: slug,
	})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	membership, err := tenancyRepo.CreateMembership(ctx, db, CreateMembershipParams{
		OrganizationID: organizationID,
		UserID:         userID,
		Role:           MembershipRoleOwner,
	})
	if err != nil {
		t.Fatalf("create membership: %v", err)
	}
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM users WHERE id = $1`, userID)
	})
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM organizations WHERE id = $1`, organizationID)
	})

	if organization.Status != OrganizationStatusActive {
		t.Fatalf("expected active organization, got %s", organization.Status)
	}
	if membership.Role != MembershipRoleOwner {
		t.Fatalf("expected owner role, got %s", membership.Role)
	}

	byID, err := tenancyRepo.GetOrganizationByID(ctx, db, organizationID)
	if err != nil {
		t.Fatalf("get organization by id: %v", err)
	}
	if byID.Slug != slug {
		t.Fatalf("expected slug %s, got %s", slug, byID.Slug)
	}

	bySlug, err := tenancyRepo.GetOrganizationBySlug(ctx, db, slug)
	if err != nil {
		t.Fatalf("get organization by slug: %v", err)
	}
	if bySlug.ID != organizationID {
		t.Fatalf("expected organization id %s, got %s", organizationID, bySlug.ID)
	}

	gotMembership, err := tenancyRepo.GetMembership(ctx, db, organizationID, userID)
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if gotMembership.Status != MembershipStatusActive {
		t.Fatalf("expected active membership, got %s", gotMembership.Status)
	}

	_, err = tenancyRepo.GetOrganizationByID(ctx, db, testdb.NewUUID(t))
	if !errors.Is(err, ErrOrganizationNotFound) {
		t.Fatalf("expected ErrOrganizationNotFound, got %v", err)
	}

	_, err = tenancyRepo.GetMembership(ctx, db, organizationID, testdb.NewUUID(t))
	if !errors.Is(err, ErrMembershipNotFound) {
		t.Fatalf("expected ErrMembershipNotFound, got %v", err)
	}
}
