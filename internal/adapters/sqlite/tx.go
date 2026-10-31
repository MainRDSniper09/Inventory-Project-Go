package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// querier es lo común entre *sql.DB y *sql.Tx. Gracias a esta interfaz los
// repositorios funcionan igual dentro o fuera de una transacción.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

// conn devuelve la transacción activa en ctx o, si no hay, la BD.
func conn(ctx context.Context, db *sql.DB) querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// TxManager implementa ports.TxManager (patrón Unit of Work).
type TxManager struct{ db *sql.DB }

var _ ports.TxManager = (*TxManager)(nil)

// NewTxManager construye el administrador de transacciones.
func NewTxManager(db *sql.DB) *TxManager { return &TxManager{db: db} }

// WithinTx ejecuta fn en una transacción. Si ya existe una en ctx, la
// reutiliza (transacciones anidadas = misma transacción).
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", mapError(err))
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", mapError(err))
	}
	return nil
}

// mapError traduce errores del driver a errores del DOMINIO. Así el núcleo
// nunca depende de códigos específicos de SQLite.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return err
	}
	msg := se.Error()
	switch se.Code() {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		return fmt.Errorf("%w: %s", domain.ErrConflict, msg)
	case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		return fmt.Errorf("%w: referenced resource does not exist", domain.ErrInvalidInput)
	case sqlite3.SQLITE_CONSTRAINT_CHECK:
		if strings.Contains(msg, "chk_inventory_non_negative") {
			return fmt.Errorf("%w: operation would leave negative stock", domain.ErrInsufficientStock)
		}
		return fmt.Errorf("%w: %s", domain.ErrInvalidInput, msg)
	}
	return err
}
