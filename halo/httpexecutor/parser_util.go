package httpexecutor

import (
	"encoding/json"
	"strconv"
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

func parseCommand(command string) string {
	parts := strings.Split(command, " ")
	return parts[0]
}

func pgValue(val any, fd pgconn.FieldDescription) (any, error) {
	if v, ok := val.(time.Time); ok {
		return parseTime(v, fd), nil
	}

	if v, ok := val.(int64); ok {
		return strconv.FormatInt(v, 10), nil
	}

	marshaller, ok := val.(json.Marshaler)
	if ok {
		jsonBytes, err := marshaller.MarshalJSON()
		if err != nil {
			return nil, err
		}
		val = string(jsonBytes)
		return val, nil
	}

	return val, nil
}

func parseTime(v time.Time, fd pgconn.FieldDescription) string {
	switch fd.DataTypeOID {
	case pgtype.TimestampOID:
		return v.Format("2006-01-02 15:04:05")
	case pgtype.TimestamptzOID:
		return v.Format("2006-01-02 15:04:05.000000-07")
	case pgtype.DateOID:
		return v.Format("2006-01-02")
	case pgtype.TimeOID:
		return v.Format("15:04:05.000000-07")
	default:
		return v.Format("2006-01-02 15:04:05")
	}
}
