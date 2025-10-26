package httpexecutor

import (
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
			name: "int64 (BIGINT) converts to string",
			val:  int64(9223372036854775807),
			fd: pgconn.FieldDescription{
				Name:        "bigint_col",
				DataTypeOID: pgtype.Int8OID,
			},
			want: "9223372036854775807",
		},
		{
			name: "small int64 converts to string",
			val:  int64(5),
			fd: pgconn.FieldDescription{
				Name:        "count",
				DataTypeOID: pgtype.Int8OID,
			},
			want: "5",
		},
		{
			name: "negative int64 converts to string",
			val:  int64(-9223372036854775808),
			fd: pgconn.FieldDescription{
				Name:        "negative_bigint",
				DataTypeOID: pgtype.Int8OID,
			},
			want: "-9223372036854775808",
		},
		{
			name: "time.Time with TimestampOID",
			val:  time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			fd: pgconn.FieldDescription{
				Name:        "created_at",
				DataTypeOID: pgtype.TimestampOID,
			},
			want: "2023-01-15 10:30:00",
		},
		{
			name: "time.Time with TimestamptzOID",
			val:  time.Date(2023, 1, 15, 10, 30, 0, 0, time.FixedZone("EST", -5*3600)),
			fd: pgconn.FieldDescription{
				Name:        "created_at",
				DataTypeOID: pgtype.TimestamptzOID,
			},
			want: "2023-01-15 10:30:00.000000-05",
		},
		{
			name: "time.Time with DateOID",
			val:  time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			fd: pgconn.FieldDescription{
				Name:        "date_col",
				DataTypeOID: pgtype.DateOID,
			},
			want: "2023-01-15",
		},
		{
			name: "time.Time with TimeOID",
			val:  time.Date(0, 1, 1, 14, 30, 45, 0, time.FixedZone("EST", -5*3600)),
			fd: pgconn.FieldDescription{
				Name:        "time_col",
				DataTypeOID: pgtype.TimeOID,
			},
			want: "14:30:45.000000-05",
		},
		{
			name: "string value passes through",
			val:  "simple string",
			fd: pgconn.FieldDescription{
				Name:        "title",
				DataTypeOID: pgtype.VarcharOID,
			},
			want: "simple string",
		},
		{
			name: "int32 value passes through",
			val:  int32(42),
			fd: pgconn.FieldDescription{
				Name:        "count",
				DataTypeOID: pgtype.Int4OID,
			},
			want: int32(42),
		},
		{
			name: "int value passes through",
			val:  42,
			fd: pgconn.FieldDescription{
				Name:        "count",
				DataTypeOID: pgtype.Int4OID,
			},
			want: 42,
		},
		{
			name: "float64 value passes through",
			val:  45.67,
			fd: pgconn.FieldDescription{
				Name:        "decimal",
				DataTypeOID: pgtype.Float8OID,
			},
			want: 45.67,
		},
		{
			name: "bool value passes through",
			val:  true,
			fd: pgconn.FieldDescription{
				Name:        "flag",
				DataTypeOID: pgtype.BoolOID,
			},
			want: true,
		},
		{
			name: "map[string]any passes through",
			val:  map[string]any{"key": "value"},
			fd: pgconn.FieldDescription{
				Name:        "data",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: map[string]any{"key": "value"},
		},
		{
			name: "slice passes through",
			val:  []any{1, 2, 3},
			fd: pgconn.FieldDescription{
				Name:        "numbers",
				DataTypeOID: pgtype.JSONBOID,
			},
			want: []any{1, 2, 3},
		},
		{
			name: "nil value passes through",
			val:  nil,
			fd: pgconn.FieldDescription{
				Name:        "null_col",
				DataTypeOID: pgtype.TextOID,
			},
			want: nil,
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

func TestParseTime(t *testing.T) {
	tests := []struct {
		name string
		val  time.Time
		fd   pgconn.FieldDescription
		want string
	}{
		{
			name: "Timestamp formatting",
			val:  time.Date(2023, 1, 15, 10, 30, 45, 0, time.UTC),
			fd: pgconn.FieldDescription{
				DataTypeOID: pgtype.TimestampOID,
			},
			want: "2023-01-15 10:30:45",
		},
		{
			name: "Timestamptz formatting with timezone",
			val:  time.Date(2023, 1, 15, 10, 30, 45, 123456000, time.FixedZone("PST", -8*3600)),
			fd: pgconn.FieldDescription{
				DataTypeOID: pgtype.TimestamptzOID,
			},
			want: "2023-01-15 10:30:45.123456-08",
		},
		{
			name: "Date formatting",
			val:  time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			fd: pgconn.FieldDescription{
				DataTypeOID: pgtype.DateOID,
			},
			want: "2023-12-25",
		},
		{
			name: "Time formatting with timezone",
			val:  time.Date(0, 1, 1, 15, 45, 30, 500000000, time.FixedZone("EST", -5*3600)),
			fd: pgconn.FieldDescription{
				DataTypeOID: pgtype.TimeOID,
			},
			want: "15:45:30.500000-05",
		},
		{
			name: "Unknown OID defaults to Timestamp format",
			val:  time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
			fd: pgconn.FieldDescription{
				DataTypeOID: 9999, // Unknown OID
			},
			want: "2023-06-15 12:00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTime(tt.val, tt.fd)
			if got != tt.want {
				t.Errorf("parseTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
