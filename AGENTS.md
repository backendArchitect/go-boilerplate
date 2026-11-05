# AGENTS.md

A Go JSON API: `net/http`, PostgreSQL (pgx + sqlc), goose migrations, and an OpenAPI contract served by oapi-codegen's strict server. README.md has the full tour.

## Done means `make check` passes

`make check` starts Postgres in Docker, regenerates code, runs `go mod tidy`, lints, scans for bugs with gospect and runs the tests. CI runs the same command and fails if it changed any files, so commit everything it produces. Don't hand back work while it fails. If gospect reports a finding, fix the code; don't add an ignore.

## Generated code: edit the source, then run `make generate`

| Edit                                                     | Generated                                   |
| -------------------------------------------------------- | ------------------------------------------- |
| `internal/api/openapi.yaml`                              | `internal/api/api.gen.go`                   |
| `internal/db/queries/*.sql`, `internal/db/migrations/*.sql` | `internal/db/*.go` (except `postgres.go`) |

Never edit generated files by hand. Your changes will be overwritten.

## Conventions

- **Contract first.** API changes start in `openapi.yaml`. After `make generate`, the build fails until `*server.Server` implements the new method (`var _ api.StrictServerInterface` in `server.go`).
- **Handlers** live in `internal/server/<resource>.go`. They return a typed response such as `api.GetUser200JSONResponse(...)`, or an error.
- **Errors.** For client-facing errors, return `errorf(api.ErrorCodeX, "message")`. `pgx.ErrNoRows` becomes `not_found`. Any other error is logged and returned as `internal_error`, so don't wrap internals into messages. To add a code, add it to the `ErrorCode` enum in `openapi.yaml`, then to `statusOf` in `respond.go`.
- **Auth is declared in the spec.** Operations need a bearer token by default; public ones set `security: []`. Inside a secured handler, `userID(ctx)` is the caller.
- **Return API types, never `db` types.** Convert with a function like `toUser`, because database rows can hold secrets such as `password_hash`.
- **Schema changes** go in a new migration (`make migration name=...`). Never edit a migration that is already merged.
- **Tests** run against a real Postgres. Extend `TestAPI` in `internal/server/server_test.go` for new endpoints, and don't mock the database.
- **Dependencies.** Prefer the standard library, and ask before adding a module.
- **Never log** request bodies, headers, passwords or tokens.

## Adding a resource (e.g. posts)

1. `make migration name=create_posts`, then write the SQL.
2. Add queries in `internal/db/queries/posts.sql`.
3. Add paths and schemas to `internal/api/openapi.yaml`.
4. Run `make generate`.
5. Implement the new methods in `internal/server/posts.go`.
6. Extend `TestAPI`, then run `make check`.
