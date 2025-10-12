package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

var ErrInvalidPayloadQueriesAndQueryBothDefined = errors.New("both queries and query are defined in the payload")

// TODO: use encoding v2 and make this more typesafe
type requestPayload struct {
	Query   string        `json:"query"`
	Params  []interface{} `json:"params"`
	Queries []struct {
		Query  string        `json:"query"`
		Params []interface{} `json:"params"`
	} `json:"queries,omitempty"`
}

func readPayload(r *http.Request) (*requestPayload, error) {
	var payload requestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}

	if len(payload.Queries) > 0 && (payload.Query != "" || len(payload.Params) > 0) {
		return nil, ErrInvalidPayloadQueriesAndQueryBothDefined
	}

	return &payload, nil
}
