# pilam — developer entrypoints.
#
# Go tools (templ, sqlc, goose, golangci-lint, air, gotestsum) are pinned in
# go.mod and run via `go tool`. The two non-Go tools — the Tailwind standalone
# binary and the templUI CLI — are fetched into ./bin by `make tools`.

SHELL := /bin/bash
.DEFAULT_GOAL := help

TAILWIND_VERSION := v4.3.3
TEMPLUI_VERSION  := v1.13.0

BIN      := bin

# Overridable: the dev container bakes the musl build in at /usr/local/bin.
TAILWIND ?= $(BIN)/tailwindcss
TEMPLUI  := $(BIN)/templui

CSS_IN  := internal/http/static/css/input.css
CSS_OUT := internal/http/static/css/app.css

# Tailwind ships one static binary per platform; pick the right asset.
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_S),Darwin)
  TAILWIND_OS := macos
else
  TAILWIND_OS := linux
endif
ifeq ($(UNAME_M),aarch64)
  TAILWIND_ARCH := arm64
else ifeq ($(UNAME_M),arm64)
  TAILWIND_ARCH := arm64
else
  TAILWIND_ARCH := x64
endif

# The musl build is the one that runs on Alpine (our dev container).
TAILWIND_LIBC :=
ifeq ($(TAILWIND_OS),linux)
  ifneq ($(wildcard /etc/alpine-release),)
    TAILWIND_LIBC := -musl
  endif
endif
TAILWIND_ASSET := tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)$(TAILWIND_LIBC)
TAILWIND_URL   := https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/$(TAILWIND_ASSET)

.PHONY: help
help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## ---------------------------------------------------------------- toolchain

.PHONY: tools
tools: $(TAILWIND) $(TEMPLUI) ## Fetch the non-Go tools into ./bin

$(TAILWIND):
	@mkdir -p $(BIN)
	@echo ">> fetching $(TAILWIND_ASSET) $(TAILWIND_VERSION)"
	@# Download to a temp name and rename, so an interrupted fetch does not
	@# leave a truncated binary that make then considers up to date. It is a
	@# ~100MB binary on a release CDN; -C - resumes rather than restarting.
	@curl -C - -sSfL -o $@.part $(TAILWIND_URL)
	@chmod +x $@.part
	@mv $@.part $@

$(TEMPLUI):
	@mkdir -p $(BIN)
	@echo ">> installing templui $(TEMPLUI_VERSION)"
	@GOBIN=$(CURDIR)/$(BIN) go install github.com/templui/templui/cmd/templui@$(TEMPLUI_VERSION)

## ---------------------------------------------------------------- codegen

.PHONY: generate
generate: ## Run all code generators (templ, sqlc)
	go tool templ generate
	go tool sqlc generate

.PHONY: css
css: $(TAILWIND) ## Compile Tailwind into the embedded stylesheet
	$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --minify

## ---------------------------------------------------------------- build & run

.PHONY: build
build: generate css ## Build the pilam binary into ./bin
	go build -o $(BIN)/pilam ./cmd/pilam

.PHONY: run
run: generate css ## Run the server on the host
	go run ./cmd/pilam serve

.PHONY: dev
dev: css ## Run the server with live reload (used inside the dev container)
	@# compose waits for the database to be healthy before starting this, so
	@# `make up` on a fresh volume brings the schema with it. Migrations are
	@# additive and applying them is idempotent, so a restart is a no-op.
	go run ./cmd/pilam migrate up
	@# Two watchers: Tailwind rewrites app.css (unminified, for readable
	@# devtools), air rebuilds and restarts the server. app.css is embedded, so
	@# a stylesheet change also triggers a Go rebuild — that is intended.
	@$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --watch=always & \
	  TW=$$!; trap "kill $$TW" EXIT INT TERM; \
	  go tool air

## ---------------------------------------------------------------- quality

GOTEST := go tool gotestsum --format dots-v2 --format-hide-empty-pkg --

.PHONY: test
test: generate css ## Run the test suite
	$(GOTEST) ./...

.PHONY: lint
lint: generate css ## Run golangci-lint
	go tool golangci-lint run

.PHONY: check
check: generate css ## Everything CI would run: tidy, vet, test, lint
	@# Read-only: prints the diff and fails rather than rewriting go.mod. A
	@# stale go.sum still builds against a warm module cache, so drift is
	@# invisible locally until it breaks a fresh clone.
	go mod tidy -diff
	go vet ./...
	$(GOTEST) ./...
	go tool golangci-lint run

.PHONY: fmt
fmt: ## Format Go and templ sources
	go fmt ./...
	go tool templ fmt .

## ---------------------------------------------------------------- database

.PHONY: migrate
migrate: ## Apply pending database migrations
	go run ./cmd/pilam migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	go run ./cmd/pilam migrate down

.PHONY: migrate-status
migrate-status: ## Show which migrations are applied
	go run ./cmd/pilam migrate status

.PHONY: migration
migration: ## Scaffold a migration: make migration name=add_styles
	@test -n "$(name)" || { echo "usage: make migration name=add_styles" >&2; exit 1; }
	@# -s numbers sequentially rather than by timestamp. With a
	@# linear history the ordering is easier to read and the file names shorter.
	go tool goose -dir db/migrations -s create $(name) sql

.PHONY: seed
seed: ## Load the demo dataset
	go run ./cmd/pilam seed

## ---------------------------------------------------------------- containers

.env:
	@cp .env.example .env
	@echo ">> wrote .env from .env.example"

.PHONY: up
up: .env ## Bring up Postgres and the app with live reload
	docker compose up --build -d
	@echo ">> http://localhost:$$(grep -E '^HTTP_PORT=' .env | cut -d= -f2)"

.PHONY: down
down: ## Stop the stack (keeps volumes)
	docker compose down

.PHONY: logs
logs: ## Follow the app logs
	docker compose logs -f app

.PHONY: clean
clean: ## Remove build output and generated files (keeps ./bin)
	rm -rf tmp $(CSS_OUT) $(BIN)/pilam
	find . -name '*_templ.go' -delete
	find . -name '*_sqlc.go' -delete

.PHONY: distclean
distclean: clean ## Also remove the fetched tools — they are a large download
	rm -rf $(BIN)
