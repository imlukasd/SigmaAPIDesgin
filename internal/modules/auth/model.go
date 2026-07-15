package auth

import "time"

type RegisterRequest struct {
	Email       string
	Password    string
	DisplayName string
}

type LoginRequest struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

type RefreshTokenRequest struct {
	RefreshToken string
}

type LogoutRequest struct {
	RefreshToken string
}

type TokenPair struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type IssueTokenPairParams struct {
	UserID           string
	SessionID        string
	SessionExpiresAt time.Time
}

type IssuedTokenPair struct {
	Tokens           TokenPair
	AccessTokenID    string
	RefreshTokenHash string
}

type AuthenticatedUser struct {
	UserID        string
	SessionID     string
	AccessTokenID string
	IssuedAt      time.Time
	ExpiresAt     time.Time
}

type RegisterResult struct {
	UserID string
	Email  string
}

type LoginResult struct {
	UserID string
	Email  string
	Tokens TokenPair
}

type RefreshTokenResult struct {
	UserID string
	Email  string
	Tokens TokenPair
}
