package tenancy

import "time"

type MembershipRole string

const (
	MembershipRoleOwner  MembershipRole = "owner"
	MembershipRoleAdmin  MembershipRole = "admin"
	MembershipRoleMember MembershipRole = "member"
)

type MembershipStatus string

const (
	MembershipStatusActive   MembershipStatus = "active"
	MembershipStatusDisabled MembershipStatus = "disabled"
)

type Membership struct {
	OrganizationID string
	UserID         string
	Role           MembershipRole
	Status         MembershipStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateMembershipParams struct {
	OrganizationID string
	UserID         string
	Role           MembershipRole
}
