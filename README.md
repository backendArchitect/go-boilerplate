# go-boilerplate

A small, production-ready starting point for a Go JSON API. It uses the standard library wherever it can and has only five direct dependencies.

| Concern       | Choice                                                                 |
| ------------- | ---------------------------------------------------------------------- |
| HTTP          | `net/http`: Go's built-in router (`GET /users/{id}`), no framework     |
| Database      | PostgreSQL via [pgx](https://github.com/jackc/pgx) connection pool     |
| Queries       | [sqlc](https://sqlc.dev): write SQL, get type-safe Go                  |
| Migrations    | [goose](https://github.com/pressly/goose), embedded in the binary      |
| Auth          | bcrypt password hashes + JWT (HS256)                                   |
| Logging       | `log/slog`: JSON in production, text in development                    |
| Config        | Environment variables, optional `.env` file                            |
| Tooling       | Makefile, live reload (air), golangci-lint, GitHub Actions, Dependabot |
| Deploy        | Multi-stage Dockerfile → ~25 MB distroless non-root image              |

Also included: graceful shutdown, server timeouts, request IDs, access logs, panic recovery, CORS, `/healthz` + `/readyz` probes, and an integration test that runs against a real Postgres.

## Quick start

Requirements: Go 1.26+ and Docker.

```sh
cp .env.example .env
make db        # start Postgres in Docker
make migrate   # create tables
make seed      # optional: user1..user10@example.com, password "password"
make dev       # run with live reload on :8080
```

Or run everything in containers with `docker compose up --build`.

```sh
curl -X POST localhost:8080/api/v1/auth/register \
  -d '{"name":"Ada","email":"ada@example.com","password":"correct horse"}'
# → {"token":"eyJ...","user":{...}}

curl localhost:8080/api/v1/users/me -H "Authorization: Bearer eyJ..."
```

## API

| Method | Path                    | Auth | Description                     |
| ------ | ----------------------- | ---- | ------------------------------- |
| GET    | `/healthz`              |      | Liveness (no dependencies)      |
| GET    | `/readyz`               |      | Readiness (pings the database)  |
| POST   | `/api/v1/auth/register` |      | Create account, returns a token |
| POST   | `/api/v1/auth/login`    |      | Returns a token                 |
| GET    | `/api/v1/users`         | ✓    | List (`?limit=20&offset=0`)     |
| GET    | `/api/v1/users/{id}`    | ✓    | Get one                         |
| GET    | `/api/v1/users/me`      | ✓    | Current user                    |
| PUT    | `/api/v1/users/me`      | ✓    | Update name and email           |
| DELETE | `/api/v1/users/me`      | ✓    | Delete account                  |

Errors are always `{"error": "message"}` with a matching status code.

## Commands

The binary has three subcommands: `api`, `migrate [up|down|status|redo|reset|version]` and `seed [count]`. Run `make` to list the Makefile targets:

```
db         Start Postgres in Docker
run        Run the API
dev        Run the API with live reload
build      Build the binary into bin/app
test       Run tests (integration tests need `make db`)
lint       Run golangci-lint
generate   Regenerate Go code from SQL (sqlc)
migrate    Apply pending migrations
migration  Create a migration: make migration name=add_posts
seed       Insert demo users
docker     Build the Docker image
```

The dev tools (sqlc, air, golangci-lint) run at pinned versions through `go run`, so you don't need to install them.

## Configuration

| Variable               | Default | Notes                                                         |
| ---------------------- | ------- | ------------------------------------------------------------- |
| `DATABASE_URL`         | —       | Required. Pool options go in the URL, e.g. `pool_max_conns=20` |
| `JWT_SECRET`           | —       | Required, ≥ 32 chars. `openssl rand -hex 32`                  |
| `JWT_TTL`              | `24h`   | Token lifetime (Go duration)                                  |
| `HTTP_ADDR`            | `:8080` |                                                               |
| `LOG_LEVEL`            | `info`  | `debug`, `info`, `warn`, `error`                              |
| `LOG_FORMAT`           | `json`  | `json` or `text`                                              |
| `CORS_ALLOWED_ORIGINS` | empty   | Comma-separated. Empty disables CORS; `*` allows any          |

All problems are reported at startup in one go, rather than failing one variable at a time.

## Layout

```
main.go                    command dispatch (api, migrate, seed)
internal/config/           env → Config
internal/db/
  migrations/*.sql         goose migrations (embedded into the binary)
  queries/*.sql            SQL queries; sqlc generates the Go next to them
  postgres.go              Connect, Migrate, IsUniqueViolation (hand-written)
internal/server/
  server.go                routes, http.Server, graceful shutdown
  middleware.go            request ID + logging, panic recovery, CORS
  respond.go               JSON helpers and error → status mapping
  auth.go                  register, login, JWT middleware
  users.go                 user handlers
```

Handlers return an `error`. Use `errorf(status, msg)` for errors the client should see. `pgx.ErrNoRows` becomes a 404, and anything else is logged and returned as a generic 500, so internal details never leak.

## Adding a resource

Here's how to add posts as an example:

1. `make migration name=create_posts`, then write the `CREATE TABLE` in the new file.
2. Add `internal/db/queries/posts.sql` with `-- name: CreatePost :one` style queries.
3. `make generate`, which gives you `db.Post`, `q.CreatePost(...)` and friends.
4. Write handlers in `internal/server/posts.go` and register the routes in `server.go`.
5. `make migrate`, then extend the test.

When handlers start to carry real business logic, pull it into its own package (e.g. `internal/posts`). Wait until there is logic to move.

## Deploying

Build the image with `docker build .` and run it with `migrate up` once per deploy, before rolling out `api`. That means a Kubernetes Job or init container, or the `migrate` service in `compose.yaml`. Point liveness probes at `/healthz` and readiness probes at `/readyz`. The server drains in-flight requests for up to 15 s on `SIGTERM`.
