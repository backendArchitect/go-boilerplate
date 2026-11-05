package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"

	"golang.org/x/crypto/bcrypt"

	"github.com/backendArchitect/go-boilerplate/internal/config"
	"github.com/backendArchitect/go-boilerplate/internal/db"
	"github.com/backendArchitect/go-boilerplate/internal/server"
)

const usage = `Usage: go-boilerplate <command> [args]

Commands:
  api                  Start the HTTP server
  migrate [command]    Run database migrations (default: up)
                       Commands: up, down, status, redo, reset, version
  seed [count]         Insert demo users (default: 10), password "password"
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 || !slices.Contains([]string{"api", "migrate", "seed"}, args[0]) {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid config:\n%w", err)
	}

	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	log := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	if cfg.LogFormat == "text" {
		log = slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	slog.SetDefault(log)

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch args[0] {
	case "api":
		return server.New(cfg, pool, log).Run(ctx)
	case "migrate":
		command := "up"
		if len(args) > 1 {
			command = args[1]
		}
		return db.Migrate(ctx, pool, command)
	default: // "seed"
		n := 10
		if len(args) > 1 {
			if n, err = strconv.Atoi(args[1]); err != nil || n < 1 {
				return errors.New("seed count must be a positive integer")
			}
		}
		return seed(ctx, db.New(pool), n)
	}
}

// seed inserts demo users user1@example.com … userN@example.com. Users that
// already exist are skipped, so it is safe to run repeatedly.
func seed(ctx context.Context, q *db.Queries, n int) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	for i := 1; i <= n; i++ {
		_, err := q.CreateUser(ctx, db.CreateUserParams{
			Name:         fmt.Sprintf("User %d", i),
			Email:        fmt.Sprintf("user%d@example.com", i),
			PasswordHash: string(hash),
		})
		if err != nil && !db.IsUniqueViolation(err) {
			return fmt.Errorf("seed user %d: %w", i, err)
		}
	}
	slog.Info("seeded users", "count", n)
	return nil
}
