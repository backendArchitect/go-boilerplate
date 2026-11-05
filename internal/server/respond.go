package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/backendArchitect/go-boilerplate/internal/api"
)

// apiError is an error the client may see. Its code is one of the ErrorCode
// values in openapi.yaml, which clients branch on.
type apiError struct {
	code api.ErrorCode
	msg  string
}

func (e *apiError) Error() string { return e.msg }

func errorf(code api.ErrorCode, format string, args ...any) error {
	return &apiError{code: code, msg: fmt.Sprintf(format, args...)}
}

// statusOf gives every error code its single HTTP status. Add new codes to
// the ErrorCode enum in openapi.yaml first, then here.
var statusOf = map[api.ErrorCode]int{
	api.ErrorCodeInvalidRequest:     http.StatusBadRequest,
	api.ErrorCodeValidationFailed:   http.StatusUnprocessableEntity,
	api.ErrorCodeUnauthorized:       http.StatusUnauthorized,
	api.ErrorCodeInvalidCredentials: http.StatusUnauthorized,
	api.ErrorCodeNotFound:           http.StatusNotFound,
	api.ErrorCodeEmailTaken:         http.StatusConflict,
	api.ErrorCodeRateLimited:        http.StatusTooManyRequests,
	api.ErrorCodeUnavailable:        http.StatusServiceUnavailable,
	api.ErrorCodeInternalError:      http.StatusInternalServerError,
}

// writeError sends err as {"error": ..., "code": ...}. An *apiError is sent
// as is, pgx.ErrNoRows becomes not_found, and anything else is logged and
// hidden behind internal_error so internals never leak.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	e, ok := errors.AsType[*apiError](err)
	switch {
	case ok:
	case errors.Is(err, pgx.ErrNoRows):
		e = &apiError{api.ErrorCodeNotFound, "not found"}
	default:
		s.log.ErrorContext(r.Context(), "request failed", "err", err, "request_id", requestID(r.Context()))
		e = &apiError{api.ErrorCodeInternalError, "internal server error"}
	}
	status, ok := statusOf[e.code]
	if !ok {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, api.Error{Code: e.code, Error: e.msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
