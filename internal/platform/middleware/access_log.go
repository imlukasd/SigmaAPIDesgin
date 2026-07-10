package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("http request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", RequestIDFromContext(r.Context()),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}
