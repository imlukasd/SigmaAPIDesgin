package catalog

import (
	"context"

	"corebe.local/api/internal/platform/database"
)

type ProductService interface {
	CreateProduct(ctx context.Context, request CreateProductRequest) (CreateProductResult, error)
	GetProduct(ctx context.Context, request GetProductRequest) (GetProductResult, error)
}

type Store interface {
	CreateProduct(ctx context.Context, db database.DBTX, params CreateProductParams) (Product, error)
	GetProduct(ctx context.Context, db database.DBTX, organizationID string, productID string) (Product, error)
}
