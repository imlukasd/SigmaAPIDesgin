package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"corebe.local/api/internal/config"
	"corebe.local/api/internal/platform/database"
	"corebe.local/api/internal/platform/migration"
)

const migrationsDir = "migrations"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := config.Load()
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		logger.Error("database pool failed to open", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	switch os.Args[1] {
	case "up":
		applied, err := migration.Up(ctx, db, migrationsDir)
		if err != nil {
			logger.Error("migrations failed", "error", err)
			os.Exit(1)
		}
		if len(applied) == 0 {
			logger.Info("no pending migrations")
			return
		}
		for _, item := range applied {
			logger.Info("migration applied", "version", item.Version, "name", item.Name)
		}
	case "status":
		statuses, err := migration.Statuses(ctx, db, migrationsDir)
		if err != nil {
			logger.Error("migration status failed", "error", err)
			os.Exit(1)
		}
		if len(statuses) == 0 {
			fmt.Println("No migration files found.")
			return
		}
		for _, status := range statuses {
			state := "pending"
			if status.Applied {
				state = "applied"
			}
			fmt.Printf("%06d %-32s %s\n", status.Migration.Version, status.Migration.Name, state)
		}
	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println("Usage: go run ./cmd/migrate <up|status>")
}
