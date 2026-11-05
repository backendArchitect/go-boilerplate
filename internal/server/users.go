package server

import (
	"math"
	"net/http"
	"strconv"

	"github.com/backendArchitect/go-boilerplate/internal/db"
)

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	limit, err := queryInt(r, "limit", 20, 100)
	if err != nil {
		return err
	}
	offset, err := queryInt(r, "offset", 0, math.MaxInt32)
	if err != nil {
		return err
	}
	users, err := s.q.ListUsers(r.Context(), db.ListUsersParams{Limit: int32(limit), Offset: int32(offset)}) //nolint:gosec // bounded by queryInt
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
	return nil
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errorf(http.StatusBadRequest, "invalid user id")
	}
	user, err := s.q.GetUser(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, user)
	return nil
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) error {
	user, err := s.q.GetUser(r.Context(), userID(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, user)
	return nil
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) error {
	var req profile
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	if err := req.normalize(); err != nil {
		return err
	}
	user, err := s.q.UpdateUser(r.Context(), db.UpdateUserParams{ID: userID(r), Name: req.Name, Email: req.Email})
	if db.IsUniqueViolation(err) {
		return errorf(http.StatusConflict, "email is already registered")
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, user)
	return nil
}

func (s *Server) deleteMe(w http.ResponseWriter, r *http.Request) error {
	n, err := s.q.DeleteUser(r.Context(), userID(r))
	if err != nil {
		return err
	}
	if n == 0 {
		return errorf(http.StatusNotFound, "not found")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
