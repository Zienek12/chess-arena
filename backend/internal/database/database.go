// Package database opens the PostgreSQL pool and applies versioned migrations.
package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLockID serializes migrations when several backend instances start at once.
const migrationLockID = 72_201_001

// Open connects to PostgreSQL and checks the connection.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

type migration struct {
	version int
	name    string
	sql     string
}

// Migrate applies every migration that has not been applied yet, each in its
// own transaction, and records it in schema_migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if err := adoptLegacySchema(ctx, conn.Conn(), migrations[0]); err != nil {
		return err
	}

	applied := make(map[int]bool)
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return err
		}
		applied[version] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, migration := range migrations {
		if applied[migration.version] {
			continue
		}
		if err := apply(ctx, conn.Conn(), migration); err != nil {
			return err
		}
	}
	return nil
}

// adoptLegacySchema marks the first migration as applied on databases created
// before schema_migrations existed, when the backend ran 001 on every start.
func adoptLegacySchema(ctx context.Context, conn *pgx.Conn, first migration) error {
	_, err := conn.Exec(ctx, `
		INSERT INTO schema_migrations (version, name)
		SELECT $1, $2
		WHERE to_regclass('engines') IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM schema_migrations)
	`, first.version, first.name)
	if err != nil {
		return fmt.Errorf("adopt legacy schema: %w", err)
	}
	return nil
}

func apply(ctx context.Context, conn *pgx.Conn, migration migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.version, err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, migration.sql); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.name, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
		migration.version, migration.name); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.name, err)
	}
	return tx.Commit(ctx)
}

// loadMigrations reads migrations/NNN_name.sql files ordered by version.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		prefix, _, found := strings.Cut(entry.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if !found || err != nil {
			return nil, fmt.Errorf("migration %s must be named NNN_name.sql", entry.Name())
		}
		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, migration{version: version, name: entry.Name(), sql: string(content)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	for index := 1; index < len(migrations); index++ {
		if migrations[index].version == migrations[index-1].version {
			return nil, fmt.Errorf("duplicate migration version %d", migrations[index].version)
		}
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no migrations found")
	}
	return migrations, nil
}
