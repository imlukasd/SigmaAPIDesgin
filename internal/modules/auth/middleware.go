package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"corebe.local/api/internal/platform/middleware"
	"corebe.local/api/internal/platform/response"
)

type authenticatedUserContextKey struct{}

func RequireAuth(verifier TokenVerifier) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessToken, ok := bearerTokenFromRequest(r)
			if !ok {
				writeAccessTokenError(w)
				return
			}

			user, err := verifier.VerifyAccessToken(r.Context(), accessToken)
			if err != nil {
				if errors.Is(err, ErrInvalidAccessToken) {
					writeAccessTokenError(w)
					return
				}
				response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
				return
			}

			next.ServeHTTP(w, r.WithContext(ContextWithAuthenticatedUser(r.Context(), user)))
		})
	}
}

func ContextWithAuthenticatedUser(ctx context.Context, user AuthenticatedUser) context.Context {
	return context.WithValue(ctx, authenticatedUserContextKey{}, user)
}

func AuthenticatedUserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(authenticatedUserContextKey{}).(AuthenticatedUser)
	return user, ok
}

func bearerTokenFromRequest(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}

	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}

	return parts[1], true
}

func writeAccessTokenError(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	response.Error(w, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "Invalid or missing access token.", nil)
}
