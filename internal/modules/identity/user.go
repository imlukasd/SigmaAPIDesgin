package identity

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type User struct {
	ID              string
	Email           string
	EmailNormalized string
	PasswordHash    string
	DisplayName     string
	Status          UserStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateUserParams struct {
	ID              string
	Email           string
	EmailNormalized string
	PasswordHash    string
	DisplayName     string
}
