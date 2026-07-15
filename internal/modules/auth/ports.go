package auth

import (
	"context"
	"time"

	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/platform/database"
)

type TransactionManager interface {
	WithinTx(ctx context.Context, fn func(context.Context, database.DBTX) error) error
}

type AuthService interface {
	Register(ctx context.Context, request RegisterRequest) (RegisterResult, error)
	Login(ctx context.Context, request LoginRequest) (LoginResult, error)
	RefreshToken(ctx context.Context, request RefreshTokenRequest) (RefreshTokenResult, error)
	Logout(ctx context.Context, request LogoutRequest) error
}

type UserRepository interface {
	CreateUser(ctx context.Context, db database.DBTX, params identity.CreateUserParams) (identity.User, error)
	GetUserByID(ctx context.Context, db database.DBTX, id string) (identity.User, error)
	GetUserByEmailNormalized(ctx context.Context, db database.DBTX, emailNormalized string) (identity.User, error)
}

type PasswordHasher interface {
	HashPassword(ctx context.Context, password string) (string, error)
	ComparePassword(ctx context.Context, password string, passwordHash string) error
}

type TokenIssuer interface {
	IssueTokenPair(ctx context.Context, params IssueTokenPairParams) (IssuedTokenPair, error)
}

type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, accessToken string) (AuthenticatedUser, error)
}

type SessionRepository interface {
	CreateAuthSession(ctx context.Context, db database.DBTX, params CreateAuthSessionParams) (AuthSession, error)
	GetAuthSessionByID(ctx context.Context, db database.DBTX, id string) (AuthSession, error)
	MarkAuthSessionUsed(ctx context.Context, db database.DBTX, id string, usedAt time.Time) (AuthSession, error)
	RevokeAuthSession(ctx context.Context, db database.DBTX, id string, revokedAt time.Time) (AuthSession, error)
	CreateRefreshToken(ctx context.Context, db database.DBTX, params CreateRefreshTokenParams) (RefreshToken, error)
	GetRefreshTokenByHashForUpdate(ctx context.Context, db database.DBTX, tokenHash string) (RefreshToken, error)
	MarkRefreshTokenUsed(ctx context.Context, db database.DBTX, id string, usedAt time.Time) (RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, db database.DBTX, id string, revokedAt time.Time) (RefreshToken, error)
}
