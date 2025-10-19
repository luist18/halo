package errors

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// PGError represents a PostgreSQL error in JSON format compatible with the
// Neon serverless driver. All optional fields use pointers to distinguish
// between empty strings and null values in the JSON output.
//
// Example JSON output:
//
//	{
//	  "message": "syntax error at or near \"SELECTs\"",
//	  "code": "42601",
//	  "detail": null,
//	  "hint": null,
//	  "position": "1",
//	  "internalPosition": null,
//	  "internalQuery": null,
//	  "severity": "ERROR",
//	  "where": null,
//	  "table": null,
//	  "column": null,
//	  "schema": null,
//	  "dataType": null,
//	  "constraint": null,
//	  "file": "scan.l",
//	  "line": "1244",
//	  "routine": "scanner_yyerror"
//	}
//
// See http://www.postgresql.org/docs/current/static/protocol-error-fields.html for
// detailed field description.
type PGError struct {
	Message          string  `json:"message"`
	Code             string  `json:"code"`
	Detail           *string `json:"detail"`
	Hint             *string `json:"hint"`
	Position         *string `json:"position"`
	InternalPosition *string `json:"internalPosition"`
	InternalQuery    *string `json:"internalQuery"`
	Severity         string  `json:"severity"`
	Where            *string `json:"where"`
	Table            *string `json:"table"`
	Column           *string `json:"column"`
	Schema           *string `json:"schema"`
	DataType         *string `json:"dataType"`
	Constraint       *string `json:"constraint"`
	File             string  `json:"file"`
	Line             string  `json:"line"`
	Routine          string  `json:"routine"`
}

// FromPgError populates the PGError fields from a pgconn.PgError instance,
// converting empty strings to nil pointers for proper JSON null representation
func (e *PGError) FromPgError(err *pgconn.PgError) {
	e.Message = err.Message
	e.Code = err.Code
	e.Detail = nullableString(err.Detail)
	e.Hint = nullableString(err.Hint)
	e.Position = nullableStringFromInt32(err.Position)
	e.InternalPosition = nullableStringFromInt32(err.InternalPosition)
	e.InternalQuery = nullableString(err.InternalQuery)
	e.Severity = err.Severity
	e.Where = nullableString(err.Where)
	e.Table = nullableString(err.TableName)
	e.Column = nullableString(err.ColumnName)
	e.Schema = nullableString(err.SchemaName)
	e.DataType = nullableString(err.DataTypeName)
	e.Constraint = nullableString(err.ConstraintName)
	e.File = err.File
	e.Line = fmt.Sprint(err.Line)
	e.Routine = err.Routine
}

// nullableStringFromInt32 converts an int32 to a string pointer, returning nil
// if the value is 0 (which represents unset in PostgreSQL error fields)
func nullableStringFromInt32(i int32) *string {
	if i == 0 {
		return nil
	}
	s := fmt.Sprint(i)
	return &s
}

// nullableString converts a string to a string pointer, returning nil if the
// string is empty to properly represent null values in JSON
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
