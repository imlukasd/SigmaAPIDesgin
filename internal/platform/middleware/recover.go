package middleware

import (
	"log/slog"
	"net/http"

	"corebe.local/api/internal/platform/response"
)

func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered", "error", recovered, "request_id", RequestIDFromContext(r.Context()))
					response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
