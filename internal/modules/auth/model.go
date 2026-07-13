package auth

import "time"

type RegisterRequest struct {
	Email       string
	Password    string
	DisplayName string
}

type LoginRequest struct {
	Email    string
	Password string
}

type TokenPair struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type IssueTokenPairParams struct {
	UserID    string
	SessionID string
}

type IssuedTokenPair struct {
	Tokens           TokenPair
	AccessTokenID    string
	RefreshTokenHash string
}

type AuthenticatedUser struct {
	UserID string
	Email  string
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
