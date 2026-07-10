package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createSchemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version bigint PRIMARY KEY,
	name text NOT NULL,
	applied_at timestamptz NOT NULL DEFAULT now()
);
`

var migrationFilePattern = regexp.MustCompile(`^(\d{6})_(.+)\.up\.sql$`)

type Migration struct {
	Version int64
	Name    string
	Path    string
}

type Status struct {
	Migration Migration
	Applied   bool
}

func Up(ctx context.Context, db *pgxpool.Pool, dir string) ([]Migration, error) {
	if err := ensureSchemaMigrations(ctx, db); err != nil {
		return nil, err
	}

	files, err := Load(dir)
	if err != nil {
		return nil, err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return nil, err
	}

	var appliedNow []Migration
	for _, file := range files {
		if applied[file.Version] {
			continue
		}

		if err := apply(ctx, db, file); err != nil {
			return appliedNow, err
		}
		appliedNow = append(appliedNow, file)
	}

	return appliedNow, nil
}

func Statuses(ctx context.Context, db *pgxpool.Pool, dir string) ([]Status, error) {
	if err := ensureSchemaMigrations(ctx, db); err != nil {
		return nil, err
	}

	files, err := Load(dir)
	if err != nil {
		return nil, err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return nil, err
	}

	statuses := make([]Status, 0, len(files))
	for _, file := range files {
		statuses = append(statuses, Status{
			Migration: file,
			Applied:   applied[file.Version],
		})
	}

	return statuses, nil
}

func Load(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	seen := map[int64]string{}
	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		match := migrationFilePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}

		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}

		if previous, exists := seen[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %06d: %s and %s", version, previous, entry.Name())
		}
		seen[version] = entry.Name()

		migrations = append(migrations, Migration{
			Version: version,
			Name:    strings.TrimSpace(match[2]),
			Path:    filepath.Join(dir, entry.Name()),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func ensureSchemaMigrations(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, createSchemaMigrationsTable); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func appliedVersions(ctx context.Context, db *pgxpool.Pool) (map[int64]bool, error) {
	rows, err := db.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int64]bool{}
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func apply(ctx context.Context, db *pgxpool.Pool, migration Migration) error {
	sql, err := os.ReadFile(migration.Path)
	if err != nil {
		return fmt.Errorf("read migration %06d_%s: %w", migration.Version, migration.Name, err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("apply migration %06d_%s: %w", migration.Version, migration.Name, err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO schema_migrations (version, name)
		VALUES ($1, $2)
	`, migration.Version, migration.Name); err != nil {
		return fmt.Errorf("record migration %06d_%s: %w", migration.Version, migration.Name, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %06d_%s: %w", migration.Version, migration.Name, err)
	}

	return nil
}
