package http

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadPayload_Success(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    *payload
	}{
		{
			name:    "single query with params",
			payload: `{"query": "SELECT * FROM users WHERE id = $1", "params": [1]}`,
			want: &payload{
				Query:  "SELECT * FROM users WHERE id = $1",
				Params: []interface{}{float64(1)}, // JSON numbers decode as float64
			},
		},
		{
			name:    "single query without params",
			payload: `{"query": "SELECT * FROM users"}`,
			want: &payload{
				Query:  "SELECT * FROM users",
				Params: nil,
			},
		},
		{
			name:    "batch queries",
			payload: `{"queries": [{"query": "SELECT 1", "params": []}, {"query": "SELECT 2", "params": []}]}`,
			want: &payload{
				Queries: []struct {
					Query  string        `json:"query"`
					Params []interface{} `json:"params"`
				}{
					{Query: "SELECT 1", Params: []interface{}{}},
					{Query: "SELECT 2", Params: []interface{}{}},
				},
			},
		},
		{
			name:    "empty queries array",
			payload: `{"queries": []}`,
			want: &payload{
				Queries: []struct {
					Query  string        `json:"query"`
					Params []interface{} `json:"params"`
				}{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Body: io.NopCloser(strings.NewReader(tt.payload)),
			}

			got, err := readPayload(req)
			if err != nil {
				t.Fatalf("readPayload() error = %v, want nil", err)
			}

			if tt.want.Query != "" && got.Query != tt.want.Query {
				t.Errorf("readPayload() query = %v, want %v", got.Query, tt.want.Query)
			}

			if len(tt.want.Queries) > 0 && len(got.Queries) != len(tt.want.Queries) {
				t.Errorf("readPayload() queries length = %v, want %v", len(got.Queries), len(tt.want.Queries))
			}
		})
	}
}

func TestReadPayload_InvalidPayloadBothDefined(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "both query and queries defined",
			payload: `{"query": "SELECT 1", "queries": [{"query": "SELECT 2"}]}`,
		},
		{
			name:    "query and queries with params",
			payload: `{"query": "SELECT 1", "params": [1], "queries": [{"query": "SELECT 2"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Body: io.NopCloser(strings.NewReader(tt.payload)),
			}

			_, err := readPayload(req)
			if err != ErrInvalidPayloadQueriesAndQueryBothDefined {
				t.Errorf("readPayload() error = %v, want %v", err, ErrInvalidPayloadQueriesAndQueryBothDefined)
			}
		})
	}
}

func TestReadPayload_TooManyQueries(t *testing.T) {
	// Create a payload with more than MaxBatchQueries
	queries := make([]string, MaxBatchQueries+1)
	for i := range queries {
		queries[i] = `{"query": "SELECT 1", "params": []}`
	}
	payload := `{"queries": [` + strings.Join(queries, ",") + `]}`

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(payload)),
	}

	_, err := readPayload(req)
	if err != ErrPayloadTooManyQueries {
		t.Errorf("readPayload() error = %v, want %v", err, ErrPayloadTooManyQueries)
	}
}

func TestReadPayload_ExactlyMaxQueries(t *testing.T) {
	// Create a payload with exactly MaxBatchQueries (should succeed)
	queries := make([]string, MaxBatchQueries)
	for i := range queries {
		queries[i] = `{"query": "SELECT 1", "params": []}`
	}
	payload := `{"queries": [` + strings.Join(queries, ",") + `]}`

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(payload)),
	}

	got, err := readPayload(req)
	if err != nil {
		t.Fatalf("readPayload() error = %v, want nil", err)
	}

	if len(got.Queries) != MaxBatchQueries {
		t.Errorf("readPayload() queries length = %v, want %v", len(got.Queries), MaxBatchQueries)
	}
}

func TestReadPayload_QueryTooLong_SingleQuery(t *testing.T) {
	// Create a single query that exceeds MaxQueryLength
	longQuery := strings.Repeat("a", MaxQueryLength+1)
	payload := `{"query": "` + longQuery + `", "params": []}`

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(payload)),
	}

	_, err := readPayload(req)
	if err != ErrQueryTooLong {
		t.Errorf("readPayload() error = %v, want %v", err, ErrQueryTooLong)
	}
}

func TestReadPayload_QueryExactlyMaxLength_SingleQuery(t *testing.T) {
	// Create a single query with exactly MaxQueryLength (should succeed)
	longQuery := strings.Repeat("a", MaxQueryLength)
	payload := `{"query": "` + longQuery + `", "params": []}`

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(payload)),
	}

	got, err := readPayload(req)
	if err != nil {
		t.Fatalf("readPayload() error = %v, want nil", err)
	}

	if len(got.Query) != MaxQueryLength {
		t.Errorf("readPayload() query length = %v, want %v", len(got.Query), MaxQueryLength)
	}
}

func TestReadPayload_QueryTooLong_BatchQuery(t *testing.T) {
	// Create a batch where one query exceeds MaxQueryLength
	longQuery := strings.Repeat("a", MaxQueryLength+1)
	payload := `{"queries": [{"query": "SELECT 1", "params": []}, {"query": "` + longQuery + `", "params": []}]}`

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(payload)),
	}

	_, err := readPayload(req)
	if err != ErrQueryTooLong {
		t.Errorf("readPayload() error = %v, want %v", err, ErrQueryTooLong)
	}
}

func TestReadPayload_QueryTooLong_FirstQueryInBatch(t *testing.T) {
	// Create a batch where the first query exceeds MaxQueryLength
	longQuery := strings.Repeat("a", MaxQueryLength+1)
	payload := `{"queries": [{"query": "` + longQuery + `", "params": []}, {"query": "SELECT 1", "params": []}]}`

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(payload)),
	}

	_, err := readPayload(req)
	if err != ErrQueryTooLong {
		t.Errorf("readPayload() error = %v, want %v", err, ErrQueryTooLong)
	}
}

func TestReadPayload_InvalidJSON(t *testing.T) {
	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(`{"query": "SELECT 1"`)),
	}

	_, err := readPayload(req)
	if err == nil {
		t.Error("readPayload() error = nil, want JSON parse error")
	}
}

func TestReadPayload_EmptyBody(t *testing.T) {
	req := &http.Request{
		Body: io.NopCloser(bytes.NewReader([]byte{})),
	}

	_, err := readPayload(req)
	if err == nil {
		t.Error("readPayload() error = nil, want error")
	}
}

func TestReadPayload_LargeValidPayload(t *testing.T) {
	// Create a payload near the limit but still valid
	// 500 queries, each with a reasonable query length
	queries := make([]string, 500)
	query := strings.Repeat("a", 1000) // 1KB query
	for i := range queries {
		queries[i] = `{"query": "` + query + `", "params": []}`
	}
	payload := `{"queries": [` + strings.Join(queries, ",") + `]}`

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(payload)),
	}

	got, err := readPayload(req)
	if err != nil {
		t.Fatalf("readPayload() error = %v, want nil", err)
	}

	if len(got.Queries) != 500 {
		t.Errorf("readPayload() queries length = %v, want %v", len(got.Queries), 500)
	}
}
