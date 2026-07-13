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
	accessTokenID, err := randomTokenString(m.random, accessTokenIDBytes)
	if err != nil {
		return IssuedTokenPair{}, err
	}

	accessTokenExpiresAt := now.Add(m.accessTokenTTL)
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
			RefreshTokenExpiresAt: now.Add(m.refreshTokenTTL),
		},
		AccessTokenID:    accessTokenID,
		RefreshTokenHash: HashRefreshToken(refreshToken),
	}, nil
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
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

func encodeJWTSegment(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode jwt segment: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func randomTokenString(random io.Reader, size int) (string, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
