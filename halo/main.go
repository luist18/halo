package main

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"

	"github.com/luist18/halo/httpexecutor"
	"github.com/luist18/halo/internal/connstr"
)

func main() {
	http.HandleFunc("/sql", func(w http.ResponseWriter, r *http.Request) {
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

			result, err := httpexecutor.Execute(r.Context(), *headers.ConnectionString, httpexecutor.Payload{
				Query:   payload.Query,
				Params:  payload.Params,
				Queries: payload.Queries,
			},
				opts,
			)
			if err != nil {
				slog.Error("failed to execute query", slog.String("error", err.Error()))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")

			// Format response based on whether it's a batch or single query
			var response interface{}
			if result.IsBatch {
				// Add transaction configuration headers for batch mode
				if opts.BatchReadOnly {
					w.Header().Set(BatchReadOnlyHeader, "true")
				}
				if opts.BatchDeferrable {
					w.Header().Set(BatchDeferrableHeader, "true")
				}
				if opts.BatchIsolationLevel != "" {
					w.Header().Set(BatchIsolationLevelHeader, opts.BatchIsolationLevel)
				}

				response = map[string]interface{}{
					"results": result.Results,
				}
			} else {
				// For single query, return the response directly
				response = result.Results[0]
			}

			if err := json.NewEncoder(w).Encode(response); err != nil {
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
	})

	// TODO: use port as a config
	slog.Info("starting server", slog.Int("port", 8080))

	log.Fatal(http.ListenAndServe(":8080", nil))
}
