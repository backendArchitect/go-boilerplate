# Dev tools run at pinned versions via `go run`, so there is nothing to install
# and go.mod stays free of tool dependencies. sqlc needs a C compiler (cgo).
SQLC = go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
AIR  = go run github.com/air-verse/air@v1.67.4
LINT = go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

TEST_DATABASE_URL ?= postgres://app:app@localhost:5432/app_test?sslmode=disable

.DEFAULT_GOAL := help
.PHONY: help db run dev build test lint generate migrate migration seed docker

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

db: ## Start Postgres in Docker
	docker compose up -d --wait db

run: ## Run the API
	go run . api

dev: ## Run the API with live reload
	$(AIR)

build: ## Build the binary into bin/app
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/app .

test: ## Run tests (integration tests need `make db`)
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -cover ./...

lint: ## Run golangci-lint
	$(LINT) run

generate: ## Regenerate Go code from SQL (sqlc)
	$(SQLC) generate

migrate: ## Apply pending migrations
	go run . migrate up

migration: ## Create a migration: make migration name=add_posts
	@test -n "$(name)" || { echo "usage: make migration name=<name>"; exit 1; }
	@f=internal/db/migrations/$$(date +%Y%m%d%H%M%S)_$(name).sql; \
		printf -- '-- +goose Up\n\n-- +goose Down\n' > $$f; echo "created $$f"

seed: ## Insert demo users
	go run . seed

docker: ## Build the Docker image
	docker build -t go-boilerplate .
