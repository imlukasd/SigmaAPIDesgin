package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/platform/testdb"
)

func TestRepositoryAuthSessionRefreshTokenLifecycleIntegration(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	authRepo := NewRepository()
	identityRepo := identity.NewRepository()

	suffix := testdb.UniqueSuffix(t)
	userID := testdb.NewUUID(t)
	sessionID := testdb.NewUUID(t)
	refreshTokenID := testdb.NewUUID(t)
	rotatedRefreshTokenID := testdb.NewUUID(t)
	email := "auth-session-user-" + suffix + "@example.com"

	_, err := identityRepo.CreateUser(ctx, db, identity.CreateUserParams{
		ID:              userID,
		Email:           email,
		EmailNormalized: email,
		PasswordHash:    "hashed-password-for-test",
		DisplayName:     "Auth Session User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM users WHERE id = $1`, userID)
	})

	userAgent := "CoreBE integration test"
	ipAddress := "203.0.113.10"
	sessionExpiresAt := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Microsecond)

	session, err := authRepo.CreateAuthSession(ctx, db, CreateAuthSessionParams{
		ID:        sessionID,
		UserID:    userID,
		UserAgent: &userAgent,
		IPAddress: &ipAddress,
		ExpiresAt: sessionExpiresAt,
	})
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}
	if session.ID != sessionID {
		t.Fatalf("expected session id %s, got %s", sessionID, session.ID)
	}
	if session.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, session.UserID)
	}
	if session.UserAgent == nil || *session.UserAgent != userAgent {
		t.Fatalf("expected user agent %s, got %v", userAgent, session.UserAgent)
	}
	if session.IPAddress == nil || *session.IPAddress != ipAddress {
		t.Fatalf("expected ip address %s, got %v", ipAddress, session.IPAddress)
	}

	readSession, err := authRepo.GetAuthSessionByID(ctx, db, sessionID)
	if err != nil {
		t.Fatalf("get auth session by id: %v", err)
	}
	if !readSession.ExpiresAt.Equal(sessionExpiresAt) {
		t.Fatalf("expected session expiry %s, got %s", sessionExpiresAt, readSession.ExpiresAt)
	}

	sessionUsedAt := time.Now().UTC().Truncate(time.Microsecond)
	usedSession, err := authRepo.MarkAuthSessionUsed(ctx, db, sessionID, sessionUsedAt)
	if err != nil {
		t.Fatalf("mark auth session used: %v", err)
	}
	if usedSession.LastUsedAt == nil || !usedSession.LastUsedAt.Equal(sessionUsedAt) {
		t.Fatalf("expected session last used at %s, got %v", sessionUsedAt, usedSession.LastUsedAt)
	}

	refreshExpiresAt := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(time.Microsecond)
	refreshToken, err := authRepo.CreateRefreshToken(ctx, db, CreateRefreshTokenParams{
		ID:        refreshTokenID,
		SessionID: sessionID,
		TokenHash: "refresh-token-hash-" + suffix,
		ExpiresAt: refreshExpiresAt,
	})
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
	if refreshToken.SessionID != sessionID {
		t.Fatalf("expected session id %s, got %s", sessionID, refreshToken.SessionID)
	}
	if refreshToken.RotatedFromTokenID != nil {
		t.Fatalf("expected initial refresh token to have no rotation parent")
	}

	byHash, err := authRepo.GetRefreshTokenByHash(ctx, db, refreshToken.TokenHash)
	if err != nil {
		t.Fatalf("get refresh token by hash: %v", err)
	}
	if byHash.ID != refreshTokenID {
		t.Fatalf("expected refresh token id %s, got %s", refreshTokenID, byHash.ID)
	}

	byHashForUpdate, err := authRepo.GetRefreshTokenByHashForUpdate(ctx, db, refreshToken.TokenHash)
	if err != nil {
		t.Fatalf("get refresh token by hash for update: %v", err)
	}
	if byHashForUpdate.ID != refreshTokenID {
		t.Fatalf("expected refresh token id %s, got %s", refreshTokenID, byHashForUpdate.ID)
	}

	usedAt := time.Now().UTC().Truncate(time.Microsecond)
	usedToken, err := authRepo.MarkRefreshTokenUsed(ctx, db, refreshTokenID, usedAt)
	if err != nil {
		t.Fatalf("mark refresh token used: %v", err)
	}
	if usedToken.LastUsedAt == nil || !usedToken.LastUsedAt.Equal(usedAt) {
		t.Fatalf("expected last used at %s, got %v", usedAt, usedToken.LastUsedAt)
	}

	rotatedToken, err := authRepo.CreateRefreshToken(ctx, db, CreateRefreshTokenParams{
		ID:                 rotatedRefreshTokenID,
		SessionID:          sessionID,
		TokenHash:          "rotated-refresh-token-hash-" + suffix,
		RotatedFromTokenID: &refreshTokenID,
		ExpiresAt:          refreshExpiresAt.Add(7 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create rotated refresh token: %v", err)
	}
	if rotatedToken.RotatedFromTokenID == nil || *rotatedToken.RotatedFromTokenID != refreshTokenID {
		t.Fatalf("expected rotated token parent %s, got %v", refreshTokenID, rotatedToken.RotatedFromTokenID)
	}

	revokedAt := time.Now().UTC().Truncate(time.Microsecond)
	revokedToken, err := authRepo.RevokeRefreshToken(ctx, db, refreshTokenID, revokedAt)
	if err != nil {
		t.Fatalf("revoke refresh token: %v", err)
	}
	if revokedToken.RevokedAt == nil || !revokedToken.RevokedAt.Equal(revokedAt) {
		t.Fatalf("expected refresh token revoked at %s, got %v", revokedAt, revokedToken.RevokedAt)
	}

	revokedSession, err := authRepo.RevokeAuthSession(ctx, db, sessionID, revokedAt)
	if err != nil {
		t.Fatalf("revoke auth session: %v", err)
	}
	if revokedSession.RevokedAt == nil || !revokedSession.RevokedAt.Equal(revokedAt) {
		t.Fatalf("expected session revoked at %s, got %v", revokedAt, revokedSession.RevokedAt)
	}
}

func TestRepositoryAuthSessionRefreshTokenNotFoundIntegration(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	authRepo := NewRepository()

	_, err := authRepo.GetAuthSessionByID(ctx, db, testdb.NewUUID(t))
	if !errors.Is(err, ErrAuthSessionNotFound) {
		t.Fatalf("expected ErrAuthSessionNotFound, got %v", err)
	}

	_, err = authRepo.GetRefreshTokenByHash(ctx, db, "missing-refresh-token-hash")
	if !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("expected ErrRefreshTokenNotFound, got %v", err)
	}

	_, err = authRepo.GetRefreshTokenByHashForUpdate(ctx, db, "missing-refresh-token-hash")
	if !errors.Is(err, ErrRefreshTokenNotFound) {
		t.Fatalf("expected ErrRefreshTokenNotFound, got %v", err)
	}
}
