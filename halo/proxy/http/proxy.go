package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/luist18/halo/httpexecutor"
	"github.com/luist18/halo/internal/connstr"
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
	http.HandleFunc(p.Endpoint, p.handleSQL)

	addr := fmt.Sprintf(":%d", p.Port)
	slog.Info("starting HTTP proxy", slog.Int("port", p.Port), slog.String("endpoint", p.Endpoint))

	return http.ListenAndServe(addr, nil)
}

// handleSQL handles SQL execution requests
func (p *HttpProxy) handleSQL(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		headers := parseHeaders(r)
		slog.Info("received request", slog.String("remote-addr", r.RemoteAddr), slog.Any("headers", headers))

		// Validate connection string
		connStrValue := headers.ConnectionString.Unwrap()
		if connStrValue == "" {
			http.Error(w, "missing connection string", http.StatusBadRequest)
			return
		}

		connConfig, err := connstr.Parse(connStrValue)
		if err != nil {
			slog.Error("invalid connection string", slog.String("error", err.Error()))
			http.Error(w, "invalid connection string: "+err.Error(), http.StatusBadRequest)
			return
		}

		if err := connConfig.Validate(); err != nil {
			slog.Error("invalid connection configuration", slog.String("error", err.Error()))
			http.Error(w, "invalid connection configuration: "+err.Error(), http.StatusBadRequest)
			return
		}

		// TODO(PER-3): max payload

		payload, err := readPayload(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		slog.Info("parsed payload", slog.String("query", payload.Query), slog.Int("num-queries", len(payload.Queries)))

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
			slog.Error("failed to execute query", slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		for key, value := range result.GetHeaders() {
			w.Header().Set(key, value)
		}

		if err := json.NewEncoder(w).Encode(result.ToResponse()); err != nil {
			slog.Error("failed to encode response", slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	default:
		// Only allow POST requests
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
}
