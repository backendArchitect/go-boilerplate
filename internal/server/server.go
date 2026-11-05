// Package server implements the API described in internal/api/openapi.yaml.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"github.com/backendArchitect/go-boilerplate/internal/api"
	"github.com/backendArchitect/go-boilerplate/internal/config"
	"github.com/backendArchitect/go-boilerplate/internal/db"
)

// The compiler rejects the build if an operation in openapi.yaml has no handler.
var _ api.StrictServerInterface = (*Server)(nil)

type Server struct {
	cfg         config.Config
	pool        *pgxpool.Pool
	q           *db.Queries
	log         *slog.Logger
	globalLimit *ipLimiter
	authLimit   *ipLimiter
}

func New(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{
		cfg:         cfg,
		pool:        pool,
		q:           db.New(pool),
		log:         log,
		globalLimit: newIPLimiter(rate.Limit(cfg.RateLimitRPS), 2*cfg.RateLimitRPS),
		authLimit:   newIPLimiter(rate.Limit(cfg.AuthRateLimitRPM)/60, cfg.AuthRateLimitRPM),
	}
}

// Handler returns the fully wired HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(api.Spec)
	})

	// Every operation in openapi.yaml, under /api/v1. Generated code decodes
	// params and bodies; anything malformed becomes 400 invalid_request.
	badRequest := func(w http.ResponseWriter, r *http.Request, err error) {
		s.writeError(w, r, errorf(api.ErrorCodeInvalidRequest, "%v", err))
	}
	api.HandlerWithOptions(
		api.NewStrictHandlerWithOptions(s, nil, api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  badRequest,
			ResponseErrorHandlerFunc: s.writeError,
		}),
		api.StdHTTPServerOptions{
			BaseURL:          "/api/v1",
			BaseRouter:       mux,
			Middlewares:      []api.MiddlewareFunc{s.authenticate},
			ErrorHandlerFunc: badRequest,
		},
	)

	// Outermost first: every request gets an ID and a log line, panics become
	// 500s, and CORS headers are set before rate limiting so browsers can read 429s.
	return s.logRequests(s.recoverPanics(s.cors(s.rateLimit(limitBody(mux)))))
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
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		s.log.WarnContext(r.Context(), "readiness check failed", "err", err)
		s.writeError(w, r, errorf(api.ErrorCodeUnavailable, "database unavailable"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
