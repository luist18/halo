package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	internalerrors "github.com/luist18/halo/proxy/http/internal/errors"
)

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware type for handlers that can return errors
type handlerFunc func(http.ResponseWriter, *http.Request) error

// withRequestLifecycle manages the complete request lifecycle including
// request ID generation, response writer wrapping, error handling, and logging
func withRequestLifecycle(next handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		requestID, err := uuid.NewV7()
		if err != nil {
			slog.Error("failed to generate request UUID", slog.String("error", err.Error()))
			requestID = uuid.Nil
		}
		reqID := requestID.String()

		ctx := context.WithValue(r.Context(), RequestUUIDContextKey, reqID)
		ctx = context.WithValue(ctx, RequestStartTimeContextKey, startTime)
		r = r.WithContext(ctx)

		slog.Info("received request",
			slog.String("remote-addr", r.RemoteAddr),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int64("content-length", r.ContentLength),
			slog.String("request-uuid", reqID))

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		handlerErr := next(rw, r)

		duration := time.Since(startTime)

		if handlerErr != nil {
			if handleErr := handleError(rw, r, handlerErr); handleErr != nil {
				slog.Error("failed to handle error",
					slog.String("handle-error", handleErr.Error()),
					slog.String("original-error", handlerErr.Error()),
					slog.String("request-uuid", reqID))
			}
		}

		slog.Info("finished handling request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.statusCode),
			slog.String("duration", formatDuration(duration)),
			slog.String("request-uuid", reqID))
	}
}

// formatDuration formats a duration with an adaptive order of magnitude
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000.0)
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	default:
		return fmt.Sprintf("%.3fs", d.Seconds())
	}
}

// handleError converts an error into an appropriate HTTP response.
// It handles different error types, including PostgreSQL errors
// and custom proxy errors.
func handleError(w http.ResponseWriter, r *http.Request, err error) error {
	requestID, _ := r.Context().Value(RequestUUIDContextKey).(string)

	// Log the error details
	slog.Error("finished handling request with error",
		slog.String("error", err.Error()),
		slog.String("request-uuid", requestID))

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
			return fmt.Errorf("failed to encode pg error response: %w", encodeErr)
		}
		return nil
	}

	var proxyErr internalerrors.ProxyError
	if errors.As(err, &proxyErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(proxyErr.StatusCode())
		if encodeErr := json.NewEncoder(w).Encode(errorResponse{
			Message: proxyErr.Error(),
			Error:   buildErrorChain(err),
		}); encodeErr != nil {
			return fmt.Errorf("failed to encode proxy error response: %w", encodeErr)
		}
		return nil
	}

	slog.Error("unhandled error", slog.String("error", err.Error()))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	if encodeErr := json.NewEncoder(w).Encode(errorResponse{
		Message: "Internal server error",
		Error:   buildErrorChain(err),
	}); encodeErr != nil {
		return fmt.Errorf("failed to encode internal error response: %w", encodeErr)
	}

	return nil
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
