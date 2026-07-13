package testdb

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"corebe.local/api/internal/config"
	"corebe.local/api/internal/platform/database"
	"corebe.local/api/internal/platform/migration"
)

const TestDatabaseURLEnv = "TEST_DATABASE_URL"

func Open(t testing.TB) *pgxpool.Pool {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv(TestDatabaseURLEnv))
	if databaseURL == "" {
		t.Skipf("set %s to run database integration tests", TestDatabaseURLEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, config.DatabaseConfig{
		URL:             databaseURL,
		MaxConns:        5,
		MinConns:        0,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: time.Minute,
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	root := repoRoot(t)
	if _, err := migration.Up(ctx, pool, filepath.Join(root, "migrations")); err != nil {
		t.Fatalf("apply test migrations: %v", err)
	}

	return pool
}

func Exec(t testing.TB, db database.DBTX, sql string, args ...any) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec test query: %v", err)
	}
}

func NewUUID(t testing.TB) string {
	t.Helper()

	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("generate uuid: %v", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func UniqueSuffix(t testing.TB) string {
	t.Helper()

	id := NewUUID(t)
	return strings.ReplaceAll(id, "-", "")
}

func repoRoot(t testing.TB) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repository root not found from %s", dir)
		}
		dir = parent
	}
}
