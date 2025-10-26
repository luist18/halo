package httpexecutor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// MaxSafeInteger is JavaScript's MAX_SAFE_INTEGER (2^53 - 1)
	// Beyond this value, JavaScript cannot safely represent integers
	MaxSafeInteger = 9007199254740991
	// MinSafeInteger is JavaScript's MIN_SAFE_INTEGER (-(2^53 - 1))
	MinSafeInteger = -9007199254740991
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

func pgValue(val any, fd pgconn.FieldDescription) (any, error) {
	// TODO: remove this, this is a catch all as most if not all types will implement this
	marshaller, ok := val.(json.Marshaler)
	if ok {
		jsonBytes, err := marshaller.MarshalJSON()
		if err != nil {
			return nil, err
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
				return nil, err
			}
			valStr = string(marshalled)
		case string:
			valStr = v
		default:
			valStr = fmt.Sprint(v)
		}
		if err := json.Unmarshal([]byte(valStr), &jsonVal); err != nil {
			return nil, err
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
			if v > MaxSafeInteger || v < MinSafeInteger {
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
			if uval > MaxSafeInteger {
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
