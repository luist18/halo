package httpexecutor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/luist18/halo/internal/data"
)

var (
	ErrInvalidQueryMode        = errors.New("invalid query mode")
	ErrPoolOptInNotImplemented = errors.New("connection pooling is not yet implemented")
	ErrInvalidPayload          = errors.New("invalid payload: both query and queries provided")
	ErrEmptyPayload            = errors.New("invalid payload: neither query nor queries provided")
	ErrInvalidIsolationLevel   = errors.New("invalid transaction isolation level")
)

type Options struct {
	PoolOptIn           bool
	BatchIsolationLevel string
	BatchReadOnly       bool
	BatchDeferrable     bool
}

type MultiQueryPayload []struct {
	Query  string        `json:"query"`
	Params []interface{} `json:"params"`
}

type Payload struct {
	Query   string            `json:"query"`
	Params  []interface{}     `json:"params"`
	Queries MultiQueryPayload `json:"queries,omitempty"`
}

type ExecutorResponse struct {
	Fields     any    `json:"fields"`
	Rows       []any  `json:"rows"`
	Command    string `json:"command"`
	RowCount   int    `json:"rowCount"`
	RowAsArray bool   `json:"rowAsArray"`
}

// Result represents the result of executing a query or batch of queries
type Result interface {
	ToResponse() interface{}
	GetHeaders() map[string]string
}

// SingleQueryResult represents the result of a single query execution
type SingleQueryResult struct {
	Response ExecutorResponse
}

func (s *SingleQueryResult) ToResponse() interface{} {
	return s.Response
}

func (s *SingleQueryResult) GetHeaders() map[string]string {
	return map[string]string{}
}

// BatchQueryResult represents the result of a batch query execution
type BatchQueryResult struct {
	Responses      []ExecutorResponse
	IsolationLevel string
	ReadOnly       bool
	Deferrable     bool
}

func (b *BatchQueryResult) ToResponse() interface{} {
	return map[string]interface{}{
		"results": b.Responses,
	}
}

func (b *BatchQueryResult) GetHeaders() map[string]string {
	headers := make(map[string]string)
	if b.ReadOnly {
		headers["Neon-Batch-Read-Only"] = "true"
	}
	if b.Deferrable {
		headers["Neon-Batch-Deferrable"] = "true"
	}
	if b.IsolationLevel != "" {
		headers["Neon-Batch-Isolation-Level"] = b.IsolationLevel
	}
	return headers
}

func Execute(ctx context.Context, connStrSecret data.Secret, payload Payload, opts Options) (Result, error) {
	// TODO(PER-14): implement connection pooling to reuse database connections
	// Pool opt-in is currently not supported
	if opts.PoolOptIn {
		return nil, ErrPoolOptInNotImplemented
	}

	if err := validatePayload(payload); err != nil {
		return nil, err
	}

	conn, err := createConnection(ctx, connStrSecret)
	if err != nil {
		return nil, err
	}
	defer closeConnection(ctx, conn)

	queryMode := getQueryMode(payload)
	switch queryMode {
	case QueryModeSingle:
		resp, err := executeQuery(ctx, conn, payload.Query, payload.Params, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}
		return &SingleQueryResult{Response: resp}, nil
	case QueryModeBatch:
		results, err := executeBatchQuery(ctx, conn, payload.Queries, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to execute batch query: %w", err)
		}
		return &BatchQueryResult{
			Responses:      results,
			IsolationLevel: opts.BatchIsolationLevel,
			ReadOnly:       opts.BatchReadOnly,
			Deferrable:     opts.BatchDeferrable,
		}, nil
	default:
		return nil, ErrInvalidQueryMode
	}
}

type QueryMode int

const (
	QueryModeInvalid QueryMode = iota
	QueryModeSingle
	QueryModeBatch
)

func getQueryMode(payload Payload) QueryMode {
	if len(payload.Queries) > 0 && payload.Query != "" {
		return QueryModeInvalid
	}
	if len(payload.Queries) > 0 {
		return QueryModeBatch
	}
	return QueryModeSingle
}

// validatePayload checks if the payload is valid and returns a descriptive error if not
func validatePayload(payload Payload) error {
	hasQuery := payload.Query != ""
	hasQueries := len(payload.Queries) > 0

	if hasQuery && hasQueries {
		return ErrInvalidPayload
	}

	if !hasQuery && !hasQueries {
		return ErrEmptyPayload
	}

	return nil
}

// createConnection establishes a new database connection with the given connection string
func createConnection(ctx context.Context, connStrSecret data.Secret) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(connStrSecret.Unwrap())
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return conn, nil
}

// closeConnection safely closes a database connection
func closeConnection(ctx context.Context, conn *pgx.Conn) {
	if conn == nil {
		return
	}

	if err := conn.Close(ctx); err != nil {
		slog.Error("failed to close connection", slog.String("error", err.Error()))
	}
}

func parseIsolationLevel(level string) (pgx.TxIsoLevel, error) {
	switch level {
	case "ReadUncommitted":
		return pgx.ReadUncommitted, nil
	case "ReadCommitted":
		return pgx.ReadCommitted, nil
	case "RepeatableRead":
		return pgx.RepeatableRead, nil
	case "Serializable":
		return pgx.Serializable, nil
	case "": // empty string means default (ReadCommitted)
		return pgx.ReadCommitted, nil
	default:
		return "", ErrInvalidIsolationLevel
	}
}

// buildTxOptions constructs transaction options from the provided Options
func buildTxOptions(opts Options) (pgx.TxOptions, error) {
	isolationLevel, err := parseIsolationLevel(opts.BatchIsolationLevel)
	if err != nil {
		return pgx.TxOptions{}, fmt.Errorf("invalid transaction isolation level: %w", err)
	}

	txOpts := pgx.TxOptions{
		IsoLevel: isolationLevel,
	}

	if opts.BatchReadOnly {
		txOpts.AccessMode = pgx.ReadOnly
	} else {
		txOpts.AccessMode = pgx.ReadWrite
	}

	if opts.BatchDeferrable {
		txOpts.DeferrableMode = pgx.Deferrable
	} else {
		txOpts.DeferrableMode = pgx.NotDeferrable
	}

	return txOpts, nil
}

type queryExecutor interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
}

func executeQuery(ctx context.Context, conn queryExecutor, query string, params []interface{}, opts Options) (ExecutorResponse, error) {
	rows, err := conn.Query(ctx, query, params...)
	if err != nil {
		return ExecutorResponse{}, err
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()

	results, err := processRows(rows, fieldDescriptions)
	if err != nil {
		return ExecutorResponse{}, fmt.Errorf("failed to process rows: %w", err)
	}

	if err := rows.Err(); err != nil {
		return ExecutorResponse{}, err
	}

	// For SELECT queries, rowCount is the number of rows returned
	// For INSERT/UPDATE/DELETE, rowCount is the number of rows affected
	rowCount := len(results)
	if rows.CommandTag().RowsAffected() > 0 {
		rowCount = int(rows.CommandTag().RowsAffected())
	}

	return ExecutorResponse{
		Fields:     fields(fieldDescriptions),
		Rows:       results,
		Command:    parseCommand(rows.CommandTag().String()),
		RowCount:   rowCount,
		RowAsArray: true,
	}, nil
}

func executeBatchQuery(ctx context.Context, conn *pgx.Conn, queries MultiQueryPayload, opts Options) ([]ExecutorResponse, error) {
	txOpts, err := buildTxOptions(opts)
	if err != nil {
		return nil, err
	}

	tx, err := conn.BeginTx(ctx, txOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if tx != nil {
			err := tx.Rollback(ctx)
			if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				slog.Error("failed to rollback transaction", slog.String("error", err.Error()))
			}
		}
	}()

	results := make([]ExecutorResponse, 0, len(queries))

	for i, q := range queries {
		resp, err := executeQuery(ctx, tx, q.Query, q.Params, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query %d in batch: %w", i+1, err)
		}
		results = append(results, resp)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	tx = nil

	return results, nil
}

func processRows(rows pgx.Rows, fieldDescriptions []pgconn.FieldDescription) ([]any, error) {
	results := make([]any, 0)

	for rows.Next() {
		values := processRawValues(rows.RawValues())
		// Array mode is always enabled, that's the default for the serverless driver
		// https://github.com/neondatabase/serverless/blob/2c51902827a043df0646caf6a5ed8d812e7fb9b6/src/httpQuery.ts#L353
		row := buildRow(values, fieldDescriptions, true)
		results = append(results, row)
	}

	return results, nil
}

// processRawValues converts raw byte slices to strings (for raw text output mode)
func processRawValues(rawValues [][]byte) []any {
	values := make([]any, len(rawValues))
	for idx, rawVal := range rawValues {
		if rawVal == nil {
			values[idx] = nil
		} else {
			values[idx] = string(rawVal)
		}
	}
	return values
}

// buildRow constructs a row as either an array or an object (OrderedMap)
func buildRow(values []any, fieldDescriptions []pgconn.FieldDescription, arrayMode bool) any {
	if arrayMode {
		return values
	}

	mappedRow := data.NewOrderedMap()
	for idx, val := range values {
		mappedRow.Set(fieldDescriptions[idx].Name, val)
	}

	return mappedRow
}
