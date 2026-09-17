package database

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestExecutorUsesPoolOutsideTransaction(t *testing.T) {
	t.Parallel()
	pool := &pgxpool.Pool{}
	require.Same(t, pool, Executor(context.Background(), pool))
}
