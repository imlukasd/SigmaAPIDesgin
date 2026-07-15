package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"corebe.local/api/internal/platform/database"
)

type Repository struct{}

func NewRepository() Repository {
	return Repository{}
}

func (r Repository) CreateAuthSession(ctx context.Context, db database.DBTX, params CreateAuthSessionParams) (AuthSession, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO auth_sessions (
			id,
			user_id,
			user_agent,
			ip_address,
			expires_at
		)
		VALUES ($1, $2, $3, $4::inet, $5)
		RETURNING id, user_id, user_agent, ip_address::text, expires_at, revoked_at, last_used_at, created_at, updated_at
	`, params.ID, params.UserID, nullableStringArg(params.UserAgent), nullableStringArg(params.IPAddress), params.ExpiresAt)

	session, err := scanAuthSession(row)
	if err != nil {
		return AuthSession{}, fmt.Errorf("create auth session: %w", err)
	}
	return session, nil
}

func (r Repository) GetAuthSessionByID(ctx context.Context, db database.DBTX, id string) (AuthSession, error) {
	row := db.QueryRow(ctx, `
		SELECT id, user_id, user_agent, ip_address::text, expires_at, revoked_at, last_used_at, created_at, updated_at
		FROM auth_sessions
		WHERE id = $1
	`, id)

	session, err := scanAuthSession(row)
	if err != nil {
		return AuthSession{}, mapAuthSessionReadError(err)
	}
	return session, nil
}

func (r Repository) MarkAuthSessionUsed(ctx context.Context, db database.DBTX, id string, usedAt time.Time) (AuthSession, error) {
	row := db.QueryRow(ctx, `
		UPDATE auth_sessions
		SET last_used_at = $2,
			updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, user_agent, ip_address::text, expires_at, revoked_at, last_used_at, created_at, updated_at
	`, id, usedAt)

	session, err := scanAuthSession(row)
	if err != nil {
		return AuthSession{}, mapAuthSessionReadError(err)
	}
	return session, nil
}

func (r Repository) RevokeAuthSession(ctx context.Context, db database.DBTX, id string, revokedAt time.Time) (AuthSession, error) {
	row := db.QueryRow(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $2,
			updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, user_agent, ip_address::text, expires_at, revoked_at, last_used_at, created_at, updated_at
	`, id, revokedAt)

	session, err := scanAuthSession(row)
	if err != nil {
		return AuthSession{}, mapAuthSessionReadError(err)
	}
	return session, nil
}

func (r Repository) CreateRefreshToken(ctx context.Context, db database.DBTX, params CreateRefreshTokenParams) (RefreshToken, error) {
	row := db.QueryRow(ctx, `
		INSERT INTO refresh_tokens (
			id,
			session_id,
			token_hash,
			rotated_from_token_id,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, session_id, token_hash, rotated_from_token_id, expires_at, revoked_at, last_used_at, created_at, updated_at
	`, params.ID, params.SessionID, params.TokenHash, nullableStringArg(params.RotatedFromTokenID), params.ExpiresAt)

	token, err := scanRefreshToken(row)
	if err != nil {
		return RefreshToken{}, fmt.Errorf("create refresh token: %w", err)
	}
	return token, nil
}

func (r Repository) GetRefreshTokenByHash(ctx context.Context, db database.DBTX, tokenHash string) (RefreshToken, error) {
	row := db.QueryRow(ctx, `
		SELECT id, session_id, token_hash, rotated_from_token_id, expires_at, revoked_at, last_used_at, created_at, updated_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, tokenHash)

	token, err := scanRefreshToken(row)
	if err != nil {
		return RefreshToken{}, mapRefreshTokenReadError(err)
	}
	return token, nil
}

func (r Repository) GetRefreshTokenByHashForUpdate(ctx context.Context, db database.DBTX, tokenHash string) (RefreshToken, error) {
	row := db.QueryRow(ctx, `
		SELECT id, session_id, token_hash, rotated_from_token_id, expires_at, revoked_at, last_used_at, created_at, updated_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash)

	token, err := scanRefreshToken(row)
	if err != nil {
		return RefreshToken{}, mapRefreshTokenReadError(err)
	}
	return token, nil
}

func (r Repository) MarkRefreshTokenUsed(ctx context.Context, db database.DBTX, id string, usedAt time.Time) (RefreshToken, error) {
	row := db.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET last_used_at = $2,
			updated_at = now()
		WHERE id = $1
		RETURNING id, session_id, token_hash, rotated_from_token_id, expires_at, revoked_at, last_used_at, created_at, updated_at
	`, id, usedAt)

	token, err := scanRefreshToken(row)
	if err != nil {
		return RefreshToken{}, mapRefreshTokenReadError(err)
	}
	return token, nil
}

func (r Repository) RevokeRefreshToken(ctx context.Context, db database.DBTX, id string, revokedAt time.Time) (RefreshToken, error) {
	row := db.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = $2,
			updated_at = now()
		WHERE id = $1
		RETURNING id, session_id, token_hash, rotated_from_token_id, expires_at, revoked_at, last_used_at, created_at, updated_at
	`, id, revokedAt)

	token, err := scanRefreshToken(row)
	if err != nil {
		return RefreshToken{}, mapRefreshTokenReadError(err)
	}
	return token, nil
}

func scanAuthSession(row pgx.Row) (AuthSession, error) {
	var session AuthSession
	var userAgent sql.NullString
	var ipAddress sql.NullString
	var revokedAt sql.NullTime
	var lastUsedAt sql.NullTime

	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&userAgent,
		&ipAddress,
		&session.ExpiresAt,
		&revokedAt,
		&lastUsedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return AuthSession{}, err
	}

	session.UserAgent = nullableStringValue(userAgent)
	session.IPAddress = nullableStringValue(ipAddress)
	session.RevokedAt = nullableTimeValue(revokedAt)
	session.LastUsedAt = nullableTimeValue(lastUsedAt)

	return session, nil
}

func scanRefreshToken(row pgx.Row) (RefreshToken, error) {
	var token RefreshToken
	var rotatedFromTokenID sql.NullString
	var revokedAt sql.NullTime
	var lastUsedAt sql.NullTime

	if err := row.Scan(
		&token.ID,
		&token.SessionID,
		&token.TokenHash,
		&rotatedFromTokenID,
		&token.ExpiresAt,
		&revokedAt,
		&lastUsedAt,
		&token.CreatedAt,
		&token.UpdatedAt,
	); err != nil {
		return RefreshToken{}, err
	}

	token.RotatedFromTokenID = nullableStringValue(rotatedFromTokenID)
	token.RevokedAt = nullableTimeValue(revokedAt)
	token.LastUsedAt = nullableTimeValue(lastUsedAt)

	return token, nil
}

func nullableStringArg(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableStringValue(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullableTimeValue(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func mapAuthSessionReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAuthSessionNotFound
	}
	return fmt.Errorf("read auth session: %w", err)
}

func mapRefreshTokenReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRefreshTokenNotFound
	}
	return fmt.Errorf("read refresh token: %w", err)
}
