package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"corebe.local/api/internal/modules/tenancy"
	"corebe.local/api/internal/platform/testdb"
)

func TestRepositoryProductTenantIsolationIntegration(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	catalogRepo := NewRepository()
	tenancyRepo := tenancy.NewRepository()

	suffix := testdb.UniqueSuffix(t)
	organizationAID := testdb.NewUUID(t)
	organizationBID := testdb.NewUUID(t)

	if _, err := tenancyRepo.CreateOrganization(ctx, db, tenancy.CreateOrganizationParams{
		ID:   organizationAID,
		Name: "Catalog Organization A",
		Slug: "catalog-a-" + suffix[:20],
	}); err != nil {
		t.Fatalf("create organization a: %v", err)
	}
	if _, err := tenancyRepo.CreateOrganization(ctx, db, tenancy.CreateOrganizationParams{
		ID:   organizationBID,
		Name: "Catalog Organization B",
		Slug: "catalog-b-" + suffix[:20],
	}); err != nil {
		t.Fatalf("create organization b: %v", err)
	}
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM organizations WHERE id = $1 OR id = $2`, organizationAID, organizationBID)
	})
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM products WHERE organization_id = $1 OR organization_id = $2`, organizationAID, organizationBID)
	})

	description := "Repository integration test product"
	productID := testdb.NewUUID(t)
	sku := "SKU_" + strings.ToUpper(suffix[:20])

	created, err := catalogRepo.CreateProduct(ctx, db, CreateProductParams{
		ID:             productID,
		OrganizationID: organizationAID,
		SKU:            sku,
		Name:           "Test Product",
		Description:    &description,
		PriceAmount:    1299,
		PriceCurrency:  "USD",
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if created.ID != productID {
		t.Fatalf("expected product id %s, got %s", productID, created.ID)
	}
	if created.OrganizationID != organizationAID {
		t.Fatalf("expected organization id %s, got %s", organizationAID, created.OrganizationID)
	}
	if created.Status != ProductStatusDraft {
		t.Fatalf("expected draft status, got %s", created.Status)
	}
	if created.Description == nil || *created.Description != description {
		t.Fatalf("expected description %q, got %v", description, created.Description)
	}

	byID, err := catalogRepo.GetProduct(ctx, db, organizationAID, productID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if byID.SKU != sku {
		t.Fatalf("expected sku %s, got %s", sku, byID.SKU)
	}

	bySKU, err := catalogRepo.GetProductBySKU(ctx, db, organizationAID, sku)
	if err != nil {
		t.Fatalf("get product by sku: %v", err)
	}
	if bySKU.ID != productID {
		t.Fatalf("expected product id %s, got %s", productID, bySKU.ID)
	}

	_, err = catalogRepo.GetProduct(ctx, db, organizationBID, productID)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound for wrong organization, got %v", err)
	}

	_, err = catalogRepo.CreateProduct(ctx, db, CreateProductParams{
		ID:             testdb.NewUUID(t),
		OrganizationID: organizationAID,
		SKU:            sku,
		Name:           "Duplicate SKU Product",
		PriceAmount:    1500,
		PriceCurrency:  "USD",
	})
	if !isConstraintError(err, "products_organization_id_sku_unique") {
		t.Fatalf("expected products_organization_id_sku_unique violation, got %v", err)
	}

	sameSKUOtherOrganization, err := catalogRepo.CreateProduct(ctx, db, CreateProductParams{
		ID:             testdb.NewUUID(t),
		OrganizationID: organizationBID,
		SKU:            sku,
		Name:           "Same SKU Other Organization",
		PriceAmount:    1500,
		PriceCurrency:  "USD",
	})
	if err != nil {
		t.Fatalf("create same sku in other organization: %v", err)
	}
	if sameSKUOtherOrganization.OrganizationID != organizationBID {
		t.Fatalf("expected organization id %s, got %s", organizationBID, sameSKUOtherOrganization.OrganizationID)
	}
}

func TestRepositoryProductNotFoundIntegration(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	repo := NewRepository()

	_, err := repo.GetProduct(ctx, db, testdb.NewUUID(t), testdb.NewUUID(t))
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}

	_, err = repo.GetProductBySKU(ctx, db, testdb.NewUUID(t), "MISSING_SKU")
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
}

func isConstraintError(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == constraintName
}
