// Package dbtest gives integration tests an isolated PostgreSQL schema.
// Tests are skipped unless TEST_DATABASE_URL is set.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool returns a pool whose search_path points at a fresh schema that is
// dropped when the test ends.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(suffix)

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// Extensions are per database. Keep pgcrypto in public, so dropping a test
	// schema never drops it while a test in another package is migrating.
	if _, err := admin.Exec(ctx, `
		DO $$ BEGIN
			PERFORM pg_advisory_xact_lock(72201002);
			CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
		END $$`); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	return pool
}
