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

	"github.com/backendArchitect/go-boilerplate/internal/api"
	"github.com/backendArchitect/go-boilerplate/internal/config"
	"github.com/backendArchitect/go-boilerplate/internal/db"
)

// TestAPI runs the main user journey against a real Postgres (make check).
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

	cfg := config.Config{JWTSecret: strings.Repeat("s", 32), JWTTTL: time.Hour} // rate limits off
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
	fail := func(method, path, token string, body any, wantStatus int, wantCode api.ErrorCode) {
		t.Helper()
		if code := call(method, path, token, body, wantStatus)["code"]; code != string(wantCode) {
			t.Fatalf("%s %s: code %v, want %s", method, path, code, wantCode)
		}
	}

	alice := map[string]string{"name": "Alice", "email": "Alice@Example.com", "password": "correct horse"}

	// Register and log in.
	reg := call("POST", "/api/v1/auth/register", "", alice, http.StatusCreated)
	token, _ := reg["token"].(string)
	if email := reg["user"].(map[string]any)["email"]; email != "alice@example.com" {
		t.Fatalf("email not normalized: %v", email)
	}
	fail("POST", "/api/v1/auth/register", "", alice, http.StatusConflict, api.ErrorCodeEmailTaken)
	fail("POST", "/api/v1/auth/register", "", map[string]string{"name": "B", "email": "nope", "password": "12345678"}, http.StatusUnprocessableEntity, api.ErrorCodeValidationFailed)
	fail("POST", "/api/v1/auth/register", "", map[string]string{"name": "B", "email": "b@example.com", "password": "short"}, http.StatusUnprocessableEntity, api.ErrorCodeValidationFailed)
	fail("POST", "/api/v1/auth/register", "", "not an object", http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	fail("POST", "/api/v1/auth/login", "", map[string]string{"email": "alice@example.com", "password": "wrong password"}, http.StatusUnauthorized, api.ErrorCodeInvalidCredentials)
	fail("POST", "/api/v1/auth/login", "", map[string]string{"email": "ghost@example.com", "password": "whatever1"}, http.StatusUnauthorized, api.ErrorCodeInvalidCredentials)
	call("POST", "/api/v1/auth/login", "", map[string]string{"email": "alice@example.com", "password": "correct horse"}, http.StatusOK)

	// Protected routes reject missing, forged and expired tokens.
	forged, _ := (&Server{cfg: config.Config{JWTSecret: strings.Repeat("x", 32), JWTTTL: time.Hour}}).issueToken(1)
	expired, _ := (&Server{cfg: config.Config{JWTSecret: cfg.JWTSecret, JWTTTL: -time.Minute}}).issueToken(1)
	for _, tok := range []string{"", forged, expired} {
		fail("GET", "/api/v1/users/me", tok, nil, http.StatusUnauthorized, api.ErrorCodeUnauthorized)
	}

	// Authenticated CRUD.
	if me := call("GET", "/api/v1/users/me", token, nil, http.StatusOK); me["name"] != "Alice" {
		t.Fatalf("GET /users/me = %v", me)
	}
	call("GET", "/api/v1/users/1", token, nil, http.StatusOK)
	fail("GET", "/api/v1/users/999", token, nil, http.StatusNotFound, api.ErrorCodeNotFound)
	fail("GET", "/api/v1/users/abc", token, nil, http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	fail("GET", "/api/v1/users?limit=500", token, nil, http.StatusBadRequest, api.ErrorCodeInvalidRequest)
	if list := call("GET", "/api/v1/users", token, nil, http.StatusOK); len(list["users"].([]any)) != 1 {
		t.Fatalf("GET /users = %v", list)
	}
	if upd := call("PUT", "/api/v1/users/me", token, map[string]string{"name": "Alice B", "email": "alice@example.com"}, http.StatusOK); upd["name"] != "Alice B" {
		t.Fatalf("PUT /users/me = %v", upd)
	}
	call("DELETE", "/api/v1/users/me", token, nil, http.StatusNoContent)
	fail("GET", "/api/v1/users/me", token, nil, http.StatusNotFound, api.ErrorCodeNotFound)

	call("GET", "/readyz", "", nil, http.StatusOK)
	call("GET", "/openapi.yaml", "", nil, http.StatusOK)
}

// TestRateLimit checks the per-IP limits, including that forging the
// client-controlled part of X-Forwarded-For doesn't buy a fresh bucket.
func TestRateLimit(t *testing.T) {
	cfg := config.Config{RateLimitRPS: 1000, AuthRateLimitRPM: 2, ClientIPHeader: "X-Forwarded-For"}
	h := New(cfg, nil, slog.New(slog.DiscardHandler)).rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	hit := func(path, xff string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("POST %s (XFF %q): status %d, want %d", path, xff, rec.Code, want)
		}
		return rec
	}

	hit("/api/v1/auth/login", "1.1.1.1, 203.0.113.9", http.StatusNoContent)
	hit("/api/v1/auth/login", "2.2.2.2, 203.0.113.9", http.StatusNoContent)
	rec := hit("/api/v1/auth/login", "3.3.3.3, 203.0.113.9", http.StatusTooManyRequests)
	if got := rec.Header().Get("Retry-After"); got != "30" {
		t.Fatalf("Retry-After = %q, want 30", got)
	}
	if !strings.Contains(rec.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("body = %s", rec.Body)
	}
	hit("/api/v1/users", "203.0.113.9", http.StatusNoContent)       // the auth limit is scoped to /auth
	hit("/api/v1/auth/login", "203.0.113.10", http.StatusNoContent) // other clients are unaffected
}
