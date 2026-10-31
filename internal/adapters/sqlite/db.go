// Package sqlite es un ADAPTADOR SECUNDARIO: implementa los puertos de salida
// (repositorios, TxManager) usando SQLite.
//
// Usamos modernc.org/sqlite, una traducción de SQLite a Go puro: no requiere
// CGO ni gcc, compila en cualquier plataforma y genera binarios estáticos.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registra el driver "sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// timeLayout es el formato con el que guardamos fechas: ordenable como texto.
const timeLayout = "2006-01-02T15:04:05.000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t, nil
}

// Open abre (o crea) la base de datos con PRAGMAs recomendados:
//   - foreign_keys(1): SQLite NO valida FKs por defecto.
//   - journal_mode(WAL): lectores y escritor concurrentes.
//   - busy_timeout(5000): espera en vez de fallar si la BD está ocupada.
//   - _txlock=immediate: las transacciones toman el lock de escritura al
//     iniciar, evitando condiciones de carrera "leer-luego-escribir".
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite admite un único escritor; limitar conexiones evita SQLITE_BUSY.
	db.SetMaxOpenConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// Migrate aplica en orden las migraciones embebidas que aún no se ejecutaron.
// Cada migración corre en su propia transacción.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		name := e.Name()
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %q must start with a numeric version: %w", name, err)
		}
		var exists int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if exists > 0 {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", name, err)
		}
		if err := applyMigration(ctx, db, version, name, string(body)); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, version int, name, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("apply migration %q: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		version, name, formatTime(time.Now())); err != nil {
		return fmt.Errorf("record migration %q: %w", name, err)
	}
	return tx.Commit()
}
