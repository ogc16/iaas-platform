package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migrationFile struct {
	version  int
	filename string
	contents string
}

// migrationFiles returns the embedded migrations sorted by numeric version.
// Filenames must follow the `NNN_description.sql` convention.
func migrationFiles() ([]migrationFile, error) {
	dirEntries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	var files []migrationFile
	for _, de := range dirEntries {
		name := de.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		numStr := name
		if i := strings.IndexByte(name, '_'); i >= 0 {
			numStr = name[:i]
		}
		version, err := strconv.Atoi(numStr)
		if err != nil {
			return nil, fmt.Errorf("migration %q does not follow the NNN_description.sql naming convention", name)
		}
		contents, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, fmt.Errorf("read embedded migration %q: %w", name, err)
		}
		files = append(files, migrationFile{version: version, filename: name, contents: string(contents)})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

// Migrate applies any embedded migrations not yet recorded in the
// schema_migrations table, in numeric order. Each migration runs in its own
// transaction. It refuses to proceed when the database has applied more
// migrations than are embedded (a downgraded binary). It returns the number
// of migrations newly applied.
func Migrate(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	if err := ensureSchemaMigrationsTable(ctx, pool); err != nil {
		return 0, err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return 0, err
	}

	files, err := migrationFiles()
	if err != nil {
		return 0, err
	}
	if len(applied) > len(files) {
		return 0, fmt.Errorf("database has %d migrations applied but only %d are embedded; refusing to run an older binary against a newer schema", len(applied), len(files))
	}

	count := 0
	for _, f := range files {
		if applied[f.version] {
			continue
		}
		if err := applyOne(ctx, pool, f); err != nil {
			return count, fmt.Errorf("migrate %s: %w", f.filename, err)
		}
		count++
	}
	return count, nil
}

func ensureSchemaMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`)
	return err
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		v, err := strconv.Atoi(version)
		if err != nil {
			return nil, fmt.Errorf("invalid version %q in schema_migrations: %w", version, err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, f migrationFile) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, f.contents); err != nil {
		return fmt.Errorf("execute %s: %w", f.filename, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, strconv.Itoa(f.version)); err != nil {
		return fmt.Errorf("record %s: %w", f.filename, err)
	}
	return tx.Commit(ctx)
}
