package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTokenManagerIssueTokenPair(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	secret := strings.Repeat("s", MinAccessTokenSecretBytes)
	randomBytes := make([]byte, accessTokenIDBytes+RefreshTokenEntropyBytes)
	for i := range randomBytes {
		randomBytes[i] = byte(i + 1)
	}

	manager, err := NewTokenManager(TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: secret,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		Clock: func() time.Time {
			return now
		},
		Random: bytes.NewReader(randomBytes),
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	issued, err := manager.IssueTokenPair(context.Background(), IssueTokenPairParams{
		UserID:    "user_123",
		SessionID: "session_123",
	})
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	if issued.AccessTokenID == "" {
		t.Fatal("expected access token id")
	}
	if issued.Tokens.AccessToken == "" {
		t.Fatal("expected access token")
	}
	if issued.Tokens.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}
	if issued.RefreshTokenHash == "" || issued.RefreshTokenHash == issued.Tokens.RefreshToken {
		t.Fatal("expected refresh token hash to be stored separately from plaintext token")
	}
	if issued.RefreshTokenHash != HashRefreshToken(issued.Tokens.RefreshToken) {
		t.Fatal("expected refresh token hash to match plaintext token")
	}
	if !issued.Tokens.AccessTokenExpiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("unexpected access token expiry: %s", issued.Tokens.AccessTokenExpiresAt)
	}
	if !issued.Tokens.RefreshTokenExpiresAt.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatalf("unexpected refresh token expiry: %s", issued.Tokens.RefreshTokenExpiresAt)
	}

	parts := strings.Split(issued.Tokens.AccessToken, ".")
	if len(parts) != 3 {
		t.Fatalf("expected jwt with 3 segments, got %d", len(parts))
	}

	var header jwtHeader
	decodeJWTSegmentForTest(t, parts[0], &header)
	if header.Algorithm != jwtAlgorithmHS256 || header.Type != jwtType {
		t.Fatalf("unexpected jwt header: %+v", header)
	}

	var claims accessTokenClaims
	decodeJWTSegmentForTest(t, parts[1], &claims)
	if claims.TokenType != accessTokenType {
		t.Fatalf("expected access token type, got %s", claims.TokenType)
	}
	if claims.Issuer != "corebe-api" {
		t.Fatalf("expected issuer, got %s", claims.Issuer)
	}
	if claims.Audience != "corebe-api" {
		t.Fatalf("expected audience, got %s", claims.Audience)
	}
	if claims.Subject != "user_123" {
		t.Fatalf("expected subject, got %s", claims.Subject)
	}
	if claims.SessionID != "session_123" {
		t.Fatalf("expected session id, got %s", claims.SessionID)
	}
	if claims.JWTID != issued.AccessTokenID {
		t.Fatalf("expected jwt id %s, got %s", issued.AccessTokenID, claims.JWTID)
	}
	if claims.IssuedAt != now.Unix() || claims.ExpiresAt != now.Add(15*time.Minute).Unix() {
		t.Fatalf("unexpected token times: %+v", claims)
	}

	verifyJWTSignatureForTest(t, issued.Tokens.AccessToken, secret)
}

func TestNewTokenManagerRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	valid := TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: strings.Repeat("s", MinAccessTokenSecretBytes),
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
	}

	tests := []struct {
		name   string
		mutate func(*TokenConfig)
	}{
		{
			name: "missing issuer",
			mutate: func(cfg *TokenConfig) {
				cfg.Issuer = " "
			},
		},
		{
			name: "missing audience",
			mutate: func(cfg *TokenConfig) {
				cfg.Audience = ""
			},
		},
		{
			name: "short secret",
			mutate: func(cfg *TokenConfig) {
				cfg.AccessTokenSecret = "short"
			},
		},
		{
			name: "non positive access ttl",
			mutate: func(cfg *TokenConfig) {
				cfg.AccessTokenTTL = 0
			},
		},
		{
			name: "refresh ttl not longer than access ttl",
			mutate: func(cfg *TokenConfig) {
				cfg.RefreshTokenTTL = cfg.AccessTokenTTL
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := valid
			tt.mutate(&cfg)

			_, err := NewTokenManager(cfg)
			if !errors.Is(err, ErrInvalidTokenConfig) {
				t.Fatalf("expected ErrInvalidTokenConfig, got %v", err)
			}
		})
	}
}

func TestTokenManagerIssueTokenPairCapsExpiryAtSessionExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	sessionExpiresAt := now.Add(10 * time.Minute)
	manager, err := NewTokenManager(TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: strings.Repeat("s", MinAccessTokenSecretBytes),
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		Clock: func() time.Time {
			return now
		},
		Random: bytes.NewReader(make([]byte, accessTokenIDBytes+RefreshTokenEntropyBytes)),
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	issued, err := manager.IssueTokenPair(context.Background(), IssueTokenPairParams{
		UserID:           "user_123",
		SessionID:        "session_123",
		SessionExpiresAt: sessionExpiresAt,
	})
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	if !issued.Tokens.AccessTokenExpiresAt.Equal(sessionExpiresAt) {
		t.Fatalf("expected capped access expiry %s, got %s", sessionExpiresAt, issued.Tokens.AccessTokenExpiresAt)
	}
	if !issued.Tokens.RefreshTokenExpiresAt.Equal(sessionExpiresAt) {
		t.Fatalf("expected capped refresh expiry %s, got %s", sessionExpiresAt, issued.Tokens.RefreshTokenExpiresAt)
	}

	parts := strings.Split(issued.Tokens.AccessToken, ".")
	var claims accessTokenClaims
	decodeJWTSegmentForTest(t, parts[1], &claims)
	if claims.ExpiresAt != sessionExpiresAt.Unix() {
		t.Fatalf("expected capped jwt exp %d, got %d", sessionExpiresAt.Unix(), claims.ExpiresAt)
	}
}

func TestTokenManagerVerifyAccessToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	secret := strings.Repeat("s", MinAccessTokenSecretBytes)
	manager, err := NewTokenManager(TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: secret,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		Clock: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	issued, err := manager.IssueTokenPair(context.Background(), IssueTokenPairParams{
		UserID:    "user_123",
		SessionID: "session_123",
	})
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	user, err := manager.VerifyAccessToken(context.Background(), issued.Tokens.AccessToken)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}

	if user.UserID != "user_123" {
		t.Fatalf("expected user id, got %s", user.UserID)
	}
	if user.SessionID != "session_123" {
		t.Fatalf("expected session id, got %s", user.SessionID)
	}
	if user.AccessTokenID != issued.AccessTokenID {
		t.Fatalf("expected access token id %s, got %s", issued.AccessTokenID, user.AccessTokenID)
	}
	if !user.IssuedAt.Equal(now) {
		t.Fatalf("expected issued at %s, got %s", now, user.IssuedAt)
	}
	if !user.ExpiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("expected expiry %s, got %s", now.Add(15*time.Minute), user.ExpiresAt)
	}
}

func TestTokenManagerVerifyAccessTokenRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	secret := strings.Repeat("s", MinAccessTokenSecretBytes)
	manager, err := NewTokenManager(TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: secret,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		Clock: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	issued, err := manager.IssueTokenPair(context.Background(), IssueTokenPairParams{
		UserID:    "user_123",
		SessionID: "session_123",
	})
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	wrongIssuer, err := NewTokenManager(TokenConfig{
		Issuer:            "other-api",
		Audience:          "corebe-api",
		AccessTokenSecret: secret,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		Clock: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("new wrong issuer token manager: %v", err)
	}

	expiredNow := now.Add(16 * time.Minute)
	expiredVerifier, err := NewTokenManager(TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: secret,
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		Clock: func() time.Time {
			return expiredNow
		},
	})
	if err != nil {
		t.Fatalf("new expired verifier token manager: %v", err)
	}

	tests := []struct {
		name     string
		verifier TokenManager
		token    string
	}{
		{
			name:     "malformed",
			verifier: manager,
			token:    "not-a-jwt",
		},
		{
			name:     "invalid signature",
			verifier: manager,
			token:    issued.Tokens.AccessToken + "tampered",
		},
		{
			name:     "wrong issuer",
			verifier: wrongIssuer,
			token:    issued.Tokens.AccessToken,
		},
		{
			name:     "expired",
			verifier: expiredVerifier,
			token:    issued.Tokens.AccessToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.verifier.VerifyAccessToken(context.Background(), tt.token)
			if !errors.Is(err, ErrInvalidAccessToken) {
				t.Fatalf("expected ErrInvalidAccessToken, got %v", err)
			}
		})
	}
}

func TestTokenManagerIssueTokenPairRejectsInvalidSubject(t *testing.T) {
	t.Parallel()

	manager := newTestTokenManager(t)

	_, err := manager.IssueTokenPair(context.Background(), IssueTokenPairParams{
		UserID:    "",
		SessionID: "session_123",
	})
	if !errors.Is(err, ErrInvalidTokenSubject) {
		t.Fatalf("expected ErrInvalidTokenSubject, got %v", err)
	}
}

func TestTokenManagerIssueTokenPairReturnsRandomError(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager(TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: strings.Repeat("s", MinAccessTokenSecretBytes),
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
		Random:            failingRandomReader{},
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}

	_, err = manager.IssueTokenPair(context.Background(), IssueTokenPairParams{
		UserID:    "user_123",
		SessionID: "session_123",
	})
	if !errors.Is(err, ErrTokenGenerationFailed) {
		t.Fatalf("expected ErrTokenGenerationFailed, got %v", err)
	}
}

func TestHashRefreshToken(t *testing.T) {
	t.Parallel()

	hash := HashRefreshToken("refresh-token")
	if hash == "" {
		t.Fatal("expected refresh token hash")
	}
	if hash == "refresh-token" {
		t.Fatal("refresh token hash must not equal plaintext token")
	}
	if hash != HashRefreshToken("refresh-token") {
		t.Fatal("expected refresh token hash to be stable")
	}
	if hash == HashRefreshToken("other-refresh-token") {
		t.Fatal("expected different refresh tokens to produce different hashes")
	}
}

func newTestTokenManager(t *testing.T) TokenManager {
	t.Helper()

	manager, err := NewTokenManager(TokenConfig{
		Issuer:            "corebe-api",
		Audience:          "corebe-api",
		AccessTokenSecret: strings.Repeat("s", MinAccessTokenSecretBytes),
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   30 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	return manager
}

func decodeJWTSegmentForTest(t *testing.T, segment string, target any) {
	t.Helper()

	payload, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatalf("decode jwt segment: %v", err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("unmarshal jwt segment: %v", err)
	}
}

func verifyJWTSignatureForTest(t *testing.T, token string, secret string) {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected jwt with 3 segments, got %d", len(parts))
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		t.Fatal("jwt signature mismatch")
	}
}

type failingRandomReader struct{}

func (failingRandomReader) Read(p []byte) (int, error) {
	return 0, errors.New("random failed")
}
