package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/luist18/halo/internal/secret"
)

var ErrInvalidQueryMode = errors.New("invalid query mode")

type Options struct {
	RawTextOutput       bool
	ArrayMode           bool
	PoolOptIn           bool
	BatchIsolationLevel string
	BatchReadOnly       bool
	BatchDeferrable     bool
}

type Payload struct {
	Query   string        `json:"query"`
	Params  []interface{} `json:"params"`
	Queries []struct {
		Query  string        `json:"query"`
		Params []interface{} `json:"params"`
	} `json:"queries,omitempty"`
}

type ExecutorResponse struct {
	Fields      any    `json:"fields"`
	Rows        []any  `json:"rows"`
	Command     string `json:"command,omitempty"`
	RowCount    int    `json:"rowCount,omitempty"`
	RowsAsArray bool   `json:"rowsAsArray,omitempty"`
}

type ExecutorResult struct {
	Results []ExecutorResponse
	IsBatch bool
}

func Execute(ctx context.Context, connStrSecret secret.Secret, payload Payload, opts Options) (ExecutorResult, error) {
	connStr := connStrSecret.Unwrap()
	if connStr == "" {
		return ExecutorResult{}, ErrInvalidConnectionString
	}

	// TODO(PER-2): check if conn str is a postgres one

	// TODO(PER-3): max payload

	// TODO: read other configuration parameters

	// if len(payload.Queries) == 0 {
	// 	http.Error(w, "No Queries", http.StatusBadRequest)
	// }

	// TODO: maintain a pool
	config, err := pgx.ParseConfig(connStr)
	if err != nil {
		return ExecutorResult{}, err
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return ExecutorResult{}, err
	}
	defer conn.Close(ctx)

	queryMode := getQueryMode(payload)
	switch queryMode {
	case QueryModeSingle:
		resp, err := executeSingleQuery(ctx, conn, payload.Query, payload.Params, opts)
		if err != nil {
			return ExecutorResult{}, err
		}
		return ExecutorResult{
			Results: []ExecutorResponse{resp},
			IsBatch: false,
		}, nil
	case QueryModeBatch:
		results, err := executeBatchQuery(ctx, conn, []struct {
			Query  string
			Params []interface{}
		}(payload.Queries), opts)
		if err != nil {
			return ExecutorResult{}, err
		}
		return ExecutorResult{
			Results: results,
			IsBatch: true,
		}, nil
	default:
		return ExecutorResult{}, ErrInvalidQueryMode
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

	// Get field descriptions from pgx which has access to the wire protocol
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

	// Get field descriptions from pgx which has access to the wire protocol
	fieldDescriptions := rows.FieldDescriptions()
	fields := fields(fieldDescriptions)

	// Process rows
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

func executeBatchQuery(ctx context.Context, conn *pgx.Conn, queries []struct {
	Query  string
	Params []interface{}
}, opts Options) ([]ExecutorResponse, error) {
	// Parse isolation level
	isolationLevel, err := parseIsolationLevel(opts.BatchIsolationLevel)
	if err != nil {
		return nil, err
	}

	// Build transaction options
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

	// Begin transaction
	tx, err := conn.BeginTx(ctx, txOpts)
	if err != nil {
		return nil, err
	}

	// Ensure rollback if we don't commit
	defer func() {
		if tx != nil {
			tx.Rollback(ctx)
		}
	}()

	results := make([]ExecutorResponse, 0, len(queries))

	// Execute all queries in the transaction
	for _, q := range queries {
		resp, err := executeSingleQueryInTx(ctx, tx, q.Query, q.Params, opts)
		if err != nil {
			return nil, err
		}
		results = append(results, resp)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Set tx to nil to prevent rollback in defer
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

		// Convert values based on raw output mode
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

func returnValue(val any, columnName string, arrayMode bool) any {
	if val != nil {
		if arrayMode {
			return val
		} else {
			return map[string]any{
				columnName: val,
			}
		}
	}

	return val
}

type field struct {
	Name             string `json:"name"`
	DataTypeID       uint32 `json:"dataTypeID"`
	TableID          uint32 `json:"tableID"`
	ColumnID         int16  `json:"columnID"`
	DataTypeSize     int16  `json:"dataTypeSize"`
	DataTypeModifier int32  `json:"dataTypeModifier"`
	Format           string `json:"format"`
}

func fields(fds []pgconn.FieldDescription) []field {
	fields := make([]field, 0, len(fds))
	for _, fd := range fds {
		fields = append(fields, field{
			Name:             fd.Name,
			DataTypeID:       fd.DataTypeOID,
			TableID:          fd.TableOID,
			ColumnID:         int16(fd.TableAttributeNumber),
			DataTypeSize:     fd.DataTypeSize,
			DataTypeModifier: fd.TypeModifier,
			Format:           "text",
		})
	}

	return fields
}

func pgRawValue(val any) (any, error) {
	// In raw output mode, convert all values to strings
	switch v := val.(type) {
	case []byte:
		val = string(v)
	case map[string]any, []any:
		// JSON-encode complex types instead of using fmt.Sprint
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return ExecutorResponse{}, err
		}
		val = string(jsonBytes)
	case time.Time:
		// Use ISO 8601 format for consistency
		val = v.Format(time.RFC3339)
	default:
		val = fmt.Sprint(v)
	}

	return val, nil
}

func pgValue(val any, fd pgconn.FieldDescription) (any, error) {
	// In normal mode, perform type-specific conversions

	// Check if it's JSON or JSONB type
	if fd.DataTypeOID == pgtype.JSONOID || fd.DataTypeOID == pgtype.JSONBOID {
		// Parse JSON/JSONB as actual JSON objects
		var jsonVal any
		valStr := ""
		switch v := val.(type) {
		case []byte:
			valStr = string(v)
		case map[string]any:
			marshalled, err := json.Marshal(v)
			if err != nil {
				return ExecutorResponse{}, err
			}
			valStr = string(marshalled)
		case string:
			valStr = v
		default:
			valStr = fmt.Sprint(v)
		}
		if err := json.Unmarshal([]byte(valStr), &jsonVal); err != nil {
			return ExecutorResponse{}, err
		}
		val = jsonVal
	} else {
		// For other types, convert byte slices to strings
		// to avoid base64 encoding in JSON output
		switch v := val.(type) {
		case []byte:
			val = string(v)
		case bool:
			val = val == "t"
		}
	}

	return val, nil
}

func parseValue(val any, rawTextOutput bool, arrayMode bool, fd pgconn.FieldDescription) (any, error) {
	var err error
	if rawTextOutput {
		val, err = pgRawValue(val)
		if err != nil {
			return nil, err
		}
	} else {
		val, err = pgValue(val, fd)
		if err != nil {
			return nil, err
		}
	}

	return returnValue(val, fd.Name, arrayMode), nil
}

func parseCommand(command string) string {
	parts := strings.Split(command, " ")
	return parts[0]
}
