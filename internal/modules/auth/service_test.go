package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/platform/database"
)

type fakeTransactionManager struct {
	db database.DBTX
}

func (f fakeTransactionManager) WithinTx(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, f.db)
}

type fakePasswordHasher struct {
	hashErr    error
	compareErr error
}

func (f fakePasswordHasher) HashPassword(ctx context.Context, password string) (string, error) {
	if f.hashErr != nil {
		return "", f.hashErr
	}
	return "hashed:" + password, nil
}

func (f fakePasswordHasher) ComparePassword(ctx context.Context, password string, passwordHash string) error {
	if f.compareErr != nil {
		return f.compareErr
	}
	return nil
}

type fakeUserRepository struct {
	createParams       identity.CreateUserParams
	createErr          error
	getID              string
	getByIDUser        identity.User
	getByIDErr         error
	getEmailNormalized string
	getUser            identity.User
	getErr             error
}

func (f *fakeUserRepository) CreateUser(ctx context.Context, db database.DBTX, params identity.CreateUserParams) (identity.User, error) {
	f.createParams = params
	if f.createErr != nil {
		return identity.User{}, f.createErr
	}

	return identity.User{
		ID:              params.ID,
		Email:           params.Email,
		EmailNormalized: params.EmailNormalized,
		PasswordHash:    params.PasswordHash,
		DisplayName:     params.DisplayName,
		Status:          identity.UserStatusActive,
	}, nil
}

func (f *fakeUserRepository) GetUserByID(ctx context.Context, db database.DBTX, id string) (identity.User, error) {
	f.getID = id
	if f.getByIDErr != nil {
		return identity.User{}, f.getByIDErr
	}
	if f.getByIDUser.ID == "" {
		return identity.User{}, identity.ErrUserNotFound
	}
	return f.getByIDUser, nil
}

func (f *fakeUserRepository) GetUserByEmailNormalized(ctx context.Context, db database.DBTX, emailNormalized string) (identity.User, error) {
	f.getEmailNormalized = emailNormalized
	if f.getErr != nil {
		return identity.User{}, f.getErr
	}
	if f.getUser.ID == "" {
		return identity.User{}, identity.ErrUserNotFound
	}
	return f.getUser, nil
}

type fakeTokenIssuer struct {
	called bool
	params IssueTokenPairParams
	issued IssuedTokenPair
	err    error
}

func (f *fakeTokenIssuer) IssueTokenPair(ctx context.Context, params IssueTokenPairParams) (IssuedTokenPair, error) {
	f.called = true
	f.params = params
	if f.err != nil {
		return IssuedTokenPair{}, f.err
	}
	return f.issued, nil
}

type fakeSessionRepository struct {
	createAuthSessionCalled bool
	createAuthSessionParams CreateAuthSessionParams
	createAuthSessionErr    error
	getAuthSessionID        string
	getAuthSession          AuthSession
	getAuthSessionErr       error
	markAuthSessionUsedID   string
	markAuthSessionUsedAt   time.Time
	markAuthSessionUsedErr  error
	revokeAuthSessionID     string
	revokeAuthSessionAt     time.Time
	revokeAuthSessionErr    error
	createRefreshCalled     bool
	createRefreshParams     CreateRefreshTokenParams
	createRefreshErr        error
	getRefreshHash          string
	getRefreshToken         RefreshToken
	getRefreshErr           error
	markRefreshUsedID       string
	markRefreshUsedAt       time.Time
	markRefreshUsedErr      error
	revokeRefreshID         string
	revokeRefreshAt         time.Time
	revokeRefreshErr        error
}

func (f *fakeSessionRepository) CreateAuthSession(ctx context.Context, db database.DBTX, params CreateAuthSessionParams) (AuthSession, error) {
	f.createAuthSessionCalled = true
	f.createAuthSessionParams = params
	if f.createAuthSessionErr != nil {
		return AuthSession{}, f.createAuthSessionErr
	}
	return AuthSession{
		ID:        params.ID,
		UserID:    params.UserID,
		UserAgent: params.UserAgent,
		IPAddress: params.IPAddress,
		ExpiresAt: params.ExpiresAt,
	}, nil
}

func (f *fakeSessionRepository) GetAuthSessionByID(ctx context.Context, db database.DBTX, id string) (AuthSession, error) {
	f.getAuthSessionID = id
	if f.getAuthSessionErr != nil {
		return AuthSession{}, f.getAuthSessionErr
	}
	if f.getAuthSession.ID == "" {
		return AuthSession{}, ErrAuthSessionNotFound
	}
	return f.getAuthSession, nil
}

func (f *fakeSessionRepository) MarkAuthSessionUsed(ctx context.Context, db database.DBTX, id string, usedAt time.Time) (AuthSession, error) {
	f.markAuthSessionUsedID = id
	f.markAuthSessionUsedAt = usedAt
	if f.markAuthSessionUsedErr != nil {
		return AuthSession{}, f.markAuthSessionUsedErr
	}
	session := f.getAuthSession
	session.LastUsedAt = &usedAt
	return session, nil
}

func (f *fakeSessionRepository) RevokeAuthSession(ctx context.Context, db database.DBTX, id string, revokedAt time.Time) (AuthSession, error) {
	f.revokeAuthSessionID = id
	f.revokeAuthSessionAt = revokedAt
	if f.revokeAuthSessionErr != nil {
		return AuthSession{}, f.revokeAuthSessionErr
	}
	session := f.getAuthSession
	session.RevokedAt = &revokedAt
	return session, nil
}

func (f *fakeSessionRepository) CreateRefreshToken(ctx context.Context, db database.DBTX, params CreateRefreshTokenParams) (RefreshToken, error) {
	f.createRefreshCalled = true
	f.createRefreshParams = params
	if f.createRefreshErr != nil {
		return RefreshToken{}, f.createRefreshErr
	}
	return RefreshToken{
		ID:        params.ID,
		SessionID: params.SessionID,
		TokenHash: params.TokenHash,
		ExpiresAt: params.ExpiresAt,
	}, nil
}

func (f *fakeSessionRepository) GetRefreshTokenByHashForUpdate(ctx context.Context, db database.DBTX, tokenHash string) (RefreshToken, error) {
	f.getRefreshHash = tokenHash
	if f.getRefreshErr != nil {
		return RefreshToken{}, f.getRefreshErr
	}
	if f.getRefreshToken.ID == "" {
		return RefreshToken{}, ErrRefreshTokenNotFound
	}
	return f.getRefreshToken, nil
}

func (f *fakeSessionRepository) MarkRefreshTokenUsed(ctx context.Context, db database.DBTX, id string, usedAt time.Time) (RefreshToken, error) {
	f.markRefreshUsedID = id
	f.markRefreshUsedAt = usedAt
	if f.markRefreshUsedErr != nil {
		return RefreshToken{}, f.markRefreshUsedErr
	}
	token := f.getRefreshToken
	token.LastUsedAt = &usedAt
	return token, nil
}

func (f *fakeSessionRepository) RevokeRefreshToken(ctx context.Context, db database.DBTX, id string, revokedAt time.Time) (RefreshToken, error) {
	f.revokeRefreshID = id
	f.revokeRefreshAt = revokedAt
	if f.revokeRefreshErr != nil {
		return RefreshToken{}, f.revokeRefreshErr
	}
	token := f.getRefreshToken
	token.RevokedAt = &revokedAt
	return token, nil
}

func TestServiceRegister(t *testing.T) {
	t.Parallel()

	users := &fakeUserRepository{}
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      users,
		Passwords:  fakePasswordHasher{},
	})

	result, err := service.Register(context.Background(), RegisterRequest{
		Email:       " User@Example.COM ",
		Password:    "secret123",
		DisplayName: " Test User ",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if result.UserID == "" {
		t.Fatal("expected generated user id")
	}
	if result.Email != "User@Example.COM" {
		t.Fatalf("expected trimmed original email, got %s", result.Email)
	}
	if users.createParams.EmailNormalized != "user@example.com" {
		t.Fatalf("expected normalized email, got %s", users.createParams.EmailNormalized)
	}
	if users.createParams.PasswordHash != "hashed:secret123" {
		t.Fatalf("expected hashed password, got %s", users.createParams.PasswordHash)
	}
	if users.createParams.DisplayName != "Test User" {
		t.Fatalf("expected trimmed display name, got %s", users.createParams.DisplayName)
	}
}

func TestServiceRegisterRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      &fakeUserRepository{},
		Passwords:  fakePasswordHasher{},
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:       "invalid-email",
		Password:    "short",
		DisplayName: "",
	})
	if !errors.Is(err, ErrInvalidRegistrationInput) {
		t.Fatalf("expected ErrInvalidRegistrationInput, got %v", err)
	}
}

func TestServiceRegisterMapsDuplicateEmail(t *testing.T) {
	t.Parallel()

	users := &fakeUserRepository{
		createErr: fmt.Errorf("create user: %w", &pgconn.PgError{
			ConstraintName: usersEmailNormalizedUniqueConstraint,
		}),
	}
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      users,
		Passwords:  fakePasswordHasher{},
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:       "taken@example.com",
		Password:    "secret123",
		DisplayName: "Taken User",
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestServiceRegisterReturnsPasswordHashError(t *testing.T) {
	t.Parallel()

	hashErr := errors.New("hash failed")
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      &fakeUserRepository{},
		Passwords:  fakePasswordHasher{hashErr: hashErr},
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:       "user@example.com",
		Password:    "secret123",
		DisplayName: "Test User",
	})
	if !errors.Is(err, hashErr) {
		t.Fatalf("expected hash error, got %v", err)
	}
}

func TestServiceLogin(t *testing.T) {
	t.Parallel()

	refreshExpiresAt := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	users := &fakeUserRepository{
		getUser: identity.User{
			ID:              "user_123",
			Email:           "User@Example.COM",
			EmailNormalized: "user@example.com",
			PasswordHash:    "hashed:secret123",
			Status:          identity.UserStatusActive,
		},
	}
	tokens := &fakeTokenIssuer{
		issued: IssuedTokenPair{
			Tokens: TokenPair{
				AccessToken:           "access-token",
				AccessTokenExpiresAt:  refreshExpiresAt.Add(-30 * 24 * time.Hour).Add(15 * time.Minute),
				RefreshToken:          "refresh-token",
				RefreshTokenExpiresAt: refreshExpiresAt,
			},
			AccessTokenID:    "access-token-id",
			RefreshTokenHash: "refresh-token-hash",
		},
	}
	sessions := &fakeSessionRepository{}
	service := NewService(ServiceConfig{
		Transactor:  fakeTransactionManager{},
		Users:       users,
		Sessions:    sessions,
		Passwords:   fakePasswordHasher{},
		TokenIssuer: tokens,
	})

	result, err := service.Login(context.Background(), LoginRequest{
		Email:     " User@Example.COM ",
		Password:  "secret123",
		UserAgent: " Test Browser ",
		IPAddress: " 127.0.0.1 ",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if users.getEmailNormalized != "user@example.com" {
		t.Fatalf("expected normalized email lookup, got %s", users.getEmailNormalized)
	}
	if result.UserID != "user_123" {
		t.Fatalf("expected user id, got %s", result.UserID)
	}
	if result.Email != "User@Example.COM" {
		t.Fatalf("expected original stored email, got %s", result.Email)
	}
	if result.Tokens.AccessToken != "access-token" || result.Tokens.RefreshToken != "refresh-token" {
		t.Fatalf("expected issued token pair, got %+v", result.Tokens)
	}
	if !tokens.called {
		t.Fatal("expected token issuer to be called")
	}
	if tokens.params.UserID != "user_123" {
		t.Fatalf("expected token subject user_123, got %s", tokens.params.UserID)
	}
	if tokens.params.SessionID == "" {
		t.Fatal("expected generated session id in token params")
	}
	if !sessions.createAuthSessionCalled {
		t.Fatal("expected auth session to be created")
	}
	if sessions.createAuthSessionParams.ID != tokens.params.SessionID {
		t.Fatalf("expected session id %s, got %s", tokens.params.SessionID, sessions.createAuthSessionParams.ID)
	}
	if sessions.createAuthSessionParams.UserID != "user_123" {
		t.Fatalf("expected session user id, got %s", sessions.createAuthSessionParams.UserID)
	}
	if sessions.createAuthSessionParams.UserAgent == nil || *sessions.createAuthSessionParams.UserAgent != "Test Browser" {
		t.Fatalf("expected trimmed user agent, got %#v", sessions.createAuthSessionParams.UserAgent)
	}
	if sessions.createAuthSessionParams.IPAddress == nil || *sessions.createAuthSessionParams.IPAddress != "127.0.0.1" {
		t.Fatalf("expected trimmed ip address, got %#v", sessions.createAuthSessionParams.IPAddress)
	}
	if !sessions.createAuthSessionParams.ExpiresAt.Equal(refreshExpiresAt) {
		t.Fatalf("expected session expiry %s, got %s", refreshExpiresAt, sessions.createAuthSessionParams.ExpiresAt)
	}
	if !sessions.createRefreshCalled {
		t.Fatal("expected refresh token to be stored")
	}
	if sessions.createRefreshParams.ID == "" {
		t.Fatal("expected generated refresh token id")
	}
	if sessions.createRefreshParams.SessionID != tokens.params.SessionID {
		t.Fatalf("expected refresh token session id %s, got %s", tokens.params.SessionID, sessions.createRefreshParams.SessionID)
	}
	if sessions.createRefreshParams.TokenHash != "refresh-token-hash" {
		t.Fatalf("expected stored refresh token hash, got %s", sessions.createRefreshParams.TokenHash)
	}
	if sessions.createRefreshParams.TokenHash == result.Tokens.RefreshToken {
		t.Fatal("refresh token store must not receive plaintext token")
	}
	if !sessions.createRefreshParams.ExpiresAt.Equal(refreshExpiresAt) {
		t.Fatalf("expected refresh token expiry %s, got %s", refreshExpiresAt, sessions.createRefreshParams.ExpiresAt)
	}
}

func TestServiceLoginRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	users := &fakeUserRepository{}
	service := NewService(ServiceConfig{
		Users:     users,
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "invalid-email",
		Password: "short",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if users.getEmailNormalized != "" {
		t.Fatalf("expected no repository lookup, got %s", users.getEmailNormalized)
	}
}

func TestServiceLoginMapsUnknownEmail(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Users:     &fakeUserRepository{},
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "missing@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestServiceLoginMapsWrongPassword(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Users: &fakeUserRepository{
			getUser: identity.User{
				ID:           "user_123",
				Email:        "user@example.com",
				PasswordHash: "hashed:secret123",
				Status:       identity.UserStatusActive,
			},
		},
		Passwords: fakePasswordHasher{compareErr: ErrInvalidCredentials},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestServiceLoginRejectsDisabledUser(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Users: &fakeUserRepository{
			getUser: identity.User{
				ID:           "user_123",
				Email:        "user@example.com",
				PasswordHash: "hashed:secret123",
				Status:       identity.UserStatusDisabled,
			},
		},
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
}

func TestServiceLoginReturnsRepositoryError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("database offline")
	service := NewService(ServiceConfig{
		Users:     &fakeUserRepository{getErr: repoErr},
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestServiceLoginReturnsTokenIssuerError(t *testing.T) {
	t.Parallel()

	tokenErr := errors.New("token issuer offline")
	sessions := &fakeSessionRepository{}
	service := NewService(ServiceConfig{
		Users: &fakeUserRepository{
			getUser: identity.User{
				ID:           "user_123",
				Email:        "user@example.com",
				PasswordHash: "hashed:secret123",
				Status:       identity.UserStatusActive,
			},
		},
		Sessions:    sessions,
		Passwords:   fakePasswordHasher{},
		TokenIssuer: &fakeTokenIssuer{err: tokenErr},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, tokenErr) {
		t.Fatalf("expected token issuer error, got %v", err)
	}
	if sessions.createAuthSessionCalled || sessions.createRefreshCalled {
		t.Fatal("expected no session writes when token issuing fails")
	}
}

func TestServiceLoginReturnsSessionStoreError(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("session store failed")
	sessions := &fakeSessionRepository{createAuthSessionErr: storeErr}
	service := newServiceForLoginStoreFailure(sessions)

	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected session store error, got %v", err)
	}
	if result.Tokens.AccessToken != "" || result.Tokens.RefreshToken != "" {
		t.Fatalf("expected no returned tokens on store failure, got %+v", result.Tokens)
	}
	if sessions.createRefreshCalled {
		t.Fatal("expected no refresh token write when session creation fails")
	}
}

func TestServiceLoginReturnsRefreshTokenStoreError(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("refresh token store failed")
	sessions := &fakeSessionRepository{createRefreshErr: storeErr}
	service := newServiceForLoginStoreFailure(sessions)

	result, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected refresh token store error, got %v", err)
	}
	if result.Tokens.AccessToken != "" || result.Tokens.RefreshToken != "" {
		t.Fatalf("expected no returned tokens on store failure, got %+v", result.Tokens)
	}
	if !sessions.createAuthSessionCalled {
		t.Fatal("expected session write before refresh token write")
	}
}

func TestServiceRefreshToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	sessionExpiresAt := now.Add(time.Hour)
	oldRefreshToken := "old-refresh-token"
	oldRefreshTokenID := "old_refresh_token_123"
	sessions := &fakeSessionRepository{
		getRefreshToken: RefreshToken{
			ID:        oldRefreshTokenID,
			SessionID: "session_123",
			TokenHash: HashRefreshToken(oldRefreshToken),
			ExpiresAt: now.Add(30 * time.Minute),
		},
		getAuthSession: AuthSession{
			ID:        "session_123",
			UserID:    "user_123",
			ExpiresAt: sessionExpiresAt,
		},
	}
	users := &fakeUserRepository{
		getByIDUser: identity.User{
			ID:     "user_123",
			Email:  "user@example.com",
			Status: identity.UserStatusActive,
		},
	}
	tokens := &fakeTokenIssuer{
		issued: IssuedTokenPair{
			Tokens: TokenPair{
				AccessToken:           "new-access-token",
				AccessTokenExpiresAt:  now.Add(15 * time.Minute),
				RefreshToken:          "new-refresh-token",
				RefreshTokenExpiresAt: sessionExpiresAt,
			},
			AccessTokenID:    "new-access-token-id",
			RefreshTokenHash: "new-refresh-token-hash",
		},
	}
	service := newServiceForRefreshToken(now, users, sessions, tokens)

	result, err := service.RefreshToken(context.Background(), RefreshTokenRequest{
		RefreshToken: " " + oldRefreshToken + " ",
	})
	if err != nil {
		t.Fatalf("refresh token: %v", err)
	}

	if sessions.getRefreshHash != HashRefreshToken(oldRefreshToken) {
		t.Fatalf("expected refresh token hash lookup, got %s", sessions.getRefreshHash)
	}
	if sessions.getAuthSessionID != "session_123" {
		t.Fatalf("expected session lookup, got %s", sessions.getAuthSessionID)
	}
	if users.getID != "user_123" {
		t.Fatalf("expected user lookup, got %s", users.getID)
	}
	if !tokens.called {
		t.Fatal("expected token issuer to be called")
	}
	if tokens.params.UserID != "user_123" || tokens.params.SessionID != "session_123" {
		t.Fatalf("unexpected token subject: %+v", tokens.params)
	}
	if !tokens.params.SessionExpiresAt.Equal(sessionExpiresAt) {
		t.Fatalf("expected session expiry cap %s, got %s", sessionExpiresAt, tokens.params.SessionExpiresAt)
	}
	if sessions.markRefreshUsedID != oldRefreshTokenID || !sessions.markRefreshUsedAt.Equal(now) {
		t.Fatalf("expected old refresh token marked used at %s, got %s %s", now, sessions.markRefreshUsedID, sessions.markRefreshUsedAt)
	}
	if !sessions.createRefreshCalled {
		t.Fatal("expected rotated refresh token to be stored")
	}
	if sessions.createRefreshParams.ID == "" {
		t.Fatal("expected generated refresh token id")
	}
	if sessions.createRefreshParams.SessionID != "session_123" {
		t.Fatalf("expected refresh session id, got %s", sessions.createRefreshParams.SessionID)
	}
	if sessions.createRefreshParams.TokenHash != "new-refresh-token-hash" {
		t.Fatalf("expected new refresh token hash, got %s", sessions.createRefreshParams.TokenHash)
	}
	if sessions.createRefreshParams.TokenHash == result.Tokens.RefreshToken {
		t.Fatal("refresh token store must not receive plaintext token")
	}
	if sessions.createRefreshParams.RotatedFromTokenID == nil || *sessions.createRefreshParams.RotatedFromTokenID != oldRefreshTokenID {
		t.Fatalf("expected rotation parent %s, got %v", oldRefreshTokenID, sessions.createRefreshParams.RotatedFromTokenID)
	}
	if !sessions.createRefreshParams.ExpiresAt.Equal(sessionExpiresAt) {
		t.Fatalf("expected new refresh expiry %s, got %s", sessionExpiresAt, sessions.createRefreshParams.ExpiresAt)
	}
	if sessions.markAuthSessionUsedID != "session_123" || !sessions.markAuthSessionUsedAt.Equal(now) {
		t.Fatalf("expected session last used update at %s, got %s %s", now, sessions.markAuthSessionUsedID, sessions.markAuthSessionUsedAt)
	}
	if result.UserID != "user_123" || result.Email != "user@example.com" {
		t.Fatalf("unexpected refresh result user: %+v", result)
	}
	if result.Tokens.AccessToken != "new-access-token" || result.Tokens.RefreshToken != "new-refresh-token" {
		t.Fatalf("expected new token pair, got %+v", result.Tokens)
	}
}

func TestServiceRefreshTokenRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionRepository{}
	tokens := &fakeTokenIssuer{}
	service := newServiceForRefreshToken(time.Now(), &fakeUserRepository{}, sessions, tokens)

	_, err := service.RefreshToken(context.Background(), RefreshTokenRequest{
		RefreshToken: " ",
	})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
	if sessions.getRefreshHash != "" {
		t.Fatalf("expected no token lookup, got %s", sessions.getRefreshHash)
	}
	if tokens.called {
		t.Fatal("expected no token issuing")
	}
}

func TestServiceRefreshTokenRejectsInvalidTokenState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	usedAt := now.Add(-time.Minute)
	revokedAt := now.Add(-time.Minute)

	tests := []struct {
		name  string
		token RefreshToken
	}{
		{
			name: "missing token",
		},
		{
			name: "used token",
			token: RefreshToken{
				ID:         "old_refresh_token_123",
				SessionID:  "session_123",
				TokenHash:  HashRefreshToken("old-refresh-token"),
				ExpiresAt:  now.Add(time.Hour),
				LastUsedAt: &usedAt,
			},
		},
		{
			name: "revoked token",
			token: RefreshToken{
				ID:        "old_refresh_token_123",
				SessionID: "session_123",
				TokenHash: HashRefreshToken("old-refresh-token"),
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: &revokedAt,
			},
		},
		{
			name: "expired token",
			token: RefreshToken{
				ID:        "old_refresh_token_123",
				SessionID: "session_123",
				TokenHash: HashRefreshToken("old-refresh-token"),
				ExpiresAt: now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sessions := &fakeSessionRepository{getRefreshToken: tt.token}
			tokens := &fakeTokenIssuer{}
			service := newServiceForRefreshToken(now, &fakeUserRepository{}, sessions, tokens)

			_, err := service.RefreshToken(context.Background(), RefreshTokenRequest{
				RefreshToken: "old-refresh-token",
			})
			if !errors.Is(err, ErrInvalidRefreshToken) {
				t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
			}
			if sessions.getAuthSessionID != "" {
				t.Fatalf("expected no session lookup, got %s", sessions.getAuthSessionID)
			}
			if tokens.called {
				t.Fatal("expected no token issuing")
			}
			if sessions.markRefreshUsedID != "" || sessions.createRefreshCalled {
				t.Fatal("expected no refresh token writes")
			}
		})
	}
}

func TestServiceRefreshTokenRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Minute)

	tests := []struct {
		name    string
		session AuthSession
	}{
		{
			name: "missing session",
		},
		{
			name: "expired session",
			session: AuthSession{
				ID:        "session_123",
				UserID:    "user_123",
				ExpiresAt: now,
			},
		},
		{
			name: "revoked session",
			session: AuthSession{
				ID:        "session_123",
				UserID:    "user_123",
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: &revokedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sessions := &fakeSessionRepository{
				getRefreshToken: RefreshToken{
					ID:        "old_refresh_token_123",
					SessionID: "session_123",
					TokenHash: HashRefreshToken("old-refresh-token"),
					ExpiresAt: now.Add(time.Hour),
				},
				getAuthSession: tt.session,
			}
			tokens := &fakeTokenIssuer{}
			service := newServiceForRefreshToken(now, &fakeUserRepository{}, sessions, tokens)

			_, err := service.RefreshToken(context.Background(), RefreshTokenRequest{
				RefreshToken: "old-refresh-token",
			})
			if !errors.Is(err, ErrInvalidRefreshToken) {
				t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
			}
			if tokens.called {
				t.Fatal("expected no token issuing")
			}
			if sessions.markRefreshUsedID != "" || sessions.createRefreshCalled {
				t.Fatal("expected no refresh token writes")
			}
		})
	}
}

func TestServiceRefreshTokenRejectsDisabledUser(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	sessions := newValidRefreshSessionRepository(now)
	users := &fakeUserRepository{
		getByIDUser: identity.User{
			ID:     "user_123",
			Email:  "user@example.com",
			Status: identity.UserStatusDisabled,
		},
	}
	tokens := &fakeTokenIssuer{}
	service := newServiceForRefreshToken(now, users, sessions, tokens)

	_, err := service.RefreshToken(context.Background(), RefreshTokenRequest{
		RefreshToken: "old-refresh-token",
	})
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
	if tokens.called {
		t.Fatal("expected no token issuing")
	}
	if sessions.markRefreshUsedID != "" || sessions.createRefreshCalled {
		t.Fatal("expected no refresh token writes")
	}
}

func TestServiceRefreshTokenReturnsTokenIssuerError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	tokenErr := errors.New("token issuer failed")
	sessions := newValidRefreshSessionRepository(now)
	users := newValidRefreshUserRepository()
	service := newServiceForRefreshToken(now, users, sessions, &fakeTokenIssuer{err: tokenErr})

	_, err := service.RefreshToken(context.Background(), RefreshTokenRequest{
		RefreshToken: "old-refresh-token",
	})
	if !errors.Is(err, tokenErr) {
		t.Fatalf("expected token issuer error, got %v", err)
	}
	if sessions.markRefreshUsedID != "" || sessions.createRefreshCalled {
		t.Fatal("expected no refresh token writes when token issuing fails")
	}
}

func TestServiceRefreshTokenReturnsStoreError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	storeErr := errors.New("store rotated refresh token failed")
	sessions := newValidRefreshSessionRepository(now)
	sessions.createRefreshErr = storeErr
	users := newValidRefreshUserRepository()
	tokens := newValidRefreshTokenIssuer(now)
	service := newServiceForRefreshToken(now, users, sessions, tokens)

	result, err := service.RefreshToken(context.Background(), RefreshTokenRequest{
		RefreshToken: "old-refresh-token",
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected store error, got %v", err)
	}
	if result.Tokens.AccessToken != "" || result.Tokens.RefreshToken != "" {
		t.Fatalf("expected no returned tokens on store failure, got %+v", result.Tokens)
	}
	if sessions.markRefreshUsedID != "old_refresh_token_123" {
		t.Fatalf("expected old token mark attempt, got %s", sessions.markRefreshUsedID)
	}
	if sessions.markAuthSessionUsedID != "" {
		t.Fatalf("expected no session last used update after refresh store failure, got %s", sessions.markAuthSessionUsedID)
	}
}

func TestServiceLogout(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	refreshToken := "refresh-token"
	sessions := &fakeSessionRepository{
		getRefreshToken: RefreshToken{
			ID:        "refresh_token_123",
			SessionID: "session_123",
			TokenHash: HashRefreshToken(refreshToken),
			ExpiresAt: now.Add(time.Hour),
		},
		getAuthSession: AuthSession{
			ID:        "session_123",
			UserID:    "user_123",
			ExpiresAt: now.Add(time.Hour),
		},
	}
	service := newServiceForRefreshToken(now, &fakeUserRepository{}, sessions, &fakeTokenIssuer{})

	err := service.Logout(context.Background(), LogoutRequest{
		RefreshToken: " " + refreshToken + " ",
	})
	if err != nil {
		t.Fatalf("logout: %v", err)
	}

	if sessions.getRefreshHash != HashRefreshToken(refreshToken) {
		t.Fatalf("expected refresh token hash lookup, got %s", sessions.getRefreshHash)
	}
	if sessions.getAuthSessionID != "session_123" {
		t.Fatalf("expected session lookup, got %s", sessions.getAuthSessionID)
	}
	if sessions.revokeRefreshID != "refresh_token_123" || !sessions.revokeRefreshAt.Equal(now) {
		t.Fatalf("expected refresh token revoked at %s, got %s %s", now, sessions.revokeRefreshID, sessions.revokeRefreshAt)
	}
	if sessions.revokeAuthSessionID != "session_123" || !sessions.revokeAuthSessionAt.Equal(now) {
		t.Fatalf("expected session revoked at %s, got %s %s", now, sessions.revokeAuthSessionID, sessions.revokeAuthSessionAt)
	}
}

func TestServiceLogoutRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionRepository{}
	service := newServiceForRefreshToken(time.Now(), &fakeUserRepository{}, sessions, &fakeTokenIssuer{})

	err := service.Logout(context.Background(), LogoutRequest{
		RefreshToken: " ",
	})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
	if sessions.getRefreshHash != "" {
		t.Fatalf("expected no token lookup, got %s", sessions.getRefreshHash)
	}
}

func TestServiceLogoutIsIdempotentForMissingToken(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionRepository{}
	service := newServiceForRefreshToken(time.Now(), &fakeUserRepository{}, sessions, &fakeTokenIssuer{})

	err := service.Logout(context.Background(), LogoutRequest{
		RefreshToken: "missing-refresh-token",
	})
	if err != nil {
		t.Fatalf("expected missing token logout to succeed, got %v", err)
	}
	if sessions.getAuthSessionID != "" {
		t.Fatalf("expected no session lookup, got %s", sessions.getAuthSessionID)
	}
	if sessions.revokeRefreshID != "" || sessions.revokeAuthSessionID != "" {
		t.Fatal("expected no revoke writes for missing token")
	}
}

func TestServiceLogoutSkipsAlreadyRevokedState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Minute)
	sessions := &fakeSessionRepository{
		getRefreshToken: RefreshToken{
			ID:        "refresh_token_123",
			SessionID: "session_123",
			TokenHash: HashRefreshToken("refresh-token"),
			ExpiresAt: now.Add(time.Hour),
			RevokedAt: &revokedAt,
		},
		getAuthSession: AuthSession{
			ID:        "session_123",
			UserID:    "user_123",
			ExpiresAt: now.Add(time.Hour),
			RevokedAt: &revokedAt,
		},
	}
	service := newServiceForRefreshToken(now, &fakeUserRepository{}, sessions, &fakeTokenIssuer{})

	err := service.Logout(context.Background(), LogoutRequest{
		RefreshToken: "refresh-token",
	})
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if sessions.revokeRefreshID != "" {
		t.Fatalf("expected no refresh revoke write, got %s", sessions.revokeRefreshID)
	}
	if sessions.revokeAuthSessionID != "" {
		t.Fatalf("expected no session revoke write, got %s", sessions.revokeAuthSessionID)
	}
}

func TestServiceLogoutReturnsRevokeError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	revokeErr := errors.New("revoke failed")
	sessions := &fakeSessionRepository{
		getRefreshToken: RefreshToken{
			ID:        "refresh_token_123",
			SessionID: "session_123",
			TokenHash: HashRefreshToken("refresh-token"),
			ExpiresAt: now.Add(time.Hour),
		},
		getAuthSession: AuthSession{
			ID:        "session_123",
			UserID:    "user_123",
			ExpiresAt: now.Add(time.Hour),
		},
		revokeRefreshErr: revokeErr,
	}
	service := newServiceForRefreshToken(now, &fakeUserRepository{}, sessions, &fakeTokenIssuer{})

	err := service.Logout(context.Background(), LogoutRequest{
		RefreshToken: "refresh-token",
	})
	if !errors.Is(err, revokeErr) {
		t.Fatalf("expected revoke error, got %v", err)
	}
	if sessions.revokeAuthSessionID != "" {
		t.Fatalf("expected no session revoke after refresh revoke failure, got %s", sessions.revokeAuthSessionID)
	}
}

func newServiceForLoginStoreFailure(sessions *fakeSessionRepository) Service {
	refreshExpiresAt := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	return NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users: &fakeUserRepository{
			getUser: identity.User{
				ID:           "user_123",
				Email:        "user@example.com",
				PasswordHash: "hashed:secret123",
				Status:       identity.UserStatusActive,
			},
		},
		Sessions:  sessions,
		Passwords: fakePasswordHasher{},
		TokenIssuer: &fakeTokenIssuer{
			issued: IssuedTokenPair{
				Tokens: TokenPair{
					AccessToken:           "access-token",
					AccessTokenExpiresAt:  refreshExpiresAt.Add(-30 * 24 * time.Hour).Add(15 * time.Minute),
					RefreshToken:          "refresh-token",
					RefreshTokenExpiresAt: refreshExpiresAt,
				},
				AccessTokenID:    "access-token-id",
				RefreshTokenHash: "refresh-token-hash",
			},
		},
	})
}

func newServiceForRefreshToken(now time.Time, users *fakeUserRepository, sessions *fakeSessionRepository, tokens *fakeTokenIssuer) Service {
	return NewService(ServiceConfig{
		Transactor:  fakeTransactionManager{},
		Users:       users,
		Sessions:    sessions,
		Passwords:   fakePasswordHasher{},
		TokenIssuer: tokens,
		Clock: func() time.Time {
			return now
		},
	})
}

func newValidRefreshSessionRepository(now time.Time) *fakeSessionRepository {
	return &fakeSessionRepository{
		getRefreshToken: RefreshToken{
			ID:        "old_refresh_token_123",
			SessionID: "session_123",
			TokenHash: HashRefreshToken("old-refresh-token"),
			ExpiresAt: now.Add(time.Hour),
		},
		getAuthSession: AuthSession{
			ID:        "session_123",
			UserID:    "user_123",
			ExpiresAt: now.Add(time.Hour),
		},
	}
}

func newValidRefreshUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		getByIDUser: identity.User{
			ID:     "user_123",
			Email:  "user@example.com",
			Status: identity.UserStatusActive,
		},
	}
}

func newValidRefreshTokenIssuer(now time.Time) *fakeTokenIssuer {
	return &fakeTokenIssuer{
		issued: IssuedTokenPair{
			Tokens: TokenPair{
				AccessToken:           "new-access-token",
				AccessTokenExpiresAt:  now.Add(15 * time.Minute),
				RefreshToken:          "new-refresh-token",
				RefreshTokenExpiresAt: now.Add(time.Hour),
			},
			AccessTokenID:    "new-access-token-id",
			RefreshTokenHash: "new-refresh-token-hash",
		},
	}
}
