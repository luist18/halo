package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/luist18/halo/httpexecutor"
	"github.com/luist18/halo/internal/connstr"
	internalerrors "github.com/luist18/halo/proxy/http/internal/errors"
)

const (
	ConnectionStringHeader    = "Neon-Connection-String"
	RawTextOutputHeader       = "Neon-Raw-Text-Output"
	ArrayModeHeader           = "Neon-Array-Mode"
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
	http.HandleFunc(p.Endpoint, errorHandler(p.handleSQL))

	addr := fmt.Sprintf(":%d", p.Port)
	slog.Info("starting HTTP proxy", slog.Int("port", p.Port), slog.String("endpoint", p.Endpoint))

	return http.ListenAndServe(addr, nil)
}

// handleSQL handles SQL execution requests
func (p *HttpProxy) handleSQL(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodPost:
		slog.Info("received request", slog.String("remote-addr", r.RemoteAddr), slog.Int64("content-length", r.ContentLength))
		if r.ContentLength > MaxPayloadSize {
			return internalerrors.NewRequestEntityTooLargeErr(
				fmt.Sprintf("payload too large: max size is %d bytes", MaxPayloadSize),
				slog.Int64("content-length", r.ContentLength),
				slog.Int("max", MaxPayloadSize))
		}

		// Overrides the request body reader to limit the size of the payload
		r.Body = http.MaxBytesReader(w, r.Body, MaxPayloadSize)

		headers := parseHeaders(r)

		// Validate connection string
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
			RawTextOutput:       headers.RawTextOutput,
			ArrayMode:           headers.ArrayMode,
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
			slog.Error("failed to encode response", slog.String("error", err.Error()))
			return internalerrors.WrapWithInternalErr(err, "failed to encode response")
		}

		return nil
	default:
		// Only allow POST requests
		return internalerrors.NewMethodNotAllowedErr("method not allowed")
	}
}

func errorHandler(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			type errorResponse struct {
				Message string `json:"message"`
				Error   string `json:"error"`
			}

			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				var httpPgErr internalerrors.PGError
				httpPgErr.FromPgError(pgErr)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				if encodeErr := json.NewEncoder(w).Encode(httpPgErr); encodeErr != nil {
					slog.Error("failed to encode pg error response", slog.String("error", encodeErr.Error()))
				}
				return
			}

			var proxyErr internalerrors.ProxyError
			if errors.As(err, &proxyErr) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(proxyErr.StatusCode())
				json.NewEncoder(w).Encode(errorResponse{
					Message: proxyErr.Error(),
					Error:   buildErrorChain(err),
				})
				return
			}

			slog.Error("unhandled error", slog.String("error", err.Error()))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(errorResponse{
				Message: "Internal server error",
				Error:   buildErrorChain(err),
			})
		}
	}
}

// buildErrorChain constructs the full error message by traversing the error chain
func buildErrorChain(err error) string {
	if err == nil {
		return ""
	}

	var messages []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		messages = append(messages, e.Error())
	}

	if len(messages) == 0 {
		return ""
	}
	if len(messages) == 1 {
		return messages[0]
	}

	// Build the chain: "message1: message2: message3"
	result := messages[0]
	for i := 1; i < len(messages); i++ {
		result += ": " + messages[i]
	}
	return result
}
