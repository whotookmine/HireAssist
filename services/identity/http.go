// HTTP helpers shared by every handler.
package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// writeJSON sends v as an indented JSON body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		slog.Error("encode response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(append(body, '\n'))
}

// errorResponse is the body of every error: {"error": "...", "details": [...]}.
type errorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

// writeError sends {"error": msg}. A cause, when given, is logged but never sent to the client,
// because it can contain details of the database that a caller has no business seeing.
func writeError(w http.ResponseWriter, status int, msg string, cause error) {
	if cause != nil {
		slog.Error(msg, "error", cause)
	}
	writeJSON(w, status, errorResponse{Error: msg})
}

// writeInvalid sends a 400 that lists every validation problem at once.
func writeInvalid(w http.ResponseWriter, problems []string) {
	writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request", Details: problems})
}

// readJSON decodes the request body into dst. It answers 400 itself and returns false when the
// body is empty, is not JSON, has a field dst does not have, or holds more than one JSON value.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB is far more than any request here needs
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // a misspelt field is an error, not something silently ignored

	err := dec.Decode(dst)
	switch {
	case errors.Is(err, io.EOF):
		writeError(w, http.StatusBadRequest, "request body is empty; send a JSON object", nil)
		return false
	case err != nil:
		writeError(w, http.StatusBadRequest, "malformed JSON body: "+strings.TrimPrefix(err.Error(), "json: "), nil)
		return false
	case dec.More():
		writeError(w, http.StatusBadRequest, "request body must hold a single JSON object", nil)
		return false
	}
	return true
}

// logRequests writes one log line per request: method, path, status and duration.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
		)
	})
}

// statusRecorder remembers the status code a handler writes.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
