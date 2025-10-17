package http

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrInvalidPayloadQueriesAndQueryBothDefined is returned when both
// queries and query are defined in the payload, which is invalid.
var ErrInvalidPayloadQueriesAndQueryBothDefined = errors.New("both queries and query are defined in the payload")

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

	return &payload, nil
}
