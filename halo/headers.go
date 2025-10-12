package main

import (
	"net/http"
	"strings"

	"github.com/luist18/halo/pkg"
)

const (
	ConnectionStringHeader    = "Neon-Connection-String"
	RawTextOutputHeader       = "Neon-Raw-Text-Output"
	ArrayModeHeader           = "Neon-Array-Mode"
	PoolOptInHeader           = "Neon-Pool-Opt-In"
	BatchIsolationLevelHeader = "Neon-Batch-Isolation-Level"
	BatchReadOnlyHeader       = "Neon-Batch-Read-Only"
	BatchDeferrableHeader     = "Neon-Batch-Deferrable"
)

type headers struct {
	ConnectionString    *pkg.Secret
	RawTextOutput       bool
	ArrayMode           bool
	PoolOptIn           bool
	BatchIsolationLevel string
	BatchReadOnly       bool
	BatchDeferrable     bool
}

func parseHeaders(r *http.Request) headers {
	return headers{
		ConnectionString:    pkg.NewSecret(strings.TrimSpace(r.Header.Get(ConnectionStringHeader))),
		RawTextOutput:       headerOrDefault(r, RawTextOutputHeader, "true", strings.TrimSpace, strings.ToLower) == "true",
		ArrayMode:           headerOrDefault(r, ArrayModeHeader, "true", strings.TrimSpace, strings.ToLower) == "true",
		PoolOptIn:           headerOrDefault(r, PoolOptInHeader, "true", strings.TrimSpace, strings.ToLower) == "true",
		BatchIsolationLevel: headerOrDefault(r, BatchIsolationLevelHeader, "ReadCommitted", strings.TrimSpace),
		BatchReadOnly:       headerOrDefault(r, BatchReadOnlyHeader, "true", strings.TrimSpace, strings.ToLower) == "true",
		BatchDeferrable:     headerOrDefault(r, BatchDeferrableHeader, "true", strings.TrimSpace, strings.ToLower) == "true",
	}
}

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
