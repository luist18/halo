package httpexecutor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

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
