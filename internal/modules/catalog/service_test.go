package catalog

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"corebe.local/api/internal/platform/database"
)

type fakeStore struct {
	createProductParams CreateProductParams
	createProductErr    error
	getProductOrgID     string
	getProductID        string
	getProduct          Product
	getProductErr       error
	getProductBySKU     Product
	getProductBySKUErr  error
}

func (f *fakeStore) CreateProduct(ctx context.Context, db database.DBTX, params CreateProductParams) (Product, error) {
	f.createProductParams = params
	if f.createProductErr != nil {
		return Product{}, f.createProductErr
	}
	return Product{
		ID:             params.ID,
		OrganizationID: params.OrganizationID,
		SKU:            params.SKU,
		Name:           params.Name,
		Description:    params.Description,
		PriceAmount:    params.PriceAmount,
		PriceCurrency:  params.PriceCurrency,
		Status:         ProductStatusDraft,
	}, nil
}

func (f *fakeStore) GetProduct(ctx context.Context, db database.DBTX, organizationID string, productID string) (Product, error) {
	f.getProductOrgID = organizationID
	f.getProductID = productID
	if f.getProductErr != nil {
		return Product{}, f.getProductErr
	}
	if f.getProduct.ID == "" {
		return Product{}, ErrProductNotFound
	}
	return f.getProduct, nil
}

func (f *fakeStore) GetProductBySKU(ctx context.Context, db database.DBTX, organizationID string, sku string) (Product, error) {
	if f.getProductBySKUErr != nil {
		return Product{}, f.getProductBySKUErr
	}
	if f.getProductBySKU.ID == "" {
		return Product{}, ErrProductNotFound
	}
	return f.getProductBySKU, nil
}

func TestServiceCreateProduct(t *testing.T) {
	t.Parallel()

	description := " Test product description "
	store := &fakeStore{}
	service := NewService(ServiceConfig{Store: store})

	result, err := service.CreateProduct(context.Background(), CreateProductRequest{
		OrganizationID: " org_123 ",
		SKU:            " sku_123 ",
		Name:           " Test Product ",
		Description:    &description,
		PriceAmount:    1299,
		PriceCurrency:  " usd ",
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	if result.Product.ID == "" {
		t.Fatal("expected generated product id")
	}
	if store.createProductParams.OrganizationID != "org_123" {
		t.Fatalf("expected trimmed organization id, got %s", store.createProductParams.OrganizationID)
	}
	if store.createProductParams.SKU != "SKU_123" {
		t.Fatalf("expected normalized sku SKU_123, got %s", store.createProductParams.SKU)
	}
	if store.createProductParams.Name != "Test Product" {
		t.Fatalf("expected trimmed name, got %s", store.createProductParams.Name)
	}
	if store.createProductParams.Description == nil || *store.createProductParams.Description != "Test product description" {
		t.Fatalf("expected trimmed description, got %v", store.createProductParams.Description)
	}
	if store.createProductParams.PriceCurrency != "USD" {
		t.Fatalf("expected normalized currency USD, got %s", store.createProductParams.PriceCurrency)
	}
	if result.Product.Status != ProductStatusDraft {
		t.Fatalf("expected draft status, got %s", result.Product.Status)
	}
}

func TestServiceCreateProductNormalizesBlankDescriptionToNil(t *testing.T) {
	t.Parallel()

	description := "   "
	store := &fakeStore{}
	service := NewService(ServiceConfig{Store: store})

	_, err := service.CreateProduct(context.Background(), CreateProductRequest{
		OrganizationID: "org_123",
		SKU:            "SKU_123",
		Name:           "Test Product",
		Description:    &description,
		PriceAmount:    0,
		PriceCurrency:  "USD",
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if store.createProductParams.Description != nil {
		t.Fatalf("expected nil description, got %v", store.createProductParams.Description)
	}
}

func TestServiceCreateProductRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request CreateProductRequest
	}{
		{
			name: "missing organization",
			request: CreateProductRequest{
				SKU:           "SKU_123",
				Name:          "Test Product",
				PriceAmount:   1299,
				PriceCurrency: "USD",
			},
		},
		{
			name: "invalid sku",
			request: CreateProductRequest{
				OrganizationID: "org_123",
				SKU:            "invalid sku",
				Name:           "Test Product",
				PriceAmount:    1299,
				PriceCurrency:  "USD",
			},
		},
		{
			name: "missing name",
			request: CreateProductRequest{
				OrganizationID: "org_123",
				SKU:            "SKU_123",
				PriceAmount:    1299,
				PriceCurrency:  "USD",
			},
		},
		{
			name: "negative price",
			request: CreateProductRequest{
				OrganizationID: "org_123",
				SKU:            "SKU_123",
				Name:           "Test Product",
				PriceAmount:    -1,
				PriceCurrency:  "USD",
			},
		},
		{
			name: "invalid currency",
			request: CreateProductRequest{
				OrganizationID: "org_123",
				SKU:            "SKU_123",
				Name:           "Test Product",
				PriceAmount:    1299,
				PriceCurrency:  "US",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeStore{}
			service := NewService(ServiceConfig{Store: store})

			_, err := service.CreateProduct(context.Background(), tt.request)
			if !errors.Is(err, ErrInvalidProductInput) {
				t.Fatalf("expected ErrInvalidProductInput, got %v", err)
			}
			if store.createProductParams.ID != "" {
				t.Fatalf("expected no repository write, got %+v", store.createProductParams)
			}
		})
	}
}

func TestServiceCreateProductMapsDuplicateSKU(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		createProductErr: fmt.Errorf("create product: %w", &pgconn.PgError{
			ConstraintName: productsOrganizationIDSKUUniqueIndex,
		}),
	}
	service := NewService(ServiceConfig{Store: store})

	_, err := service.CreateProduct(context.Background(), CreateProductRequest{
		OrganizationID: "org_123",
		SKU:            "SKU_123",
		Name:           "Test Product",
		PriceAmount:    1299,
		PriceCurrency:  "USD",
	})
	if !errors.Is(err, ErrProductSKUExists) {
		t.Fatalf("expected ErrProductSKUExists, got %v", err)
	}
}

func TestServiceGetProduct(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		getProduct: Product{
			ID:             "product_123",
			OrganizationID: "org_123",
			SKU:            "SKU_123",
			Status:         ProductStatusDraft,
		},
	}
	service := NewService(ServiceConfig{Store: store})

	result, err := service.GetProduct(context.Background(), GetProductRequest{
		OrganizationID: " org_123 ",
		ProductID:      " product_123 ",
	})
	if err != nil {
		t.Fatalf("get product: %v", err)
	}

	if store.getProductOrgID != "org_123" {
		t.Fatalf("expected organization lookup org_123, got %s", store.getProductOrgID)
	}
	if store.getProductID != "product_123" {
		t.Fatalf("expected product lookup product_123, got %s", store.getProductID)
	}
	if result.Product.ID != "product_123" {
		t.Fatalf("expected product id product_123, got %s", result.Product.ID)
	}
}

func TestServiceGetProductRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(ServiceConfig{Store: store})

	_, err := service.GetProduct(context.Background(), GetProductRequest{
		OrganizationID: "",
		ProductID:      "product_123",
	})
	if !errors.Is(err, ErrInvalidProductInput) {
		t.Fatalf("expected ErrInvalidProductInput, got %v", err)
	}
	if store.getProductID != "" {
		t.Fatalf("expected no repository read, got %s", store.getProductID)
	}
}

func TestServiceGetProductPropagatesNotFound(t *testing.T) {
	t.Parallel()

	store := &fakeStore{getProductErr: ErrProductNotFound}
	service := NewService(ServiceConfig{Store: store})

	_, err := service.GetProduct(context.Background(), GetProductRequest{
		OrganizationID: "org_123",
		ProductID:      "product_123",
	})
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
}
