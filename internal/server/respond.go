package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// httpError is an error with a status code and a message safe to show clients.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func errorf(status int, format string, args ...any) error {
	return &httpError{status: status, msg: fmt.Sprintf(format, args...)}
}

// handle adapts a handler that returns an error. Errors become JSON responses:
// *httpError keeps its status, pgx.ErrNoRows becomes 404, anything else is
// logged and returned as an opaque 500.
func (s *Server) handle(h func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		he, ok := errors.AsType[*httpError](err)
		switch {
		case ok:
		case errors.Is(err, pgx.ErrNoRows):
			he = &httpError{http.StatusNotFound, "not found"}
		default:
			s.log.ErrorContext(r.Context(), "request failed", "err", err, "request_id", requestID(r.Context()))
			he = &httpError{http.StatusInternalServerError, "internal server error"}
		}
		writeJSON(w, he.status, map[string]string{"error": he.msg})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeJSON reads a JSON body of at most 1 MB and rejects unknown fields.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errorf(http.StatusBadRequest, "invalid JSON body: %v", err)
	}
	return nil
}

// queryInt parses an optional integer query parameter in [0, maxVal].
func queryInt(r *http.Request, key string, fallback, maxVal int) (int, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > maxVal {
		return 0, errorf(http.StatusBadRequest, "%s must be an integer between 0 and %d", key, maxVal)
	}
	return n, nil
}
