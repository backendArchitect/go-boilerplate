package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/backendArchitect/go-boilerplate/internal/config"
	"github.com/backendArchitect/go-boilerplate/internal/db"
)

// TestAPI runs the main user journey against a real Postgres.
// Run with: make test (starts nothing; expects `docker compose up -d db`).
func TestAPI(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := t.Context()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users RESTART IDENTITY"); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{JWTSecret: strings.Repeat("s", 32), JWTTTL: time.Hour}
	srv := httptest.NewServer(New(cfg, pool, slog.New(slog.DiscardHandler)).Handler())
	t.Cleanup(srv.Close)

	call := func(method, path, token string, body any, wantStatus int) map[string]any {
		t.Helper()
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req, _ := http.NewRequestWithContext(ctx, method, srv.URL+path, &buf)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != wantStatus {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.StatusCode, wantStatus, raw)
		}
		if strings.Contains(string(raw), "$2a$") {
			t.Fatalf("%s %s: response leaks a password hash: %s", method, path, raw)
		}
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return out
	}

	alice := map[string]string{"name": "Alice", "email": "Alice@Example.com", "password": "correct horse"}

	// Register and log in.
	reg := call("POST", "/api/v1/auth/register", "", alice, http.StatusCreated)
	token, _ := reg["token"].(string)
	if email := reg["user"].(map[string]any)["email"]; email != "alice@example.com" {
		t.Fatalf("email not normalized: %v", email)
	}
	call("POST", "/api/v1/auth/register", "", alice, http.StatusConflict)
	call("POST", "/api/v1/auth/register", "", map[string]string{"name": "B", "email": "nope", "password": "12345678"}, http.StatusUnprocessableEntity)
	call("POST", "/api/v1/auth/register", "", map[string]string{"name": "B", "email": "b@example.com", "password": "short"}, http.StatusUnprocessableEntity)
	call("POST", "/api/v1/auth/login", "", map[string]string{"email": "alice@example.com", "password": "wrong password"}, http.StatusUnauthorized)
	call("POST", "/api/v1/auth/login", "", map[string]string{"email": "ghost@example.com", "password": "whatever1"}, http.StatusUnauthorized)
	call("POST", "/api/v1/auth/login", "", map[string]string{"email": "alice@example.com", "password": "correct horse"}, http.StatusOK)

	// Protected routes reject missing, forged and expired tokens.
	forged := &Server{cfg: config.Config{JWTSecret: strings.Repeat("x", 32), JWTTTL: time.Hour}}
	expired := &Server{cfg: config.Config{JWTSecret: cfg.JWTSecret, JWTTTL: -time.Minute}}
	for _, s := range []*Server{forged, expired} {
		rec := httptest.NewRecorder()
		_ = s.respondWithToken(rec, http.StatusOK, db.User{ID: 1})
		var out authResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		call("GET", "/api/v1/users/me", out.Token, nil, http.StatusUnauthorized)
	}
	call("GET", "/api/v1/users/me", "", nil, http.StatusUnauthorized)

	// Authenticated CRUD.
	me := call("GET", "/api/v1/users/me", token, nil, http.StatusOK)
	if me["name"] != "Alice" {
		t.Fatalf("GET /users/me = %v", me)
	}
	call("GET", "/api/v1/users/1", token, nil, http.StatusOK)
	call("GET", "/api/v1/users/999", token, nil, http.StatusNotFound)
	call("GET", "/api/v1/users/abc", token, nil, http.StatusBadRequest)
	call("GET", "/api/v1/users?limit=500", token, nil, http.StatusBadRequest)
	if list := call("GET", "/api/v1/users", token, nil, http.StatusOK); len(list["users"].([]any)) != 1 {
		t.Fatalf("GET /users = %v", list)
	}
	upd := call("PUT", "/api/v1/users/me", token, map[string]string{"name": "Alice B", "email": "alice@example.com"}, http.StatusOK)
	if upd["name"] != "Alice B" {
		t.Fatalf("PUT /users/me = %v", upd)
	}
	call("PUT", "/api/v1/users/me", token, map[string]string{"name": "Alice", "email": "alice@example.com", "role": "admin"}, http.StatusBadRequest)
	call("DELETE", "/api/v1/users/me", token, nil, http.StatusNoContent)
	call("GET", "/api/v1/users/me", token, nil, http.StatusNotFound)

	call("GET", "/readyz", "", nil, http.StatusOK)
}
