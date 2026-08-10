package catalog

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"corebe.local/api/internal/platform/id"
)

const productsOrganizationIDSKUUniqueIndex = "products_organization_id_sku_unique"

var (
	productSKUPattern      = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
	productCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Service struct {
	db    *pgxpool.Pool
	store Store
}

type ServiceConfig struct {
	DB    *pgxpool.Pool
	Store Store
}

type CreateProductRequest struct {
	OrganizationID string
	SKU            string
	Name           string
	Description    *string
	PriceAmount    int64
	PriceCurrency  string
}

type CreateProductResult struct {
	Product Product
}

type GetProductRequest struct {
	OrganizationID string
	ProductID      string
}

type GetProductResult struct {
	Product Product
}

func NewService(cfg ServiceConfig) Service {
	return Service{
		db:    cfg.DB,
		store: cfg.Store,
	}
}

func (s Service) CreateProduct(ctx context.Context, request CreateProductRequest) (CreateProductResult, error) {
	organizationID := strings.TrimSpace(request.OrganizationID)
	sku := strings.ToUpper(strings.TrimSpace(request.SKU))
	name := strings.TrimSpace(request.Name)
	description := optionalTrimmedString(request.Description)
	priceCurrency := strings.ToUpper(strings.TrimSpace(request.PriceCurrency))

	if !validCreateProductInput(organizationID, sku, name, request.PriceAmount, priceCurrency) {
		return CreateProductResult{}, ErrInvalidProductInput
	}

	productID, err := id.NewUUID()
	if err != nil {
		return CreateProductResult{}, err
	}

	product, err := s.store.CreateProduct(ctx, s.db, CreateProductParams{
		ID:             productID,
		OrganizationID: organizationID,
		SKU:            sku,
		Name:           name,
		Description:    description,
		PriceAmount:    request.PriceAmount,
		PriceCurrency:  priceCurrency,
	})
	if err != nil {
		return CreateProductResult{}, mapCreateProductError(err)
	}

	return CreateProductResult{Product: product}, nil
}

func (s Service) GetProduct(ctx context.Context, request GetProductRequest) (GetProductResult, error) {
	organizationID := strings.TrimSpace(request.OrganizationID)
	productID := strings.TrimSpace(request.ProductID)
	if organizationID == "" || productID == "" {
		return GetProductResult{}, ErrInvalidProductInput
	}

	product, err := s.store.GetProduct(ctx, s.db, organizationID, productID)
	if err != nil {
		return GetProductResult{}, err
	}

	return GetProductResult{Product: product}, nil
}

func optionalTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validCreateProductInput(organizationID string, sku string, name string, priceAmount int64, priceCurrency string) bool {
	return organizationID != "" &&
		productSKUPattern.MatchString(sku) &&
		name != "" &&
		priceAmount >= 0 &&
		productCurrencyPattern.MatchString(priceCurrency)
}

func mapCreateProductError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == productsOrganizationIDSKUUniqueIndex {
		return ErrProductSKUExists
	}
	return err
}
