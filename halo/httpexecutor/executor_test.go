package httpexecutor

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/luist18/halo/internal/data"
)

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
			connStr := data.NewSecret("postgres://user:password@localhost:5432/dbname")
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
