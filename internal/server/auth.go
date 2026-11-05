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

	"github.com/backendArchitect/go-boilerplate/internal/api"
	"github.com/backendArchitect/go-boilerplate/internal/db"
)

var (
	errEmailTaken         = errorf(api.ErrorCodeEmailTaken, "email is already registered")
	errInvalidCredentials = errorf(api.ErrorCodeInvalidCredentials, "invalid email or password")
)

// normalizeProfile trims the name and lowercases the email, then validates both.
func normalizeProfile(name, email string) (string, string, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" {
		return "", "", errorf(api.ErrorCodeValidationFailed, "name is required")
	}
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		return "", "", errorf(api.ErrorCodeValidationFailed, "a valid email is required")
	}
	return name, email, nil
}

func (s *Server) Register(ctx context.Context, req api.RegisterRequestObject) (api.RegisterResponseObject, error) {
	name, email, err := normalizeProfile(req.Body.Name, req.Body.Email)
	if err != nil {
		return nil, err
	}
	// bcrypt only uses the first 72 bytes, so longer passwords are rejected.
	if n := len(req.Body.Password); n < 8 || n > 72 {
		return nil, errorf(api.ErrorCodeValidationFailed, "password must be 8 to 72 bytes")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user, err := s.q.CreateUser(ctx, db.CreateUserParams{Name: name, Email: email, PasswordHash: string(hash)})
	if db.IsUniqueViolation(err) {
		return nil, errEmailTaken
	}
	if err != nil {
		return nil, err
	}
	res, err := s.authResponse(user)
	if err != nil {
		return nil, err
	}
	return api.Register201JSONResponse(res), nil
}

// dummyHash is compared against when the email is unknown, so a login
// attempt takes the same time whether or not the account exists.
var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	return h
})

func (s *Server) Login(ctx context.Context, req api.LoginRequestObject) (api.LoginResponseObject, error) {
	user, err := s.q.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Body.Email)))
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(req.Body.Password))
		return nil, errInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Body.Password)) != nil {
		return nil, errInvalidCredentials
	}
	res, err := s.authResponse(user)
	if err != nil {
		return nil, err
	}
	return api.Login200JSONResponse(res), nil
}

func (s *Server) authResponse(user db.User) (api.AuthResponse, error) {
	token, err := s.issueToken(user.ID)
	return api.AuthResponse{Token: token, User: toUser(user)}, err
}

func (s *Server) issueToken(userID int64) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTTTL)),
	}).SignedString([]byte(s.cfg.JWTSecret))
}

func (s *Server) parseToken(raw string) (int64, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return []byte(s.cfg.JWTSecret), nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(claims.Subject, 10, 64)
}

type userIDKey struct{}

// userID returns the authenticated user's ID. Only valid in operations that
// require bearerAuth in openapi.yaml.
func userID(ctx context.Context) int64 { return ctx.Value(userIDKey{}).(int64) }

// authenticate requires "Authorization: Bearer <jwt>" on operations that
// openapi.yaml secures with bearerAuth; the generated router marks those with
// api.BearerAuthScopes. Operations with `security: []` pass straight through.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(api.BearerAuthScopes) == nil {
			next.ServeHTTP(w, r)
			return
		}
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			s.writeError(w, r, errorf(api.ErrorCodeUnauthorized, "missing bearer token"))
			return
		}
		id, err := s.parseToken(raw)
		if err != nil {
			s.writeError(w, r, errorf(api.ErrorCodeUnauthorized, "invalid or expired token"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
	})
}
