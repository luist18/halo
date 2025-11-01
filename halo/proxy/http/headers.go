package http

import (
	"net/http"
	"strings"

	"github.com/luist18/halo/internal/data"
)

// headers represents HTTP headers sent by the client.
type headers struct {
	// ConnectionString holds the database connection string as a secret.
	// Use Unwrap() to get the actual value. This avoids leaking it in logs.
	ConnectionString *data.Secret

	// PoolOptIn indicates whether to reuse connections via a connection pool.
	PoolOptIn bool

	// BatchIsolationLevel sets the isolation level for batch transactions.
	BatchIsolationLevel string

	// BatchReadOnly indicates if the batch transaction is read-only.
	BatchReadOnly bool

	// BatchDeferrable indicates if the batch transaction is deferrable.
	BatchDeferrable bool
}

// parseHeaders reads HTTP headers and returns a headers struct.
//
// Default values are the following:
//
//	PoolOptIn: false TODO(PER-14): implement connection pooling to reuse database connections and change this to default true
//	BatchIsolationLevel: None -> will default to ReadCommitted
//	BatchReadOnly: false
//	BatchDeferrable: false
func parseHeaders(r *http.Request) headers {
	return headers{
		ConnectionString: data.NewSecret(strings.TrimSpace(r.Header.Get(ConnectionStringHeader))),
		// TODO(PER-14): implement connection pooling to reuse database connections and change this to default true
		PoolOptIn:           headerOrDefault(r, PoolOptInHeader, "false", strings.TrimSpace, strings.ToLower) == "true",
		BatchIsolationLevel: headerOrDefault(r, BatchIsolationLevelHeader, "", strings.TrimSpace),
		BatchReadOnly:       headerOrDefault(r, BatchReadOnlyHeader, "false", strings.TrimSpace, strings.ToLower) == "true",
		BatchDeferrable:     headerOrDefault(r, BatchDeferrableHeader, "false", strings.TrimSpace, strings.ToLower) == "true",
	}
}

// headerOrDefault returns the header value or a default if not set.
// Optional functions can be applied to transform the value (e.g., TrimSpace).
//
// Example: headerOrDefault(r, "Content-Type", "application/json", strings.TrimSpace, strings.ToLower)
// will return the value of the "Content-Type" header, trimmed and converted to
// lowercase, or "application/json" if the header is not set.
func headerOrDefault(r *http.Request, key string, def string, opts ...func(string) string) string {
	val := r.Header.Get(key)
	if val == "" {
		return def
	}

	for _, opt := range opts {
		val = opt(val)
	}

	return val
}
