package http

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSQL_PayloadSizeLimit_ContentLength(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	tests := []struct {
		name           string
		contentLength  int64
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "content length within limit",
			contentLength:  1024,                  // 1KB
			wantStatusCode: http.StatusBadRequest, // Will fail on missing connection string, but passes size check
			wantError:      false,
		},
		{
			name:           "content length at max limit",
			contentLength:  MaxPayloadSize,
			wantStatusCode: http.StatusBadRequest, // Will fail on missing connection string, but passes size check
			wantError:      false,
		},
		{
			name:           "content length exceeds max limit",
			contentLength:  MaxPayloadSize + 1,
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantError:      true,
		},
		{
			name:           "content length far exceeds max limit",
			contentLength:  MaxPayloadSize * 2,
			wantStatusCode: http.StatusRequestEntityTooLarge,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request with the specified content length
			body := bytes.NewReader(make([]byte, 100)) // Small body, we're testing header check
			req := httptest.NewRequest(http.MethodPost, "/sql", body)
			req.ContentLength = tt.contentLength

			rr := httptest.NewRecorder()
			errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("handleSQL() status = %v, want %v", rr.Code, tt.wantStatusCode)
			}

			if tt.wantError && !strings.Contains(rr.Body.String(), "payload too large") {
				t.Errorf("handleSQL() body = %v, want error message about payload size", rr.Body.String())
			}
		})
	}
}

func TestHandleSQL_MaxBytesReader(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	// Create a payload larger than MaxPayloadSize
	largePayload := bytes.Repeat([]byte("a"), MaxPayloadSize+1000)

	req := httptest.NewRequest(http.MethodPost, "/sql", bytes.NewReader(largePayload))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost/db")
	req.ContentLength = int64(len(largePayload))

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	// Should be rejected due to Content-Length check
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("handleSQL() status = %v, want %v", rr.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestHandleSQL_MaxBytesReader_NoContentLength(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	// Create a payload larger than MaxPayloadSize without setting Content-Length
	// This tests the MaxBytesReader enforcement
	largePayload := bytes.Repeat([]byte("a"), MaxPayloadSize+1000)

	req := httptest.NewRequest(http.MethodPost, "/sql", bytes.NewReader(largePayload))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost/db")
	// Explicitly set ContentLength to -1 to simulate unknown size
	req.ContentLength = -1

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	// The MaxBytesReader should still enforce the limit
	// We expect an error when trying to read the body
	if rr.Code == http.StatusOK {
		t.Error("handleSQL() should not return OK for oversized payload")
	}
}

func TestHandleSQL_MethodNotAllowed(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/sql", nil)
			rr := httptest.NewRecorder()

			errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("handleSQL() status = %v, want %v", rr.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleSQL_MissingConnectionString(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	req := httptest.NewRequest(http.MethodPost, "/sql", strings.NewReader(`{"query": "SELECT 1"}`))
	req.ContentLength = int64(len(`{"query": "SELECT 1"}`))
	// No connection string header set

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handleSQL() status = %v, want %v", rr.Code, http.StatusBadRequest)
	}

	if !strings.Contains(rr.Body.String(), "missing connection string") {
		t.Errorf("handleSQL() body = %v, want error about missing connection string", rr.Body.String())
	}
}

func TestHandleSQL_InvalidPayloadJSON(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	req := httptest.NewRequest(http.MethodPost, "/sql", strings.NewReader(`{"query": "SELECT 1"`))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost:5432/db")
	req.ContentLength = int64(len(`{"query": "SELECT 1"`))

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handleSQL() status = %v, want %v", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleSQL_TooManyQueries(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	// Create a payload with more than MaxBatchQueries
	queries := make([]string, MaxBatchQueries+1)
	for i := range queries {
		queries[i] = `{"query": "SELECT 1", "params": []}`
	}
	payload := `{"queries": [` + strings.Join(queries, ",") + `]}`

	req := httptest.NewRequest(http.MethodPost, "/sql", strings.NewReader(payload))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost:5432/db")
	req.ContentLength = int64(len(payload))

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handleSQL() status = %v, want %v", rr.Code, http.StatusBadRequest)
	}

	if !strings.Contains(rr.Body.String(), "too many queries") {
		t.Errorf("handleSQL() body = %v, want error about too many queries", rr.Body.String())
	}
}

func TestHandleSQL_QueryTooLong(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	// Create a query that exceeds MaxQueryLength
	longQuery := strings.Repeat("a", MaxQueryLength+1)
	payload := `{"query": "` + longQuery + `", "params": []}`

	req := httptest.NewRequest(http.MethodPost, "/sql", strings.NewReader(payload))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost:5432/db")
	req.ContentLength = int64(len(payload))

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handleSQL() status = %v, want %v", rr.Code, http.StatusBadRequest)
	}

	if !strings.Contains(rr.Body.String(), "exceeds maximum length") {
		t.Errorf("handleSQL() body = %v, want error about query length", rr.Body.String())
	}
}

func TestHandleSQL_ValidPayloadWithinLimits(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	// Create a valid payload within all limits
	payload := `{"query": "SELECT 1", "params": []}`

	req := httptest.NewRequest(http.MethodPost, "/sql", strings.NewReader(payload))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost:5432/db")
	req.ContentLength = int64(len(payload))

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	// It will fail at execution stage (no real DB), but should pass all size validations
	// We expect either StatusInternalServerError (DB connection fails) or StatusBadRequest (invalid connection string)
	if rr.Code == http.StatusRequestEntityTooLarge {
		t.Error("handleSQL() should not return RequestEntityTooLarge for valid payload size")
	}

	if strings.Contains(rr.Body.String(), "too many queries") || strings.Contains(rr.Body.String(), "exceeds maximum length") {
		t.Errorf("handleSQL() should not return size validation errors for valid payload")
	}
}

func TestHandleSQL_BatchQueriesAtLimit(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	// Create a payload with exactly MaxBatchQueries (should pass validation)
	queries := make([]string, MaxBatchQueries)
	for i := range queries {
		queries[i] = `{"query": "SELECT 1", "params": []}`
	}
	payload := `{"queries": [` + strings.Join(queries, ",") + `]}`

	req := httptest.NewRequest(http.MethodPost, "/sql", strings.NewReader(payload))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost:5432/db")
	req.ContentLength = int64(len(payload))

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	// Should not fail on size validation
	if strings.Contains(rr.Body.String(), "too many queries") {
		t.Error("handleSQL() should not return too many queries error for exactly MaxBatchQueries")
	}
}

func TestHandleSQL_QueryAtMaxLength(t *testing.T) {
	proxy := NewHttpProxy(8080, "/sql")

	// Create a query with exactly MaxQueryLength (should pass validation)
	longQuery := strings.Repeat("a", MaxQueryLength)
	payload := `{"query": "` + longQuery + `", "params": []}`

	req := httptest.NewRequest(http.MethodPost, "/sql", strings.NewReader(payload))
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost:5432/db")
	req.ContentLength = int64(len(payload))

	rr := httptest.NewRecorder()
	errorHandler(proxy.handleSQL).ServeHTTP(rr, req)

	// Should not fail on size validation
	if strings.Contains(rr.Body.String(), "exceeds maximum length") {
		t.Error("handleSQL() should not return query too long error for exactly MaxQueryLength")
	}
}

func TestMaxBytesReader_Integration(t *testing.T) {
	// Test that MaxBytesReader actually limits the read
	largeData := bytes.Repeat([]byte("x"), MaxPayloadSize+1000)
	reader := io.NopCloser(bytes.NewReader(largeData))

	// Create a mock ResponseWriter
	rr := httptest.NewRecorder()

	// Wrap with MaxBytesReader
	limitedReader := http.MaxBytesReader(rr, reader, MaxPayloadSize)

	// Try to read all data
	data, err := io.ReadAll(limitedReader)

	// Should either get an error or truncated data
	if err == nil && len(data) > MaxPayloadSize {
		t.Errorf("MaxBytesReader allowed reading %d bytes, max is %d", len(data), MaxPayloadSize)
	}
}
