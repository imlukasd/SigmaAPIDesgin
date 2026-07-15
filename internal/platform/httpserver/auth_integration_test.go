package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"corebe.local/api/internal/config"
	"corebe.local/api/internal/modules/auth"
	"corebe.local/api/internal/platform/testdb"
)

func TestAuthHTTPFlowIntegration(t *testing.T) {
	db := testdb.Open(t)

	email := "http-auth-" + testdb.UniqueSuffix(t) + "@example.com"
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM users WHERE email_normalized = $1`, email)
	})

	server, err := New(testHTTPConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	if err != nil {
		t.Fatalf("new http server: %v", err)
	}

	registerResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":        email,
		"password":     "secret123",
		"display_name": "HTTP Auth User",
	}, "")
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("expected register status 201, got %d: %s", registerResponse.Code, registerResponse.Body.String())
	}

	loginResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": "secret123",
	}, "")
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
	login := decodeAuthIntegrationResponse(t, loginResponse.Body.Bytes())
	if login.Data.UserID == "" {
		t.Fatal("expected login user id")
	}
	if login.Data.Tokens.AccessToken == "" {
		t.Fatal("expected login access token")
	}
	if login.Data.Tokens.RefreshToken == "" {
		t.Fatal("expected login refresh token")
	}

	unauthorizedMeResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/auth/me", nil, "")
	if unauthorizedMeResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated me status 401, got %d: %s", unauthorizedMeResponse.Code, unauthorizedMeResponse.Body.String())
	}

	meResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/auth/me", nil, login.Data.Tokens.AccessToken)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("expected me status 200, got %d: %s", meResponse.Code, meResponse.Body.String())
	}
	me := decodeAuthIntegrationResponse(t, meResponse.Body.Bytes())
	if me.Data.UserID != login.Data.UserID {
		t.Fatalf("expected me user id %s, got %s", login.Data.UserID, me.Data.UserID)
	}

	refreshResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": login.Data.Tokens.RefreshToken,
	}, "")
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("expected refresh status 200, got %d: %s", refreshResponse.Code, refreshResponse.Body.String())
	}
	refreshed := decodeAuthIntegrationResponse(t, refreshResponse.Body.Bytes())
	if refreshed.Data.Tokens.AccessToken == "" {
		t.Fatal("expected refreshed access token")
	}
	if refreshed.Data.Tokens.RefreshToken == "" {
		t.Fatal("expected refreshed refresh token")
	}
	if refreshed.Data.Tokens.RefreshToken == login.Data.Tokens.RefreshToken {
		t.Fatal("expected refresh token rotation")
	}

	reuseResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": login.Data.Tokens.RefreshToken,
	}, "")
	if reuseResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected reused refresh token status 401, got %d: %s", reuseResponse.Code, reuseResponse.Body.String())
	}

	logoutResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/logout", map[string]any{
		"refresh_token": refreshed.Data.Tokens.RefreshToken,
	}, "")
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("expected logout status 204, got %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}

	refreshAfterLogoutResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": refreshed.Data.Tokens.RefreshToken,
	}, "")
	if refreshAfterLogoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected refresh after logout status 401, got %d: %s", refreshAfterLogoutResponse.Code, refreshAfterLogoutResponse.Body.String())
	}
}

func testHTTPConfig() config.Config {
	return config.Config{
		AppEnv:          "test",
		HTTPAddr:        ":0",
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
		IdleTimeout:     time.Second,
		ShutdownTimeout: time.Second,
		Auth: config.AuthConfig{
			PasswordBcryptCost:  bcrypt.MinCost,
			AccessTokenIssuer:   "corebe-api",
			AccessTokenAudience: "corebe-api",
			AccessTokenSecret:   strings.Repeat("s", auth.MinAccessTokenSecretBytes),
			AccessTokenTTL:      15 * time.Minute,
			RefreshTokenTTL:     30 * 24 * time.Hour,
		},
	}
}

func doJSONRequest(t *testing.T, handler http.Handler, method string, target string, body any, accessToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type authIntegrationResponse struct {
	Data struct {
		UserID        string `json:"user_id"`
		SessionID     string `json:"session_id"`
		AccessTokenID string `json:"access_token_id"`
		Email         string `json:"email"`
		Tokens        struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	} `json:"data"`
}

func decodeAuthIntegrationResponse(t *testing.T, payload []byte) authIntegrationResponse {
	t.Helper()

	var response authIntegrationResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode auth response: %v: %s", err, string(payload))
	}
	return response
}
