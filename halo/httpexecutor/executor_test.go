package httpexecutor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/luist18/halo/internal/secret"
)

func TestReturnValue(t *testing.T) {
	tests := []struct {
		name       string
		val        any
		columnName string
		arrayMode  bool
		want       any
	}{
		{
			name:       "nil value with arrayMode true",
			val:        nil,
			columnName: "id",
			arrayMode:  true,
			want:       nil,
		},
		{
			name:       "nil value with arrayMode false",
			val:        nil,
			columnName: "name",
			arrayMode:  false,
			want:       nil,
		},
		{
			name:       "string value with arrayMode true",
			val:        "test string",
			columnName: "title",
			arrayMode:  true,
			want:       "test string",
		},
		{
			name:       "string value with arrayMode false",
			val:        "test string",
			columnName: "title",
			arrayMode:  false,
			want: map[string]any{
				"title": "test string",
			},
		},
		{
			name:       "int value with arrayMode true",
			val:        42,
			columnName: "count",
			arrayMode:  true,
			want:       42,
		},
		{
			name:       "int value with arrayMode false",
			val:        42,
			columnName: "count",
			arrayMode:  false,
			want: map[string]any{
				"count": 42,
			},
		},
		{
			name:       "float value with arrayMode true",
			val:        3.14,
			columnName: "price",
			arrayMode:  true,
			want:       3.14,
		},
		{
			name:       "float value with arrayMode false",
			val:        3.14,
			columnName: "price",
			arrayMode:  false,
			want: map[string]any{
				"price": 3.14,
			},
		},
		{
			name:       "bool value with arrayMode true",
			val:        true,
			columnName: "active",
			arrayMode:  true,
			want:       true,
		},
		{
			name:       "bool value with arrayMode false",
			val:        false,
			columnName: "active",
			arrayMode:  false,
			want: map[string]any{
				"active": false,
			},
		},
		{
			name:       "map value with arrayMode true",
			val:        map[string]any{"key": "value"},
			columnName: "data",
			arrayMode:  true,
			want:       map[string]any{"key": "value"},
		},
		{
			name:       "map value with arrayMode false",
			val:        map[string]any{"key": "value"},
			columnName: "data",
			arrayMode:  false,
			want: map[string]any{
				"data": map[string]any{"key": "value"},
			},
		},
		{
			name:       "slice value with arrayMode true",
			val:        []int{1, 2, 3},
			columnName: "numbers",
			arrayMode:  true,
			want:       []int{1, 2, 3},
		},
		{
			name:       "slice value with arrayMode false",
			val:        []int{1, 2, 3},
			columnName: "numbers",
			arrayMode:  false,
			want: map[string]any{
				"numbers": []int{1, 2, 3},
			},
		},
		{
			name:       "empty string with arrayMode false",
			val:        "",
			columnName: "description",
			arrayMode:  false,
			want: map[string]any{
				"description": "",
			},
		},
		{
			name:       "zero value int with arrayMode false",
			val:        0,
			columnName: "count",
			arrayMode:  false,
			want: map[string]any{
				"count": 0,
			},
		},
		{
			name:       "column name with special characters",
			val:        "test",
			columnName: "user_name",
			arrayMode:  false,
			want: map[string]any{
				"user_name": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := returnValue(tt.val, tt.columnName, tt.arrayMode)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("returnValue() = %v (type: %T), want %v (type: %T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestPgRawValue(t *testing.T) {
	testTime := time.Date(2023, 10, 15, 12, 30, 45, 0, time.UTC)

	tests := []struct {
		name    string
		val     any
		want    any
		wantErr bool
	}{
		{
			name: "byte slice to string",
			val:  []byte("hello world"),
			want: "hello world",
		},
		{
			name: "map to JSON string",
			val:  map[string]any{"key": "value", "count": 42},
			want: `{"count":42,"key":"value"}`,
		},
		{
			name: "slice to JSON string",
			val:  []any{1, 2, 3, "four"},
			want: `[1,2,3,"four"]`,
		},
		{
			name: "nested map to JSON string",
			val:  map[string]any{"nested": map[string]any{"inner": "value"}},
			want: `{"nested":{"inner":"value"}}`,
		},
		{
			name: "time.Time to RFC3339",
			val:  testTime,
			want: "2023-10-15T12:30:45Z",
		},
		{
			name: "string value",
			val:  "simple string",
			want: "simple string",
		},
		{
			name: "int value",
			val:  42,
			want: "42",
		},
		{
			name: "float value",
			val:  3.14159,
			want: "3.14159",
		},
		{
			name: "bool value true",
			val:  true,
			want: "true",
		},
		{
			name: "bool value false",
			val:  false,
			want: "false",
		},
		{
			name: "empty string",
			val:  "",
			want: "",
		},
		{
			name: "zero int",
			val:  0,
			want: "0",
		},
		{
			name: "empty map",
			val:  map[string]any{},
			want: "{}",
		},
		{
			name: "empty slice",
			val:  []any{},
			want: "[]",
		},
		{
			name: "empty byte slice",
			val:  []byte{},
			want: "",
		},
		{
			name:    "map with unmarshalable value (channel)",
			val:     map[string]any{"channel": make(chan int)},
			wantErr: true,
		},
		{
			name:    "slice with unmarshalable value (channel)",
			val:     []any{make(chan int)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pgRawValue(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("pgRawValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pgRawValue() = %v (type: %T), want %v (type: %T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestPgValue(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		fd      pgconn.FieldDescription
		want    any
		wantErr bool
	}{
		{
			name: "JSON byte slice",
			val:  []byte(`{"key":"value"}`),
			fd: pgconn.FieldDescription{
				Name:        "data",
				DataTypeOID: pgtype.JSONOID,
			},
			want: map[string]any{"key": "value"},
		},
		{
			name: "JSONB byte slice",
			val:  []byte(`{"count":42}`),
			fd: pgconn.FieldDescription{
				Name:        "metadata",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: map[string]any{"count": float64(42)},
		},
		{
			name: "JSON string",
			val:  `{"name":"test"}`,
			fd: pgconn.FieldDescription{
				Name:        "config",
				DataTypeOID: pgtype.JSONOID,
			},
			want: map[string]any{"name": "test"},
		},
		{
			name: "JSON array",
			val:  []byte(`[1,2,3]`),
			fd: pgconn.FieldDescription{
				Name:        "numbers",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: []any{float64(1), float64(2), float64(3)},
		},
		{
			name: "JSON map[string]any",
			val:  map[string]any{"key": "value"},
			fd: pgconn.FieldDescription{
				Name:        "data",
				DataTypeOID: pgtype.JSONOID,
			},
			want: map[string]any{"key": "value"},
		},
		{
			name: "non-JSON byte slice",
			val:  []byte("plain text"),
			fd: pgconn.FieldDescription{
				Name:        "name",
				DataTypeOID: pgtype.TextOID,
			},
			want: "plain text",
		},
		{
			name: "non-JSON string value",
			val:  "simple string",
			fd: pgconn.FieldDescription{
				Name:        "title",
				DataTypeOID: pgtype.VarcharOID,
			},
			want: "simple string",
		},
		{
			name: "non-JSON int value",
			val:  42,
			fd: pgconn.FieldDescription{
				Name:        "count",
				DataTypeOID: pgtype.Int4OID,
			},
			want: 42,
		},
		{
			name: "empty byte slice",
			val:  []byte{},
			fd: pgconn.FieldDescription{
				Name:        "data",
				DataTypeOID: pgtype.TextOID,
			},
			want: "",
		},
		{
			name: "nested JSON object",
			val:  `{"outer":{"inner":"value"}}`,
			fd: pgconn.FieldDescription{
				Name:        "nested",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: map[string]any{"outer": map[string]any{"inner": "value"}},
		},
		{
			name: "invalid JSON",
			val:  []byte(`{invalid json`),
			fd: pgconn.FieldDescription{
				Name:        "bad",
				DataTypeOID: pgtype.JSONOID,
			},
			wantErr: true,
		},
		{
			name: "JSON default case - int value",
			val:  123,
			fd: pgconn.FieldDescription{
				Name:        "number",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: float64(123),
		},
		{
			name: "JSON default case - float value",
			val:  45.67,
			fd: pgconn.FieldDescription{
				Name:        "decimal",
				DataTypeOID: pgtype.JSONOID,
			},
			want: float64(45.67),
		},
		{
			name: "JSON default case - bool value",
			val:  true,
			fd: pgconn.FieldDescription{
				Name:        "flag",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: true,
		},
		{
			name: "JSON map with unmarshalable value (channel)",
			val:  map[string]any{"channel": make(chan int)},
			fd: pgconn.FieldDescription{
				Name:        "bad_map",
				DataTypeOID: pgtype.JSONOID,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pgValue(tt.val, tt.fd)
			if (err != nil) != tt.wantErr {
				t.Errorf("pgValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pgValue() = %v (type: %T), want %v (type: %T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	testTime := time.Date(2023, 10, 15, 12, 30, 45, 0, time.UTC)

	tests := []struct {
		name          string
		val           any
		rawTextOutput bool
		arrayMode     bool
		fd            pgconn.FieldDescription
		want          any
		wantErr       bool
	}{
		{
			name:          "raw mode - string in array mode",
			val:           "test",
			rawTextOutput: true,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "name",
				DataTypeOID: pgtype.TextOID,
			},
			want: "test",
		},
		{
			name:          "raw mode - string in object mode",
			val:           "test",
			rawTextOutput: true,
			arrayMode:     false,
			fd: pgconn.FieldDescription{
				Name:        "name",
				DataTypeOID: pgtype.TextOID,
			},
			want: map[string]any{"name": "test"},
		},
		{
			name:          "raw mode - time in array mode",
			val:           testTime,
			rawTextOutput: true,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "created_at",
				DataTypeOID: pgtype.TimestampOID,
			},
			want: "2023-10-15T12:30:45Z",
		},
		{
			name:          "raw mode - map to JSON string in object mode",
			val:           map[string]any{"key": "value"},
			rawTextOutput: true,
			arrayMode:     false,
			fd: pgconn.FieldDescription{
				Name:        "data",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: map[string]any{"data": `{"key":"value"}`},
		},
		{
			name:          "normal mode - JSON in array mode",
			val:           []byte(`{"key":"value"}`),
			rawTextOutput: false,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "data",
				DataTypeOID: pgtype.JSONOID,
			},
			want: map[string]any{"key": "value"},
		},
		{
			name:          "normal mode - JSON in object mode",
			val:           []byte(`{"key":"value"}`),
			rawTextOutput: false,
			arrayMode:     false,
			fd: pgconn.FieldDescription{
				Name:        "data",
				DataTypeOID: pgtype.JSONOID,
			},
			want: map[string]any{"data": map[string]any{"key": "value"}},
		},
		{
			name:          "normal mode - byte slice in array mode",
			val:           []byte("text"),
			rawTextOutput: false,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "name",
				DataTypeOID: pgtype.TextOID,
			},
			want: "text",
		},
		{
			name:          "normal mode - int in object mode",
			val:           42,
			rawTextOutput: false,
			arrayMode:     false,
			fd: pgconn.FieldDescription{
				Name:        "count",
				DataTypeOID: pgtype.Int4OID,
			},
			want: map[string]any{"count": 42},
		},
		{
			name:          "raw mode - int to string in array mode",
			val:           123,
			rawTextOutput: true,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "id",
				DataTypeOID: pgtype.Int4OID,
			},
			want: "123",
		},
		{
			name:          "raw mode - slice to JSON in array mode",
			val:           []any{1, 2, 3},
			rawTextOutput: true,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "numbers",
				DataTypeOID: pgtype.Int4OID,
			},
			want: "[1,2,3]",
		},
		{
			name:          "raw mode - error from pgRawValue with unmarshalable map",
			val:           map[string]any{"channel": make(chan int)},
			rawTextOutput: true,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "bad_data",
				DataTypeOID: pgtype.TextOID,
			},
			wantErr: true,
		},
		{
			name:          "raw mode - error from pgRawValue with unmarshalable slice",
			val:           []any{make(chan int)},
			rawTextOutput: true,
			arrayMode:     false,
			fd: pgconn.FieldDescription{
				Name:        "bad_array",
				DataTypeOID: pgtype.TextOID,
			},
			wantErr: true,
		},
		{
			name:          "normal mode - error from pgValue with invalid JSON",
			val:           []byte(`{bad json`),
			rawTextOutput: false,
			arrayMode:     true,
			fd: pgconn.FieldDescription{
				Name:        "invalid",
				DataTypeOID: pgtype.JSONOID,
			},
			wantErr: true,
		},
		{
			name:          "normal mode - error from pgValue with unmarshalable map",
			val:           map[string]any{"channel": make(chan int)},
			rawTextOutput: false,
			arrayMode:     false,
			fd: pgconn.FieldDescription{
				Name:        "bad_json",
				DataTypeOID: pgtype.JSONBOID,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValue(tt.val, tt.rawTextOutput, tt.arrayMode, tt.fd)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseValue() = %v (type: %T), want %v (type: %T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestFields(t *testing.T) {
	tests := []struct {
		name string
		fds  []pgconn.FieldDescription
		want []field
	}{
		{
			name: "single field",
			fds: []pgconn.FieldDescription{
				{
					Name:                 "id",
					DataTypeOID:          pgtype.Int4OID,
					TableOID:             12345,
					TableAttributeNumber: 1,
					DataTypeSize:         4,
					TypeModifier:         -1,
				},
			},
			want: []field{
				{
					Name:             "id",
					DataTypeID:       pgtype.Int4OID,
					TableID:          12345,
					ColumnID:         1,
					DataTypeSize:     4,
					DataTypeModifier: -1,
					Format:           "text",
				},
			},
		},
		{
			name: "multiple fields",
			fds: []pgconn.FieldDescription{
				{
					Name:                 "id",
					DataTypeOID:          pgtype.Int4OID,
					TableOID:             12345,
					TableAttributeNumber: 1,
					DataTypeSize:         4,
					TypeModifier:         -1,
				},
				{
					Name:                 "name",
					DataTypeOID:          pgtype.TextOID,
					TableOID:             12345,
					TableAttributeNumber: 2,
					DataTypeSize:         -1,
					TypeModifier:         -1,
				},
				{
					Name:                 "data",
					DataTypeOID:          pgtype.JSONBOID,
					TableOID:             12345,
					TableAttributeNumber: 3,
					DataTypeSize:         -1,
					TypeModifier:         -1,
				},
			},
			want: []field{
				{
					Name:             "id",
					DataTypeID:       pgtype.Int4OID,
					TableID:          12345,
					ColumnID:         1,
					DataTypeSize:     4,
					DataTypeModifier: -1,
					Format:           "text",
				},
				{
					Name:             "name",
					DataTypeID:       pgtype.TextOID,
					TableID:          12345,
					ColumnID:         2,
					DataTypeSize:     -1,
					DataTypeModifier: -1,
					Format:           "text",
				},
				{
					Name:             "data",
					DataTypeID:       pgtype.JSONBOID,
					TableID:          12345,
					ColumnID:         3,
					DataTypeSize:     -1,
					DataTypeModifier: -1,
					Format:           "text",
				},
			},
		},
		{
			name: "empty fields",
			fds:  []pgconn.FieldDescription{},
			want: []field{},
		},
		{
			name: "field with zero values",
			fds: []pgconn.FieldDescription{
				{
					Name:                 "col",
					DataTypeOID:          0,
					TableOID:             0,
					TableAttributeNumber: 0,
					DataTypeSize:         0,
					TypeModifier:         0,
				},
			},
			want: []field{
				{
					Name:             "col",
					DataTypeID:       0,
					TableID:          0,
					ColumnID:         0,
					DataTypeSize:     0,
					DataTypeModifier: 0,
					Format:           "text",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fields(tt.fds)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "SELECT command",
			command: "SELECT 1",
			want:    "SELECT",
		},
		{
			name:    "INSERT command",
			command: "INSERT 0 1",
			want:    "INSERT",
		},
		{
			name:    "UPDATE command",
			command: "UPDATE 5",
			want:    "UPDATE",
		},
		{
			name:    "DELETE command",
			command: "DELETE 3",
			want:    "DELETE",
		},
		{
			name:    "CREATE command",
			command: "CREATE TABLE",
			want:    "CREATE",
		},
		{
			name:    "DROP command",
			command: "DROP TABLE",
			want:    "DROP",
		},
		{
			name:    "single word command",
			command: "COMMIT",
			want:    "COMMIT",
		},
		{
			name:    "empty command",
			command: "",
			want:    "",
		},
		{
			name:    "command with multiple spaces",
			command: "SELECT  10",
			want:    "SELECT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommand(tt.command)
			if got != tt.want {
				t.Errorf("parseCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecute_PoolOptInNotImplemented(t *testing.T) {
	tests := []struct {
		name      string
		opts      Options
		wantErr   error
		shouldErr bool
	}{
		{
			name: "pool opt-in enabled returns error",
			opts: Options{
				RawTextOutput:       true,
				ArrayMode:           true,
				PoolOptIn:           true,
				BatchIsolationLevel: "ReadCommitted",
				BatchReadOnly:       false,
				BatchDeferrable:     false,
			},
			wantErr:   ErrPoolOptInNotImplemented,
			shouldErr: true,
		},
		{
			name: "pool opt-in disabled should not return error for pool feature",
			opts: Options{
				RawTextOutput:       true,
				ArrayMode:           true,
				PoolOptIn:           false,
				BatchIsolationLevel: "ReadCommitted",
				BatchReadOnly:       false,
				BatchDeferrable:     false,
			},
			wantErr:   nil,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			connStr := secret.NewSecret("postgres://user:password@localhost:5432/dbname")
			payload := Payload{
				Query:  "SELECT 1",
				Params: []interface{}{},
			}

			_, err := Execute(ctx, *connStr, payload, tt.opts)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Execute() expected error but got nil")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Execute() error = %v, want %v", err, tt.wantErr)
				}
			} else if tt.opts.PoolOptIn {
				// If PoolOptIn is false, we expect other errors (like connection errors)
				// but NOT the ErrPoolOptInNotImplemented
				if errors.Is(err, ErrPoolOptInNotImplemented) {
					t.Errorf("Execute() should not return ErrPoolOptInNotImplemented when PoolOptIn is false, got: %v", err)
				}
			}
		})
	}
}
