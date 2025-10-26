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
	// TODO: remove this, this is a catch all as most if not all types will implement this
	marshaller, ok := val.(json.Marshaler)
	if ok {
		jsonBytes, err := marshaller.MarshalJSON()
		if err != nil {
			return ExecutorResponse{}, err
		}
		val = string(jsonBytes)
		return val, nil
	}

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
		case time.Time:
			switch fd.DataTypeOID {
			case pgtype.TimestampOID:
				val = v.Format("2006-01-02 15:04:05")
			case pgtype.TimestamptzOID:
				val = v.Format("2006-01-02 15:04:05.000000-07")
			case pgtype.DateOID:
				val = v.Format("2006-01-02")
			case pgtype.TimeOID:
				val = v.Format("15:04:05.000000-07")
			}
		case int64:
			// Convert large integers to strings to preserve precision
			// JavaScript's MAX_SAFE_INTEGER is 2^53 - 1 = 9007199254740991
			if v > 9007199254740991 || v < -9007199254740991 {
				val = fmt.Sprint(v)
			}
		case uint, uint32, uint64:
			// For unsigned types, only check the upper bound
			var uval uint64
			switch uv := val.(type) {
			case uint:
				uval = uint64(uv)
			case uint32:
				uval = uint64(uv)
			case uint64:
				uval = uv
			}
			if uval > 9007199254740991 {
				val = fmt.Sprint(uval)
			}
		case pgtype.Numeric:
			numeric := val.(pgtype.Numeric)
			numbytes, _ := numeric.MarshalJSON()
			val = string(numbytes)
		}
	}

	return val, nil
}

func parseCommand(command string) string {
	parts := strings.Split(command, " ")
	return parts[0]
}
