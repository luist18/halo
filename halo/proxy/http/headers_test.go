package http

import (
	"net/http/httptest"
	"testing"
)

func TestParseHeadersDefaultsPoolOptInTrue(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set(ConnectionStringHeader, "postgres://user:pass@localhost/db")

	headers := parseHeaders(req)
	if !headers.PoolOptIn {
		t.Fatalf("expected PoolOptIn to default to true")
	}
}
