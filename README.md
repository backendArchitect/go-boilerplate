# go-boilerplate

A small, production-ready starting point for a Go JSON API. It uses the standard library wherever it can, and it's set up so coding agents can work on it safely.

| Concern      | Choice                                                                          |
| ------------ | ------------------------------------------------------------------------------- |
| HTTP         | `net/http` with Go's built-in router, no framework                              |
| API contract | [OpenAPI 3](internal/api/openapi.yaml) → [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) typed handlers |
| Database     | PostgreSQL via [pgx](https://github.com/jackc/pgx) connection pool              |
| Queries      | [sqlc](https://sqlc.dev): write SQL, get type-safe Go                           |
| Migrations   | [goose](https://github.com/pressly/goose), embedded in the binary               |
| Auth         | bcrypt password hashes + JWT (HS256)                                            |
| Logging      | `log/slog`: JSON in production, text in development                             |
| Tooling      | `make check`, live reload (air), golangci-lint, GitHub Actions, Dependabot      |
| Agents       | [AGENTS.md](AGENTS.md) house rules (and `CLAUDE.md`, which imports it)          |
| Deploy       | Multi-stage Dockerfile → ~25 MB distroless non-root image                       |

Also included: per-IP rate limiting, stable error codes, graceful shutdown, server timeouts, request IDs, access logs, panic recovery, CORS, `/healthz` + `/readyz` probes, and an integration test that runs against a real Postgres.

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

Before committing, run `make check`. It does everything CI does.

## API

The contract is [`internal/api/openapi.yaml`](internal/api/openapi.yaml), and the running server serves it at `GET /openapi.yaml`. Point client generators, API tools or AI agents at it.

| Method | Path                    | Auth | Description                     |
| ------ | ----------------------- | ---- | ------------------------------- |
| POST   | `/api/v1/auth/register` |      | Create account, returns a token |
| POST   | `/api/v1/auth/login`    |      | Returns a token                 |
| GET    | `/api/v1/users`         | ✓    | List (`?limit=20&offset=0`)     |
| GET    | `/api/v1/users/{id}`    | ✓    | Get one                         |
| GET    | `/api/v1/users/me`      | ✓    | Current user                    |
| PUT    | `/api/v1/users/me`      | ✓    | Update name and email           |
| DELETE | `/api/v1/users/me`      | ✓    | Delete account                  |

`/healthz` (liveness) and `/readyz` (pings the database) sit outside the API.

Every error looks like `{"error": "email is already registered", "code": "email_taken"}`. Clients should branch on `code`, which is stable and always maps to the same status. The message may change. The codes are `invalid_request` 400, `validation_failed` 422, `unauthorized` 401, `invalid_credentials` 401, `not_found` 404, `email_taken` 409, `rate_limited` 429, `unavailable` 503 and `internal_error` 500.

## Commands

The binary has three subcommands: `api`, `migrate [up|down|status|redo|reset|version]` and `seed [count]`. Run `make` to list the Makefile targets:

```
check      Run before committing: generate, tidy, lint, test (CI runs exactly this)
db         Start Postgres in Docker
run        Run the API
dev        Run the API with live reload
build      Build the binary into bin/app
test       Run tests (integration tests need `make db`)
lint       Run golangci-lint
generate   Regenerate Go code from SQL (sqlc) and openapi.yaml (oapi-codegen)
migrate    Apply pending migrations
migration  Create a migration: make migration name=add_posts
seed       Insert demo users
docker     Build the Docker image
```

The dev tools (sqlc, oapi-codegen, air, golangci-lint) run at pinned versions through `go run`, so you don't need to install them.

## Configuration

| Variable               | Default | Notes                                                          |
| ---------------------- | ------- | -------------------------------------------------------------- |
| `DATABASE_URL`         | —       | Required. Pool options go in the URL, e.g. `pool_max_conns=20` |
| `JWT_SECRET`           | —       | Required, ≥ 32 chars. `openssl rand -hex 32`                   |
| `JWT_TTL`              | `24h`   | Token lifetime (Go duration)                                   |
| `HTTP_ADDR`            | `:8080` |                                                                |
| `LOG_LEVEL`            | `info`  | `debug`, `info`, `warn`, `error`                               |
| `LOG_FORMAT`           | `json`  | `json` or `text`                                               |
| `CORS_ALLOWED_ORIGINS` | empty   | Comma-separated. Empty disables CORS; `*` allows any           |
| `RATE_LIMIT_RPS`       | `20`    | Requests/second per client IP on every route; `0` disables     |
| `AUTH_RATE_LIMIT_RPM`  | `10`    | Requests/minute per client IP on `/api/v1/auth/*`; `0` disables |
| `CLIENT_IP_HEADER`     | empty   | Set behind a proxy; see [Deploying](#deploying)                |

All problems are reported at startup in one go, rather than failing one variable at a time.

## Layout

```
main.go                    command dispatch (api, migrate, seed)
internal/api/
  openapi.yaml             the API contract (source of truth)
  api.gen.go               generated: types, routing, typed handler interface
internal/config/           env → Config
internal/db/
  migrations/*.sql         goose migrations (embedded into the binary)
  queries/*.sql            SQL queries; sqlc generates the Go next to them
  postgres.go              Connect, Migrate, IsUniqueViolation (hand-written)
internal/server/
  server.go                wiring, http.Server, graceful shutdown
  middleware.go            request ID + logging, panic recovery, CORS, body limit
  ratelimit.go             per-IP token buckets
  respond.go               error codes → statuses, JSON helpers
  auth.go                  register, login, JWT middleware
  users.go                 user handlers
```

`*server.Server` implements the generated `api.StrictServerInterface`, so the build fails if an operation in the spec has no handler. Generated code decodes path, query and body parameters, and handlers return typed responses or an error. Use `errorf(api.ErrorCodeX, msg)` for errors the client should see. `pgx.ErrNoRows` becomes `not_found`, and anything else is logged and returned as `internal_error`, so internal details never leak. Whether an operation needs a token is declared in the spec with `security`, not in Go.

## Adding a resource

Here's how to add posts as an example:

1. `make migration name=create_posts`, then write the `CREATE TABLE`.
2. Add queries to `internal/db/queries/posts.sql` (`-- name: CreatePost :one`).
3. Add the paths and schemas to `internal/api/openapi.yaml`.
4. `make generate`. The build now fails until you implement the new methods.
5. Implement them in `internal/server/posts.go`.
6. Extend `TestAPI`, then run `make check`.

When handlers start to carry real business logic, pull it into its own package (e.g. `internal/posts`). Wait until there is logic to move.

## Deploying

Build the image with `docker build .` and run it with `migrate up` once per deploy, before rolling out `api`. That means a Kubernetes Job or init container, or the `migrate` service in `compose.yaml`. Point liveness probes at `/healthz` and readiness probes at `/readyz`. The server drains in-flight requests for up to 15 s on `SIGTERM`.

Behind a load balancer, every request arrives from the proxy's IP, so all clients would share one rate-limit bucket. Set `CLIENT_IP_HEADER` to the header your proxy sets to the real client IP, such as `X-Real-IP`, `CF-Connecting-IP` or `Fly-Client-IP`. If you use `X-Forwarded-For`, the last entry is used, which assumes exactly one proxy hop. Rate limits are per process, so with several replicas each one allows the full limit. Enforce limits at the load balancer if you need them to be global.
