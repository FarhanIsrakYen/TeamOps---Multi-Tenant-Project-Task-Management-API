package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the narrow query surface repositories need. Both pgxpool.Pool and
// pgx.Tx satisfy it, which keeps transactional repository work explicit.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Transactor gives services a database-agnostic transaction boundary. The
// callback receives a context carrying the transaction; repositories select
// it through Executor and therefore do not need transaction-specific methods.
type Transactor interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}

type PGXTransactor struct{ pool *pgxpool.Pool }
type transactionContextKey struct{}

func NewTransactor(pool *pgxpool.Pool) *PGXTransactor {
	return &PGXTransactor{pool: pool}
}

// Executor returns the current transaction when ctx is transactional, or the
// pool otherwise. Every repository query should go through this function.
func Executor(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(transactionContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

func (t *PGXTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(transactionContextKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txCtx := context.WithValue(ctx, transactionContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func Open(ctx context.Context, url string, maxConns int32, tracers ...pgx.QueryTracer) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	cfg.MaxConns = maxConns
	if len(tracers) > 0 {
		cfg.ConnConfig.Tracer = tracers[0]
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return NewTransactor(pool).WithinTransaction(ctx, func(txCtx context.Context) error {
		return fn(Executor(txCtx, pool).(pgx.Tx))
	})
}
