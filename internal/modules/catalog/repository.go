package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"corebe.local/api/internal/platform/database"
)

type Repository struct{}

func NewRepository() Repository {
	return Repository{}
}

func (r Repository) CreateProduct(ctx context.Context, db database.DBTX, params CreateProductParams) (Product, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO products (
			id,
			organization_id,
			sku,
			name,
			description,
			price_amount,
			price_currency
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, organization_id, sku, name, description, price_amount, price_currency, status, created_at, updated_at
	`, params.ID, params.OrganizationID, params.SKU, params.Name, nullableStringArg(params.Description), params.PriceAmount, params.PriceCurrency)

	product, err := scanProduct(row)
	if err != nil {
		return Product{}, fmt.Errorf("create product: %w", err)
	}
	return product, nil
}

func (r Repository) GetProduct(ctx context.Context, db database.DBTX, organizationID string, productID string) (Product, error) {
	row := db.QueryRow(ctx, `
		SELECT id, organization_id, sku, name, description, price_amount, price_currency, status, created_at, updated_at
		FROM products
		WHERE organization_id = $1 AND id = $2
	`, organizationID, productID)

	product, err := scanProduct(row)
	if err != nil {
		return Product{}, mapProductReadError(err)
	}
	return product, nil
}

func (r Repository) GetProductBySKU(ctx context.Context, db database.DBTX, organizationID string, sku string) (Product, error) {
	row := db.QueryRow(ctx, `
		SELECT id, organization_id, sku, name, description, price_amount, price_currency, status, created_at, updated_at
		FROM products
		WHERE organization_id = $1 AND sku = $2
	`, organizationID, sku)

	product, err := scanProduct(row)
	if err != nil {
		return Product{}, mapProductReadError(err)
	}
	return product, nil
}

func scanProduct(row pgx.Row) (Product, error) {
	var product Product
	var description sql.NullString

	if err := row.Scan(
		&product.ID,
		&product.OrganizationID,
		&product.SKU,
		&product.Name,
		&description,
		&product.PriceAmount,
		&product.PriceCurrency,
		&product.Status,
		&product.CreatedAt,
		&product.UpdatedAt,
	); err != nil {
		return Product{}, err
	}

	product.Description = nullableStringValue(description)
	return product, nil
}

func nullableStringArg(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableStringValue(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func mapProductReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProductNotFound
	}
	return fmt.Errorf("read product: %w", err)
}
