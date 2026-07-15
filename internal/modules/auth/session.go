package auth

import "time"

type AuthSession struct {
	ID         string
	UserID     string
	UserAgent  *string
	IPAddress  *string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CreateAuthSessionParams struct {
	ID        string
	UserID    string
	UserAgent *string
	IPAddress *string
	ExpiresAt time.Time
}

type RefreshToken struct {
	ID                 string
	SessionID          string
	TokenHash          string
	RotatedFromTokenID *string
	ExpiresAt          time.Time
	RevokedAt          *time.Time
	LastUsedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateRefreshTokenParams struct {
	ID                 string
	SessionID          string
	TokenHash          string
	RotatedFromTokenID *string
	ExpiresAt          time.Time
}
