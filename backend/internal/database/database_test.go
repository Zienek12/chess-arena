package database

import (
	"context"
	"testing"

	"chess-arena/arena/internal/database/dbtest"
)

func TestMigrateIsIdempotent(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	for run := 1; run <= 2; run++ {
		if err := Migrate(ctx, pool); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	migrations, _ := loadMigrations()
	if count != len(migrations) {
		t.Fatalf("recorded %d migrations, want %d", count, len(migrations))
	}
}

func TestMigrateAdoptsLegacySchema(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	// A database created by the old backend: 001 applied, no schema_migrations.
	if _, err := pool.Exec(ctx, migrations[0].sql); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var movetime bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'tournaments' AND column_name = 'movetime_ms')
	`).Scan(&movetime); err != nil {
		t.Fatal(err)
	}
	if !movetime {
		t.Fatal("migration 002 was not applied on top of the legacy schema")
	}
}
