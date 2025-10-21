package httpexecutor

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/luist18/halo/internal/secret"
)

var (
	ErrInvalidQueryMode        = errors.New("invalid query mode")
	ErrPoolOptInNotImplemented = errors.New("connection pooling is not yet implemented")
)

type Options struct {
	RawTextOutput       bool
	ArrayMode           bool
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
	Fields      any    `json:"fields"`
	Rows        []any  `json:"rows"`
	Command     string `json:"command,omitempty"`
	RowCount    int    `json:"rowCount,omitempty"`
	RowsAsArray bool   `json:"rowsAsArray,omitempty"`
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

func Execute(ctx context.Context, connStrSecret secret.Secret, payload Payload, opts Options) (Result, error) {
	// TODO(PER-14): implement connection pooling to reuse database connections
	// Pool opt-in is currently not supported
	if opts.PoolOptIn {
		return nil, ErrPoolOptInNotImplemented
	}

	config, err := pgx.ParseConfig(connStrSecret.Unwrap())
	if err != nil {
		return nil, err
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := conn.Close(ctx)
		if err != nil {
			slog.Error("failed to close connection", slog.String("error", err.Error()))
		}
	}()

	queryMode := getQueryMode(payload)
	switch queryMode {
	case QueryModeSingle:
		resp, err := executeSingleQuery(ctx, conn, payload.Query, payload.Params, opts)
		if err != nil {
			return nil, err
		}
		return &SingleQueryResult{Response: resp}, nil
	case QueryModeBatch:
		results, err := executeBatchQuery(ctx, conn, payload.Queries, opts)
		if err != nil {
			return nil, err
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

func executeSingleQuery(ctx context.Context, conn *pgx.Conn, query string, params []interface{}, opts Options) (ExecutorResponse, error) {
	rows, err := conn.Query(ctx, query, params...)
	if err != nil {
		return ExecutorResponse{}, err
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	fields := fields(fieldDescriptions)

	// Process rows
	results, err := processRows(rows, fieldDescriptions, opts)
	if err != nil {
		return ExecutorResponse{}, err
	}

	// {"fields":[{"name":"jsonb","dataTypeID":3802,"tableID":0,"columnID":0,"dataTypeSize":-1,"dataTypeModifier":-1,"format":"text"}],"rows":[["map[t:4]"]],"command":"SELECT","rowCount":1,"rowsAsArray":true}

	if err := rows.Err(); err != nil {
		return ExecutorResponse{}, err
	}

	return ExecutorResponse{
		Fields:      fields,
		Rows:        results,
		Command:     parseCommand(rows.CommandTag().String()),
		RowCount:    len(results),
		RowsAsArray: opts.ArrayMode,
	}, nil
}

func executeSingleQueryInTx(ctx context.Context, tx pgx.Tx, query string, params []interface{}, opts Options) (ExecutorResponse, error) {
	rows, err := tx.Query(ctx, query, params...)
	if err != nil {
		return ExecutorResponse{}, err
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	fields := fields(fieldDescriptions)

	results, err := processRows(rows, fieldDescriptions, opts)
	if err != nil {
		return ExecutorResponse{}, err
	}

	if err := rows.Err(); err != nil {
		return ExecutorResponse{}, err
	}

	return ExecutorResponse{
		Fields:      fields,
		Rows:        results,
		Command:     parseCommand(rows.CommandTag().String()),
		RowCount:    len(results),
		RowsAsArray: opts.ArrayMode,
	}, nil
}

func executeBatchQuery(ctx context.Context, conn *pgx.Conn, queries MultiQueryPayload, opts Options) ([]ExecutorResponse, error) {
	isolationLevel, err := parseIsolationLevel(opts.BatchIsolationLevel)
	if err != nil {
		return nil, err
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

	tx, err := conn.BeginTx(ctx, txOpts)
	if err != nil {
		return nil, err
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

	for _, q := range queries {
		resp, err := executeSingleQueryInTx(ctx, tx, q.Query, q.Params, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, resp)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	tx = nil

	return results, nil
}

func processRows(rows pgx.Rows, fieldDescriptions []pgconn.FieldDescription, opts Options) ([]any, error) {
	results := make([]any, 0)

	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}

		row := make([]any, len(vals))
		for idx, val := range vals {
			val, err = parseValue(val, opts.RawTextOutput, opts.ArrayMode, fieldDescriptions[idx])
			if err != nil {
				return nil, err
			}

			row[idx] = val
		}

		results = append(results, row)
	}

	return results, nil
}
