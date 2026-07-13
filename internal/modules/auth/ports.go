package auth

import (
	"context"

	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/platform/database"
)

type TransactionManager interface {
	WithinTx(ctx context.Context, fn func(context.Context, database.DBTX) error) error
}

type UserRepository interface {
	CreateUser(ctx context.Context, db database.DBTX, params identity.CreateUserParams) (identity.User, error)
	GetUserByEmailNormalized(ctx context.Context, db database.DBTX, emailNormalized string) (identity.User, error)
}

type PasswordHasher interface {
	HashPassword(ctx context.Context, password string) (string, error)
	ComparePassword(ctx context.Context, password string, passwordHash string) error
}

type TokenIssuer interface {
	IssueTokenPair(ctx context.Context, params IssueTokenPairParams) (IssuedTokenPair, error)
}
