// Package server is the HTTP API: routing, middleware and handlers.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/backendArchitect/go-boilerplate/internal/config"
	"github.com/backendArchitect/go-boilerplate/internal/db"
)

type Server struct {
	cfg  config.Config
	pool *pgxpool.Pool
	q    *db.Queries
	log  *slog.Logger
}

func New(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{cfg: cfg, pool: pool, q: db.New(pool), log: log}
}

// Handler returns the fully wired HTTP handler. Routes use Go's built-in
// pattern syntax: "METHOD /path/{param}".
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("GET /readyz", s.handle(s.ready))

	mux.Handle("POST /api/v1/auth/register", s.handle(s.register))
	mux.Handle("POST /api/v1/auth/login", s.handle(s.login))

	mux.Handle("GET /api/v1/users", s.requireAuth(s.listUsers))
	mux.Handle("GET /api/v1/users/{id}", s.requireAuth(s.getUser))
	mux.Handle("GET /api/v1/users/me", s.requireAuth(s.getMe))
	mux.Handle("PUT /api/v1/users/me", s.requireAuth(s.updateMe))
	mux.Handle("DELETE /api/v1/users/me", s.requireAuth(s.deleteMe))

	// Outermost first: every request gets an ID and a log line, panics become 500s.
	return s.logRequests(s.recoverPanics(s.cors(mux)))
}

// Run serves HTTP until ctx is cancelled, then drains in-flight requests.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.HTTPAddr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(s.log.Handler(), slog.LevelError),
	}

	errc := make(chan error, 1)
	go func() {
		s.log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	s.log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// ready reports whether the app can serve traffic (database reachable).
// Keep /healthz dependency-free so a database blip doesn't restart pods.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) error {
	if err := s.pool.Ping(r.Context()); err != nil {
		s.log.WarnContext(r.Context(), "readiness check failed", "err", err)
		return errorf(http.StatusServiceUnavailable, "database unavailable")
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
