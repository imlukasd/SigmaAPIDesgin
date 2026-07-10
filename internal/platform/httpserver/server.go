package httpserver

import (
	"log/slog"
	"net/http"

	"corebe.local/api/internal/config"
	"corebe.local/api/internal/modules/health"
	"corebe.local/api/internal/modules/payment"
	"corebe.local/api/internal/platform/middleware"
	"corebe.local/api/internal/platform/response"
)

func New(cfg config.Config, logger *slog.Logger, db health.Database) *http.Server {
	mux := http.NewServeMux()
	healthHandler := health.NewHandler(db)

	mux.HandleFunc("GET /healthz", healthHandler.HandleHealthz)
	mux.HandleFunc("GET /readyz", healthHandler.HandleReadyz)
	mux.HandleFunc("GET /v1/payments/capabilities", payment.HandleCapabilities)
	mux.HandleFunc("GET /v1", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"name":    "CoreBE API",
				"version": "v1",
				"env":     cfg.AppEnv,
			},
		})
	})

	handler := middleware.Chain(
		mux,
		middleware.Recover(logger),
		middleware.RequestID,
		middleware.AccessLog(logger),
		middleware.SecurityHeaders,
	)

	return &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}
