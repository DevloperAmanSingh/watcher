BINARY      := watcher
BIN_DIR     := bin
PKG         := ./...
GOOSE       := go run github.com/pressly/goose/v3/cmd/goose@v3.24.1
GOLANGCILINT := go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

ifneq (,$(wildcard .env))
include .env
export
endif

.DEFAULT_GOAL := help

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F': ' '{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

## build: compile the binary into bin/
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) ./cmd

## run: start the monitoring engine
run:
	go run ./cmd guard

## test: run all tests
test:
	go test -race -count=1 $(PKG)

## test-short: skip tests that need the docker stack
test-short:
	go test -race -short -count=1 $(PKG)

## cover: run tests and open the coverage report
cover:
	go test -race -coverprofile=coverage.out $(PKG)
	go tool cover -html=coverage.out

## fmt: format all Go source
fmt:
	go fmt $(PKG)

## vet: run go vet
vet:
	go vet $(PKG)

## lint: run golangci-lint
lint:
	$(GOLANGCILINT) run

## tidy: sync go.mod and go.sum
tidy:
	go mod tidy

## up: start Postgres, Redis and mailpit, waiting until healthy
up:
	docker compose up -d --wait

## down: stop the stack, keeping volumes
down:
	docker compose down

## reset: stop the stack and delete all data
reset:
	docker compose down -v

## logs: tail the stack logs
logs:
	docker compose logs -f

## migrate: apply all pending migrations
migrate:
	$(GOOSE) -dir migrations postgres "$(GOOSE_DBSTRING)" up

## migrate-down: roll back the most recent migration
migrate-down:
	$(GOOSE) -dir migrations postgres "$(GOOSE_DBSTRING)" down

## migrate-status: show migration state
migrate-status:
	$(GOOSE) -dir migrations postgres "$(GOOSE_DBSTRING)" status

## dev: bring up the stack and apply migrations
dev: up migrate

## check: everything CI runs
check: fmt vet lint test

## clean: remove build and coverage artifacts
clean:
	rm -rf $(BIN_DIR) coverage.out

.PHONY: help build run test test-short cover fmt vet lint tidy up down reset \
        logs migrate migrate-down migrate-status dev check clean
