package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/luist18/halo/httpexecutor"
	"github.com/luist18/halo/internal/connstr"
	internalerrors "github.com/luist18/halo/proxy/http/internal/errors"
)

type contextKey string

const (
	// Context keys
	RequestUUIDContextKey      contextKey = "request-uuid"
	RequestStartTimeContextKey contextKey = "request-start-time"

	ConnectionStringHeader    = "Neon-Connection-String"
	PoolOptInHeader           = "Neon-Pool-Opt-In"
	BatchIsolationLevelHeader = "Neon-Batch-Isolation-Level"
	BatchReadOnlyHeader       = "Neon-Batch-Read-Only"
	BatchDeferrableHeader     = "Neon-Batch-Deferrable"

	// Payload size limits
	MaxPayloadSize  = 10 * 1024 * 1024 // 10MB
	MaxBatchQueries = 1024
	MaxQueryLength  = 100 * 1024 // 100KB
)

// HttpProxy represents an HTTP proxy server for PostgreSQL connections
type HttpProxy struct {
	Port     int
	Endpoint string
}

// NewHttpProxy creates a new HttpProxy instance with the specified port and endpoint
func NewHttpProxy(port int, endpoint string) *HttpProxy {
	return &HttpProxy{
		Port:     port,
		Endpoint: endpoint,
	}
}

// Start starts the HTTP proxy server and blocks until the server stops
func (p *HttpProxy) Start() error {
	http.HandleFunc(p.Endpoint, withRequestLifecycle(p.handleSQL))

	addr := fmt.Sprintf(":%d", p.Port)
	slog.Info("starting HTTP proxy", slog.Int("port", p.Port), slog.String("endpoint", p.Endpoint))

	return http.ListenAndServe(addr, nil)
}

// handleSQL handles SQL execution requests
func (p *HttpProxy) handleSQL(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodPost:
		if r.ContentLength > MaxPayloadSize {
			return internalerrors.NewRequestEntityTooLargeErr(
				fmt.Sprintf("payload too large: max size is %d bytes", MaxPayloadSize),
				slog.Int64("content-length", r.ContentLength),
				slog.Int("max", MaxPayloadSize))
		}

		// Overrides the request body reader to limit the size of the payload
		r.Body = http.MaxBytesReader(w, r.Body, MaxPayloadSize)

		headers := parseHeaders(r)

		connStrValue := headers.ConnectionString.Unwrap()
		if connStrValue == "" {
			return internalerrors.NewInvalidInputErr("missing connection string")
		}

		connConfig, err := connstr.Parse(connStrValue)
		if err != nil {
			return internalerrors.WrapWithInvalidInputErr(err, "invalid connection string")
		}

		if err := connConfig.Validate(); err != nil {
			return internalerrors.WrapWithInvalidInputErr(err, "invalid connection configuration")
		}

		payload, err := readPayload(r)
		if err != nil {
			return internalerrors.WrapWithInvalidInputErr(err, "invalid payload")
		}

		opts := httpexecutor.Options{
			PoolOptIn:           headers.PoolOptIn,
			BatchIsolationLevel: headers.BatchIsolationLevel,
			BatchReadOnly:       headers.BatchReadOnly,
			BatchDeferrable:     headers.BatchDeferrable,
		}

		execPayload := httpexecutor.Payload{
			Query:   payload.Query,
			Params:  payload.Params,
			Queries: payload.Queries,
		}

		result, err := httpexecutor.Execute(r.Context(), *headers.ConnectionString, execPayload, opts)
		if err != nil {
			return err
		}

		w.Header().Set("Content-Type", "application/json")
		for key, value := range result.GetHeaders() {
			w.Header().Set(key, value)
		}

		if err := json.NewEncoder(w).Encode(result.ToResponse()); err != nil {
			return internalerrors.WrapWithInternalErr(err, "failed to encode response")
		}

		return nil
	default:
		// Only allow POST requests
		return internalerrors.NewMethodNotAllowedErr("method not allowed")
	}
}
