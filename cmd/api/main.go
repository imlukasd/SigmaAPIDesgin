package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"corebe.local/api/internal/config"
	"corebe.local/api/internal/platform/database"
	"corebe.local/api/internal/platform/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	cfg := config.Load()

	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		logger.Error("database pool failed to open", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	server, err := httpserver.New(cfg, logger, db)
	if err != nil {
		logger.Error("api server failed to initialize", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("api server starting", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("api server shutting down")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
		os.Exit(1)
	}
}
