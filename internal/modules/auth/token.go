package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	MinAccessTokenSecretBytes = 32
	RefreshTokenEntropyBytes  = 32

	accessTokenIDBytes = 16
	accessTokenType    = "access"
	jwtAlgorithmHS256  = "HS256"
	jwtType            = "JWT"
)

type TokenConfig struct {
	Issuer            string
	Audience          string
	AccessTokenSecret string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration

	Clock  func() time.Time
	Random io.Reader
}

type TokenManager struct {
	issuer            string
	audience          string
	accessTokenSecret []byte
	accessTokenTTL    time.Duration
	refreshTokenTTL   time.Duration
	clock             func() time.Time
	random            io.Reader
}

var _ TokenIssuer = TokenManager{}
var _ TokenVerifier = TokenManager{}

func NewTokenManager(cfg TokenConfig) (TokenManager, error) {
	issuer := strings.TrimSpace(cfg.Issuer)
	if issuer == "" {
		return TokenManager{}, fmt.Errorf("%w: issuer is required", ErrInvalidTokenConfig)
	}

	audience := strings.TrimSpace(cfg.Audience)
	if audience == "" {
		return TokenManager{}, fmt.Errorf("%w: audience is required", ErrInvalidTokenConfig)
	}

	secret := []byte(cfg.AccessTokenSecret)
	if len(secret) < MinAccessTokenSecretBytes {
		return TokenManager{}, fmt.Errorf("%w: access token secret must be at least %d bytes", ErrInvalidTokenConfig, MinAccessTokenSecretBytes)
	}

	if cfg.AccessTokenTTL <= 0 {
		return TokenManager{}, fmt.Errorf("%w: access token ttl must be positive", ErrInvalidTokenConfig)
	}

	if cfg.RefreshTokenTTL <= cfg.AccessTokenTTL {
		return TokenManager{}, fmt.Errorf("%w: refresh token ttl must be longer than access token ttl", ErrInvalidTokenConfig)
	}

	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}

	random := cfg.Random
	if random == nil {
		random = rand.Reader
	}

	return TokenManager{
		issuer:            issuer,
		audience:          audience,
		accessTokenSecret: secret,
		accessTokenTTL:    cfg.AccessTokenTTL,
		refreshTokenTTL:   cfg.RefreshTokenTTL,
		clock:             clock,
		random:            random,
	}, nil
}

func (m TokenManager) IssueTokenPair(ctx context.Context, params IssueTokenPairParams) (IssuedTokenPair, error) {
	if err := ctx.Err(); err != nil {
		return IssuedTokenPair{}, err
	}

	userID := strings.TrimSpace(params.UserID)
	sessionID := strings.TrimSpace(params.SessionID)
	if userID == "" || sessionID == "" {
		return IssuedTokenPair{}, ErrInvalidTokenSubject
	}

	now := m.clock().UTC()
	sessionExpiresAt := params.SessionExpiresAt.UTC()
	if !params.SessionExpiresAt.IsZero() && !sessionExpiresAt.After(now) {
		return IssuedTokenPair{}, ErrInvalidTokenSubject
	}

	accessTokenID, err := randomTokenString(m.random, accessTokenIDBytes)
	if err != nil {
		return IssuedTokenPair{}, err
	}

	accessTokenExpiresAt := now.Add(m.accessTokenTTL)
	refreshTokenExpiresAt := now.Add(m.refreshTokenTTL)
	if !params.SessionExpiresAt.IsZero() {
		accessTokenExpiresAt = minTime(accessTokenExpiresAt, sessionExpiresAt)
		refreshTokenExpiresAt = minTime(refreshTokenExpiresAt, sessionExpiresAt)
	}

	accessToken, err := signAccessToken(m.accessTokenSecret, accessTokenClaims{
		TokenType: accessTokenType,
		Issuer:    m.issuer,
		Subject:   userID,
		Audience:  m.audience,
		SessionID: sessionID,
		ExpiresAt: accessTokenExpiresAt.Unix(),
		IssuedAt:  now.Unix(),
		JWTID:     accessTokenID,
	})
	if err != nil {
		return IssuedTokenPair{}, err
	}

	refreshToken, err := randomTokenString(m.random, RefreshTokenEntropyBytes)
	if err != nil {
		return IssuedTokenPair{}, err
	}

	return IssuedTokenPair{
		Tokens: TokenPair{
			AccessToken:           accessToken,
			AccessTokenExpiresAt:  accessTokenExpiresAt,
			RefreshToken:          refreshToken,
			RefreshTokenExpiresAt: refreshTokenExpiresAt,
		},
		AccessTokenID:    accessTokenID,
		RefreshTokenHash: HashRefreshToken(refreshToken),
	}, nil
}

func (m TokenManager) VerifyAccessToken(ctx context.Context, accessToken string) (AuthenticatedUser, error) {
	if err := ctx.Err(); err != nil {
		return AuthenticatedUser{}, err
	}

	token := strings.TrimSpace(accessToken)
	if token == "" {
		return AuthenticatedUser{}, ErrInvalidAccessToken
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AuthenticatedUser{}, ErrInvalidAccessToken
	}

	var header jwtHeader
	if err := decodeJWTSegment(parts[0], &header); err != nil {
		return AuthenticatedUser{}, ErrInvalidAccessToken
	}
	if header.Algorithm != jwtAlgorithmHS256 || header.Type != jwtType {
		return AuthenticatedUser{}, ErrInvalidAccessToken
	}

	expectedSignature := signJWTInput(m.accessTokenSecret, parts[0]+"."+parts[1])
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return AuthenticatedUser{}, ErrInvalidAccessToken
	}

	var claims accessTokenClaims
	if err := decodeJWTSegment(parts[1], &claims); err != nil {
		return AuthenticatedUser{}, ErrInvalidAccessToken
	}

	now := m.clock().UTC()
	if !m.validAccessTokenClaims(claims, now) {
		return AuthenticatedUser{}, ErrInvalidAccessToken
	}

	return AuthenticatedUser{
		UserID:        claims.Subject,
		SessionID:     claims.SessionID,
		AccessTokenID: claims.JWTID,
		IssuedAt:      time.Unix(claims.IssuedAt, 0).UTC(),
		ExpiresAt:     time.Unix(claims.ExpiresAt, 0).UTC(),
	}, nil
}

func (m TokenManager) validAccessTokenClaims(claims accessTokenClaims, now time.Time) bool {
	return claims.TokenType == accessTokenType &&
		claims.Issuer == m.issuer &&
		claims.Audience == m.audience &&
		strings.TrimSpace(claims.Subject) != "" &&
		strings.TrimSpace(claims.SessionID) != "" &&
		strings.TrimSpace(claims.JWTID) != "" &&
		claims.IssuedAt > 0 &&
		claims.ExpiresAt > claims.IssuedAt &&
		claims.ExpiresAt > now.Unix()
}

func minTime(a time.Time, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func HashRefreshToken(refreshToken string) string {
	sum := sha256.Sum256([]byte(refreshToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type accessTokenClaims struct {
	TokenType string `json:"token_type"`
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	SessionID string `json:"sid"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	JWTID     string `json:"jti"`
}

func signAccessToken(secret []byte, claims accessTokenClaims) (string, error) {
	headerSegment, err := encodeJWTSegment(jwtHeader{
		Algorithm: jwtAlgorithmHS256,
		Type:      jwtType,
	})
	if err != nil {
		return "", err
	}

	claimsSegment, err := encodeJWTSegment(claims)
	if err != nil {
		return "", err
	}

	signingInput := headerSegment + "." + claimsSegment
	signature := signJWTInput(secret, signingInput)

	return signingInput + "." + signature, nil
}

func signJWTInput(secret []byte, signingInput string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encodeJWTSegment(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode jwt segment: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeJWTSegment(segment string, target any) error {
	payload, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func randomTokenString(random io.Reader, size int) (string, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
