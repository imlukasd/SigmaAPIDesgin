package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeTokenVerifier struct {
	accessToken string
	user        AuthenticatedUser
	err         error
}

func (f *fakeTokenVerifier) VerifyAccessToken(ctx context.Context, accessToken string) (AuthenticatedUser, error) {
	f.accessToken = accessToken
	if f.err != nil {
		return AuthenticatedUser{}, f.err
	}
	return f.user, nil
}

func TestRequireAuth(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	verifier := &fakeTokenVerifier{
		user: AuthenticatedUser{
			UserID:        "user_123",
			SessionID:     "session_123",
			AccessTokenID: "access_token_123",
			ExpiresAt:     expiresAt,
		},
	}
	var gotUser AuthenticatedUser
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		var ok bool
		gotUser, ok = AuthenticatedUserFromContext(r.Context())
		if !ok {
			t.Fatal("expected authenticated user in context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequireAuth(verifier)(next)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer access-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if verifier.accessToken != "access-token" {
		t.Fatalf("expected access token, got %s", verifier.accessToken)
	}
	if gotUser.UserID != "user_123" || gotUser.SessionID != "session_123" || !gotUser.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected authenticated user: %+v", gotUser)
	}
}

func TestRequireAuthRejectsMissingOrMalformedBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
	}{
		{name: "missing"},
		{name: "wrong scheme", header: "Basic abc"},
		{name: "empty bearer", header: "Bearer"},
		{name: "too many fields", header: "Bearer one two"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			verifier := &fakeTokenVerifier{}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler must not be called")
			})
			handler := RequireAuth(verifier)(next)
			req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("expected WWW-Authenticate Bearer, got %s", rec.Header().Get("WWW-Authenticate"))
			}
			if verifier.accessToken != "" {
				t.Fatalf("expected verifier not to be called, got %s", verifier.accessToken)
			}
		})
	}
}

func TestRequireAuthRejectsInvalidAccessToken(t *testing.T) {
	t.Parallel()

	verifier := &fakeTokenVerifier{err: ErrInvalidAccessToken}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	handler := RequireAuth(verifier)(next)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if verifier.accessToken != "invalid-token" {
		t.Fatalf("expected verifier token invalid-token, got %s", verifier.accessToken)
	}
}

func TestRequireAuthReturnsInternalErrorForVerifierFailure(t *testing.T) {
	t.Parallel()

	verifier := &fakeTokenVerifier{err: errors.New("verifier failed")}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	})
	handler := RequireAuth(verifier)(next)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer access-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
