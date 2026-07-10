package tenancy

import "time"

type OrganizationStatus string

const (
	OrganizationStatusActive   OrganizationStatus = "active"
	OrganizationStatusDisabled OrganizationStatus = "disabled"
)

type Organization struct {
	ID        string
	Name      string
	Slug      string
	Status    OrganizationStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateOrganizationParams struct {
	ID   string
	Name string
	Slug string
}
