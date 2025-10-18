package http

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	// ErrInvalidPayloadQueriesAndQueryBothDefined is returned when both
	// queries and query are defined in the payload, which is invalid.
	ErrInvalidPayloadQueriesAndQueryBothDefined = errors.New("both queries and query are defined in the payload")

	// ErrPayloadTooManyQueries is returned when the payload contains more queries than allowed.
	ErrPayloadTooManyQueries = errors.New("payload contains too many queries")

	// ErrQueryTooLong is returned when a query exceeds the maximum allowed length.
	ErrQueryTooLong = errors.New("query exceeds maximum length")
)

// payload is the request payload for the HTTP proxy.
type payload struct {
	Query   string        `json:"query"`
	Params  []interface{} `json:"params"`
	Queries []struct {
		Query  string        `json:"query"`
		Params []interface{} `json:"params"`
	} `json:"queries"`
}

// readPayload parses the HTTP request body into a payload representing
// either a single SQL query or a batch of queries.
// The payload must contain only one form: either a single query with
// optional "params", or a list of queries under "queries", each with
// its own "query" and "params". Including both forms is invalid.
//
// Example of a single query payload:
//
//	{
//	  "query": "SELECT * FROM users WHERE id = $1",
//	  "params": [1]
//	}
//
// Example of a batch query payload:
//
//	{
//	  "queries": [
//	    { "query": "SELECT * FROM users WHERE id = $1", "params": [1] },
//	    { "query": "SELECT * FROM users WHERE id = $1", "params": [2] }
//	  ]
//	}
func readPayload(r *http.Request) (*payload, error) {
	var payload payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}

	if len(payload.Queries) > 0 && (payload.Query != "" || len(payload.Params) > 0) {
		return nil, ErrInvalidPayloadQueriesAndQueryBothDefined
	}

	if len(payload.Queries) > MaxBatchQueries {
		return nil, ErrPayloadTooManyQueries
	}

	if payload.Query != "" && len(payload.Query) > MaxQueryLength {
		return nil, ErrQueryTooLong
	}

	for _, q := range payload.Queries {
		if len(q.Query) > MaxQueryLength {
			return nil, ErrQueryTooLong
		}
	}

	return &payload, nil
}
