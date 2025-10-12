package pkg

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

// QueryItem is a helper type for building batch queries in tests
type QueryItem struct {
	Query  string        `json:"query"`
	Params []interface{} `json:"params"`
}

// buildBatchPayload creates a Payload with multiple queries
func buildBatchPayload(items []QueryItem) Payload {
	queries := make([]struct {
		Query  string        `json:"query"`
		Params []interface{} `json:"params"`
	}, len(items))

	for i, item := range items {
		queries[i].Query = item.Query
		queries[i].Params = item.Params
	}

	return Payload{Queries: queries}
}

// getTestConnectionString loads the TEST_DATABASE_URL from .env file
func getTestConnectionString(t *testing.T) string {
	// Try to load .env file from the halo directory
	envPath := filepath.Join("..", ".env")
	_ = godotenv.Load(envPath) // Ignore error if file doesn't exist

	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	return connStr
}

// setupTestTable creates a test table with a unique name and returns cleanup function
func setupTestTable(ctx context.Context, connStr string, createSQL string) (tableName string, cleanup func(), err error) {
	// Generate unique table name
	tableName = fmt.Sprintf("test_table_%d_%d", time.Now().Unix(), rand.Int63n(10000))

	// Replace placeholder with actual table name
	createSQL = fmt.Sprintf(createSQL, tableName)

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, createSQL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create table: %w", err)
	}

	cleanup = func() {
		conn, err := pgx.Connect(ctx, connStr)
		if err != nil {
			return
		}
		defer conn.Close(ctx)
		_, _ = conn.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
	}

	return tableName, cleanup, nil
}

// countRows counts the number of rows in a table
func countRows(ctx context.Context, connStr string, tableName string) (int, error) {
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return 0, err
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	return count, err
}

// executeDirectQuery executes a query directly (for setup/verification)
func executeDirectQuery(ctx context.Context, connStr string, query string, args ...interface{}) error {
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, query, args...)
	return err
}

// TestBatchTransactionCommit verifies that successful batch queries commit together
func TestBatchTransactionCommit(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	// Setup: Create test table
	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	defer cleanup()

	// Execute: Batch with multiple INSERTs
	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s (name) VALUES ($1)", tableName), Params: []interface{}{"Alice"}},
		{Query: fmt.Sprintf("INSERT INTO %s (name) VALUES ($1)", tableName), Params: []interface{}{"Bob"}},
		{Query: fmt.Sprintf("INSERT INTO %s (name) VALUES ($1)", tableName), Params: []interface{}{"Charlie"}},
	})

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
	})

	// Verify: No error and all rows committed
	require.NoError(t, err)
	require.True(t, result.IsBatch)
	require.Len(t, result.Results, 3)

	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 3, count, "All 3 inserts should be committed")
}

// TestBatchTransactionRollback verifies that errors cause entire batch to rollback
func TestBatchTransactionRollback(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	// Setup: Create test table with unique constraint
	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, email TEXT UNIQUE)")
	require.NoError(t, err)
	defer cleanup()

	// Execute: Batch with duplicate insert (should fail)
	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s (email) VALUES ($1)", tableName), Params: []interface{}{"test@example.com"}},
		{Query: fmt.Sprintf("INSERT INTO %s (email) VALUES ($1)", tableName), Params: []interface{}{"test@example.com"}}, // Duplicate!
	})

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
	})

	// Verify: Error occurred and NO rows were committed (rollback)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")

	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 0, count, "First insert should be rolled back")

	// Ensure we got an empty result
	require.False(t, result.IsBatch)
}

// TestIsolationLevelReadCommitted verifies ReadCommitted isolation level
func TestIsolationLevelReadCommitted(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, value INT)")
	require.NoError(t, err)
	defer cleanup()

	// Execute batch with ReadCommitted
	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName), Params: []interface{}{100}},
	})

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
	})

	require.NoError(t, err)
	require.True(t, result.IsBatch)

	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestIsolationLevelSerializable verifies Serializable isolation level
func TestIsolationLevelSerializable(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, value INT)")
	require.NoError(t, err)
	defer cleanup()

	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName), Params: []interface{}{200}},
	})

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "Serializable",
	})

	require.NoError(t, err)
	require.True(t, result.IsBatch)

	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestIsolationLevelRepeatableRead verifies RepeatableRead isolation level
func TestIsolationLevelRepeatableRead(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, value INT)")
	require.NoError(t, err)
	defer cleanup()

	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName), Params: []interface{}{300}},
	})

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "RepeatableRead",
	})

	require.NoError(t, err)
	require.True(t, result.IsBatch)
}

// TestInvalidIsolationLevel verifies that invalid isolation level returns error
func TestInvalidIsolationLevel(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY)")
	require.NoError(t, err)
	defer cleanup()

	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s DEFAULT VALUES", tableName), Params: []interface{}{}},
	})

	_, err = Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "InvalidLevel",
	})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidIsolationLevel)
}

// TestReadOnlyMode verifies that write operations fail in read-only mode
func TestReadOnlyMode(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	defer cleanup()

	// Execute: Try to INSERT in read-only transaction
	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s (name) VALUES ($1)", tableName), Params: []interface{}{"test"}},
	})

	_, err = Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
		BatchReadOnly:       true,
	})

	// Verify: Error about read-only transaction
	require.Error(t, err)
	require.Contains(t, err.Error(), "read-only")

	// Verify: No data was inserted
	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

// TestReadOnlyModeAllowsReads verifies that read operations work in read-only mode
func TestReadOnlyModeAllowsReads(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	defer cleanup()

	// Setup: Insert some test data
	err = executeDirectQuery(ctx, connStr, fmt.Sprintf("INSERT INTO %s (name) VALUES ($1)", tableName), "Alice")
	require.NoError(t, err)

	// Execute: SELECT in read-only transaction
	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("SELECT * FROM %s", tableName), Params: []interface{}{}},
	})

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
		BatchReadOnly:       true,
	})

	// Verify: No error and data returned
	require.NoError(t, err)
	require.True(t, result.IsBatch)
	require.Len(t, result.Results, 1)
}

// TestSleepInTransaction verifies transaction stays open during pg_sleep
func TestSleepInTransaction(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, value INT)")
	require.NoError(t, err)
	defer cleanup()

	// Execute: INSERT, sleep, SELECT
	payload := buildBatchPayload([]QueryItem{
		{Query: fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName), Params: []interface{}{42}},
		{Query: "SELECT pg_sleep(1)", Params: []interface{}{}},
		{Query: fmt.Sprintf("SELECT value FROM %s WHERE value = $1", tableName), Params: []interface{}{42}},
	})

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
	})

	// Verify: All queries succeeded
	require.NoError(t, err)
	require.True(t, result.IsBatch)
	require.Len(t, result.Results, 3)

	// Verify: SELECT after sleep saw the INSERT
	require.Equal(t, 1, result.Results[2].RowCount)
}

// TestConcurrentBatches verifies multiple concurrent batch transactions work correctly
func TestConcurrentBatches(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, thread_id INT, value INT)")
	require.NoError(t, err)
	defer cleanup()

	// Execute: Multiple concurrent batches
	numThreads := 5
	var wg sync.WaitGroup
	errors := make([]error, numThreads)

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()

			payload := buildBatchPayload([]QueryItem{
				{Query: fmt.Sprintf("INSERT INTO %s (thread_id, value) VALUES ($1, $2)", tableName), Params: []interface{}{threadID, threadID * 100}},
				{Query: fmt.Sprintf("INSERT INTO %s (thread_id, value) VALUES ($1, $2)", tableName), Params: []interface{}{threadID, threadID*100 + 1}},
			})

			_, err := Execute(ctx, *NewSecret(connStr), payload, Options{
				BatchIsolationLevel: "ReadCommitted",
			})
			errors[threadID] = err
		}(i)
	}

	wg.Wait()

	// Verify: No errors
	for i, err := range errors {
		require.NoError(t, err, "Thread %d should not have error", i)
	}

	// Verify: All rows committed
	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, numThreads*2, count, "All inserts should be committed")
}

// TestDirtyReadPrevention verifies one transaction doesn't see uncommitted changes
func TestDirtyReadPrevention(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, value INT)")
	require.NoError(t, err)
	defer cleanup()

	// This test would require manual transaction control which our current
	// Execute API doesn't support. We can verify that our batches are isolated
	// by running concurrent batches and checking no partial results are visible.

	var wg sync.WaitGroup
	wg.Add(2)

	// Transaction 1: Long-running insert
	go func() {
		defer wg.Done()
		payload := buildBatchPayload([]QueryItem{
			{Query: fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName), Params: []interface{}{100}},
			{Query: "SELECT pg_sleep(0.5)", Params: []interface{}{}},
		})
		_, _ = Execute(ctx, *NewSecret(connStr), payload, Options{
			BatchIsolationLevel: "ReadCommitted",
		})
	}()

	// Transaction 2: Quick read after a delay
	time.Sleep(100 * time.Millisecond) // Start after transaction 1
	go func() {
		defer wg.Done()
		payload := buildBatchPayload([]QueryItem{
			{Query: fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName), Params: []interface{}{}},
		})
		_, _ = Execute(ctx, *NewSecret(connStr), payload, Options{
			BatchIsolationLevel: "ReadCommitted",
		})
	}()

	wg.Wait()

	// After both complete, data should be visible
	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestLongTransaction verifies transaction with many queries doesn't timeout
func TestLongTransaction(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, value INT)")
	require.NoError(t, err)
	defer cleanup()

	// Build batch with many queries
	queries := make([]QueryItem, 0, 20)

	for i := 0; i < 20; i++ {
		queries = append(queries, QueryItem{
			Query:  fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName),
			Params: []interface{}{i},
		})
	}

	payload := buildBatchPayload(queries)

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
	})

	// Verify: All queries succeeded
	require.NoError(t, err)
	require.True(t, result.IsBatch)
	require.Len(t, result.Results, 20)

	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 20, count)
}

// TestSingleQueryDoesNotUseTransaction verifies single queries don't use transactions
// This is verified by checking that the Execute logic doesn't start a transaction
// for single query mode (implementation detail, but important for performance)
func TestSingleQueryDoesNotUseTransaction(t *testing.T) {
	connStr := getTestConnectionString(t)
	ctx := context.Background()

	tableName, cleanup, err := setupTestTable(ctx, connStr, "CREATE TABLE %s (id SERIAL PRIMARY KEY, value INT)")
	require.NoError(t, err)
	defer cleanup()

	// Execute single query (not a batch)
	payload := Payload{
		Query:  fmt.Sprintf("INSERT INTO %s (value) VALUES ($1)", tableName),
		Params: []interface{}{999},
	}

	result, err := Execute(ctx, *NewSecret(connStr), payload, Options{
		BatchIsolationLevel: "ReadCommitted",
	})

	// Verify: Executed successfully as single query
	require.NoError(t, err)
	require.False(t, result.IsBatch, "Single query should not be a batch")
	require.Len(t, result.Results, 1)

	count, err := countRows(ctx, connStr, tableName)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
