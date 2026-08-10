package catalog

import "time"

type ProductStatus string

const (
	ProductStatusDraft    ProductStatus = "draft"
	ProductStatusActive   ProductStatus = "active"
	ProductStatusArchived ProductStatus = "archived"
)

type Product struct {
	ID             string
	OrganizationID string
	SKU            string
	Name           string
	Description    *string
	PriceAmount    int64
	PriceCurrency  string
	Status         ProductStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateProductParams struct {
	ID             string
	OrganizationID string
	SKU            string
	Name           string
	Description    *string
	PriceAmount    int64
	PriceCurrency  string
}
