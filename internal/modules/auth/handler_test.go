package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeAuthHTTPService struct {
	registerRequest RegisterRequest
	registerResult  RegisterResult
	registerErr     error
	loginRequest    LoginRequest
	loginResult     LoginResult
	loginErr        error
	refreshRequest  RefreshTokenRequest
	refreshResult   RefreshTokenResult
	refreshErr      error
	logoutRequest   LogoutRequest
	logoutErr       error
}

func (f *fakeAuthHTTPService) Register(ctx context.Context, request RegisterRequest) (RegisterResult, error) {
	f.registerRequest = request
	if f.registerErr != nil {
		return RegisterResult{}, f.registerErr
	}
	return f.registerResult, nil
}

func (f *fakeAuthHTTPService) Login(ctx context.Context, request LoginRequest) (LoginResult, error) {
	f.loginRequest = request
	if f.loginErr != nil {
		return LoginResult{}, f.loginErr
	}
	return f.loginResult, nil
}

func (f *fakeAuthHTTPService) RefreshToken(ctx context.Context, request RefreshTokenRequest) (RefreshTokenResult, error) {
	f.refreshRequest = request
	if f.refreshErr != nil {
		return RefreshTokenResult{}, f.refreshErr
	}
	return f.refreshResult, nil
}

func (f *fakeAuthHTTPService) Logout(ctx context.Context, request LogoutRequest) error {
	f.logoutRequest = request
	return f.logoutErr
}

func TestHandlerRegister(t *testing.T) {
	t.Parallel()

	service := &fakeAuthHTTPService{
		registerResult: RegisterResult{
			UserID: "user_123",
			Email:  "user@example.com",
		},
	}
	handler := NewHandler(service)
	req := newAuthHTTPRequest(t, http.MethodPost, "/v1/auth/register", `{
		"email": "user@example.com",
		"password": "secret123",
		"display_name": "Test User"
	}`)
	rec := httptest.NewRecorder()

	handler.HandleRegister(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.registerRequest.Email != "user@example.com" {
		t.Fatalf("expected register email, got %s", service.registerRequest.Email)
	}
	assertJSONField(t, rec.Body.String(), "data.user_id", "user_123")
	assertJSONField(t, rec.Body.String(), "data.email", "user@example.com")
}

func TestHandlerLogin(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 7, 15, 9, 30, 0, 0, time.UTC)
	service := &fakeAuthHTTPService{
		loginResult: LoginResult{
			UserID: "user_123",
			Email:  "user@example.com",
			Tokens: TokenPair{
				AccessToken:           "access-token",
				AccessTokenExpiresAt:  expiresAt,
				RefreshToken:          "refresh-token",
				RefreshTokenExpiresAt: expiresAt.Add(30 * 24 * time.Hour),
			},
		},
	}
	handler := NewHandler(service)
	req := newAuthHTTPRequest(t, http.MethodPost, "/v1/auth/login", `{
		"email": "user@example.com",
		"password": "secret123"
	}`)
	req.Header.Set("User-Agent", "CoreBE test browser")
	req.RemoteAddr = "203.0.113.10:54321"
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.loginRequest.UserAgent != "CoreBE test browser" {
		t.Fatalf("expected user agent, got %s", service.loginRequest.UserAgent)
	}
	if service.loginRequest.IPAddress != "203.0.113.10" {
		t.Fatalf("expected client ip, got %s", service.loginRequest.IPAddress)
	}
	assertJSONField(t, rec.Body.String(), "data.tokens.access_token", "access-token")
	assertJSONField(t, rec.Body.String(), "data.tokens.refresh_token", "refresh-token")
}

func TestHandlerRefreshToken(t *testing.T) {
	t.Parallel()

	service := &fakeAuthHTTPService{
		refreshResult: RefreshTokenResult{
			UserID: "user_123",
			Email:  "user@example.com",
			Tokens: TokenPair{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
			},
		},
	}
	handler := NewHandler(service)
	req := newAuthHTTPRequest(t, http.MethodPost, "/v1/auth/refresh", `{
		"refresh_token": "old-refresh-token"
	}`)
	rec := httptest.NewRecorder()

	handler.HandleRefreshToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.refreshRequest.RefreshToken != "old-refresh-token" {
		t.Fatalf("expected refresh token request, got %s", service.refreshRequest.RefreshToken)
	}
	assertJSONField(t, rec.Body.String(), "data.tokens.access_token", "new-access-token")
	assertJSONField(t, rec.Body.String(), "data.tokens.refresh_token", "new-refresh-token")
}

func TestHandlerLogout(t *testing.T) {
	t.Parallel()

	service := &fakeAuthHTTPService{}
	handler := NewHandler(service)
	req := newAuthHTTPRequest(t, http.MethodPost, "/v1/auth/logout", `{
		"refresh_token": "refresh-token"
	}`)
	rec := httptest.NewRecorder()

	handler.HandleLogout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %s", rec.Body.String())
	}
	if service.logoutRequest.RefreshToken != "refresh-token" {
		t.Fatalf("expected logout refresh token, got %s", service.logoutRequest.RefreshToken)
	}
}

func TestHandlerMe(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(&fakeAuthHTTPService{})
	req := newAuthHTTPRequest(t, http.MethodGet, "/v1/auth/me", `{}`)
	req = req.WithContext(ContextWithAuthenticatedUser(req.Context(), AuthenticatedUser{
		UserID:        "user_123",
		SessionID:     "session_123",
		AccessTokenID: "access_token_123",
		ExpiresAt:     expiresAt,
	}))
	rec := httptest.NewRecorder()

	handler.HandleMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	assertJSONField(t, rec.Body.String(), "data.user_id", "user_123")
	assertJSONField(t, rec.Body.String(), "data.session_id", "session_123")
	assertJSONField(t, rec.Body.String(), "data.access_token_id", "access_token_123")
}

func TestHandlerMapsAuthErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid credentials", err: ErrInvalidCredentials, wantStatus: http.StatusUnauthorized, wantCode: "INVALID_CREDENTIALS"},
		{name: "email exists", err: ErrEmailAlreadyExists, wantStatus: http.StatusConflict, wantCode: "EMAIL_ALREADY_EXISTS"},
		{name: "disabled user", err: ErrUserDisabled, wantStatus: http.StatusForbidden, wantCode: "USER_DISABLED"},
		{name: "unknown", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeAuthHTTPService{loginErr: tt.err}
			handler := NewHandler(service)
			req := newAuthHTTPRequest(t, http.MethodPost, "/v1/auth/login", `{
				"email": "user@example.com",
				"password": "secret123"
			}`)
			rec := httptest.NewRecorder()

			handler.HandleLogin(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			assertJSONField(t, rec.Body.String(), "error.code", tt.wantCode)
		})
	}
}

func TestHandlerRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	service := &fakeAuthHTTPService{}
	handler := NewHandler(service)
	req := newAuthHTTPRequest(t, http.MethodPost, "/v1/auth/login", `{
		"email": "user@example.com",
		"password": "secret123",
		"unexpected": true
	}`)
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	assertJSONField(t, rec.Body.String(), "error.code", "INVALID_JSON")
	if service.loginRequest.Email != "" {
		t.Fatalf("expected service not to be called, got %+v", service.loginRequest)
	}
}

func TestHandlerRejectsMissingJSONContentType(t *testing.T) {
	t.Parallel()

	service := &fakeAuthHTTPService{}
	handler := NewHandler(service)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"secret123"}`))
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected status 415, got %d: %s", rec.Code, rec.Body.String())
	}
	assertJSONField(t, rec.Body.String(), "error.code", "UNSUPPORTED_MEDIA_TYPE")
}

func newAuthHTTPRequest(t *testing.T, method string, target string, body string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertJSONField(t *testing.T, body string, path string, want string) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}

	parts := strings.Split(path, ".")
	var current any = payload
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected object at %s in %s", part, body)
		}
		current = object[part]
	}

	got, ok := current.(string)
	if !ok {
		t.Fatalf("expected string at %s, got %T", path, current)
	}
	if got != want {
		t.Fatalf("expected %s to be %s, got %s", path, want, got)
	}
}
