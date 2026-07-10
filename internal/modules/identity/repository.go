package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"corebe.local/api/internal/platform/database"
)

var ErrUserNotFound = errors.New("identity user not found")

type Repository struct{}

func NewRepository() Repository {
	return Repository{}
}

func (r Repository) CreateUser(ctx context.Context, db database.DBTX, params CreateUserParams) (User, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO users (
			id,
			email,
			email_normalized,
			password_hash,
			display_name
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, email_normalized, password_hash, display_name, status, created_at, updated_at
	`, params.ID, params.Email, params.EmailNormalized, params.PasswordHash, params.DisplayName)

	user, err := scanUser(row)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r Repository) GetUserByID(ctx context.Context, db database.DBTX, id string) (User, error) {
	row := db.QueryRow(ctx, `
		SELECT id, email, email_normalized, password_hash, display_name, status, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id)

	user, err := scanUser(row)
	if err != nil {
		return User{}, mapUserReadError(err)
	}
	return user, nil
}

func (r Repository) GetUserByEmailNormalized(ctx context.Context, db database.DBTX, emailNormalized string) (User, error) {
	row := db.QueryRow(ctx, `
		SELECT id, email, email_normalized, password_hash, display_name, status, created_at, updated_at
		FROM users
		WHERE email_normalized = $1
	`, emailNormalized)

	user, err := scanUser(row)
	if err != nil {
		return User{}, mapUserReadError(err)
	}
	return user, nil
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	if err := row.Scan(
		&user.ID,
		&user.Email,
		&user.EmailNormalized,
		&user.PasswordHash,
		&user.DisplayName,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return User{}, err
	}
	return user, nil
}

func mapUserReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	return fmt.Errorf("read user: %w", err)
}
