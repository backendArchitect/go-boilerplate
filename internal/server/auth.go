package server

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/backendArchitect/go-boilerplate/internal/db"
)

// profile is the user-editable part of a user, shared by register and update.
type profile struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (p *profile) normalize() error {
	p.Name = strings.TrimSpace(p.Name)
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	if p.Name == "" {
		return errorf(http.StatusUnprocessableEntity, "name is required")
	}
	if a, err := mail.ParseAddress(p.Email); err != nil || a.Address != p.Email {
		return errorf(http.StatusUnprocessableEntity, "a valid email is required")
	}
	return nil
}

type authResponse struct {
	Token string  `json:"token"`
	User  db.User `json:"user"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		profile
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	if err := req.normalize(); err != nil {
		return err
	}
	// bcrypt only uses the first 72 bytes, so longer passwords are rejected.
	if n := len(req.Password); n < 8 || n > 72 {
		return errorf(http.StatusUnprocessableEntity, "password must be 8 to 72 bytes")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user, err := s.q.CreateUser(r.Context(), db.CreateUserParams{Name: req.Name, Email: req.Email, PasswordHash: string(hash)})
	if db.IsUniqueViolation(err) {
		return errorf(http.StatusConflict, "email is already registered")
	}
	if err != nil {
		return err
	}
	return s.respondWithToken(w, http.StatusCreated, user)
}

// dummyHash is compared against when the email is unknown, so a login
// attempt takes the same time whether or not the account exists.
var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	return h
})

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}

	user, err := s.q.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(req.Password))
		return errorf(http.StatusUnauthorized, "invalid email or password")
	}
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return errorf(http.StatusUnauthorized, "invalid email or password")
	}
	return s.respondWithToken(w, http.StatusOK, user)
}

func (s *Server) respondWithToken(w http.ResponseWriter, status int, user db.User) error {
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(user.ID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTTTL)),
	}).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return err
	}
	writeJSON(w, status, authResponse{Token: token, User: user})
	return nil
}

type userIDKey struct{}

// userID returns the authenticated user's ID. Only valid behind requireAuth.
func userID(r *http.Request) int64 { return r.Context().Value(userIDKey{}).(int64) }

// requireAuth rejects requests without a valid "Authorization: Bearer <jwt>".
func (s *Server) requireAuth(h func(http.ResponseWriter, *http.Request) error) http.Handler {
	return s.handle(func(w http.ResponseWriter, r *http.Request) error {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			return errorf(http.StatusUnauthorized, "missing bearer token")
		}
		var claims jwt.RegisteredClaims
		_, err := jwt.ParseWithClaims(raw, &claims,
			func(*jwt.Token) (any, error) { return []byte(s.cfg.JWTSecret), nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			return errorf(http.StatusUnauthorized, "invalid or expired token")
		}
		id, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			return errorf(http.StatusUnauthorized, "invalid or expired token")
		}
		return h(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
	})
}
