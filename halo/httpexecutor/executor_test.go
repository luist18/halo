package httpexecutor

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/luist18/halo/internal/data"
	"github.com/stretchr/testify/require"
)

type stubManagedConn struct {
	queryErr error
	closed   bool
}

func (s *stubManagedConn) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return nil, s.queryErr
}

func (s *stubManagedConn) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return nil, nil
}

func (s *stubManagedConn) Close(ctx context.Context) error {
	s.closed = true
	return nil
}

func TestExecute_AcquiresConnectionAndRunsSingleQuery(t *testing.T) {
	originalAcquire := acquireConnectionFn
	originalExecuteQuery := executeQueryFn
	defer func() {
		acquireConnectionFn = originalAcquire
		executeQueryFn = originalExecuteQuery
	}()

	conn := &stubManagedConn{}
	var capturedPoolOptIn bool

	acquireConnectionFn = func(ctx context.Context, secret data.Secret, poolOptIn bool) (managedConn, error) {
		capturedPoolOptIn = poolOptIn
		return conn, nil
	}

	executeQueryFn = func(ctx context.Context, qc queryExecutor, query string, params []interface{}, opts Options) (ExecutorResponse, error) {
		require.Equal(t, "SELECT 1", query)
		return ExecutorResponse{RowCount: 1}, nil
	}

	result, err := Execute(context.Background(), *data.NewSecret("postgres://user:pass@localhost:5432/db"), Payload{
		Query: "SELECT 1",
	}, Options{
		PoolOptIn: true,
	})

	require.NoError(t, err)
	require.IsType(t, &SingleQueryResult{}, result)
	require.True(t, capturedPoolOptIn)
	require.True(t, conn.closed, "connection should have been closed")
}

func TestExecute_DelegatesToBatchExecutor(t *testing.T) {
	originalAcquire := acquireConnectionFn
	originalExecuteBatch := executeBatchQueryFn
	defer func() {
		acquireConnectionFn = originalAcquire
		executeBatchQueryFn = originalExecuteBatch
	}()

	conn := &stubManagedConn{}
	acquireConnectionFn = func(ctx context.Context, secret data.Secret, poolOptIn bool) (managedConn, error) {
		return conn, nil
	}

	var batchCalled bool
	executeBatchQueryFn = func(ctx context.Context, mc managedConn, queries MultiQueryPayload, opts Options) ([]ExecutorResponse, error) {
		batchCalled = true
		require.Len(t, queries, 1)
		return []ExecutorResponse{
			{RowCount: 1},
		}, nil
	}

	result, err := Execute(context.Background(), *data.NewSecret("postgres://user:pass@localhost:5432/db"), Payload{
		Queries: MultiQueryPayload{
			{Query: "SELECT 1"},
		},
	}, Options{})

	require.NoError(t, err)
	require.True(t, batchCalled, "batch executor should be invoked")
	require.IsType(t, &BatchQueryResult{}, result)
	require.True(t, conn.closed, "connection should have been closed")
}
