.PHONY: all build test lint clean run-api run-worker run-cli setup docker-up docker-down migrate help

# Build variables
BINARY_DIR := bin
API_BINARY := $(BINARY_DIR)/api
WORKER_BINARY := $(BINARY_DIR)/worker
CLI_BINARY := $(BINARY_DIR)/qtest
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Go variables
GOPATH := $(shell go env GOPATH)
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Default target
all: lint test build

# Build all binaries
build: build-api build-worker build-cli

build-api:
	@echo "Building API server..."
	@mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(API_BINARY) ./cmd/api

build-worker:
	@echo "Building worker..."
	@mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(WORKER_BINARY) ./cmd/worker

build-cli:
	@echo "Building CLI..."
	@mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(CLI_BINARY) ./cmd/cli

# Run services
run-api: build-api
	@echo "Starting API server..."
	$(API_BINARY)

run-worker: build-worker
	@echo "Starting worker..."
	$(WORKER_BINARY)

run-cli:
	@go run ./cmd/cli $(ARGS)

# Development
dev-api:
	@go run ./cmd/api

dev-worker:
	@WORKER_TYPE=all go run ./cmd/worker

# Testing
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

test-short:
	@echo "Running short tests..."
	go test -v -short ./...

test-coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Linting
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet only..."; \
		go vet ./...; \
	fi

lint-fix:
	@echo "Fixing lint issues..."
	golangci-lint run --fix ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download

tidy:
	@echo "Tidying modules..."
	go mod tidy

# Database
migrate-up:
	@echo "Running migrations..."
	@if [ -f ./bin/migrate ]; then \
		./bin/migrate -path ./migrations -database "$(DATABASE_URL)" up; \
	else \
		echo "Install migrate: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; \
	fi

migrate-down:
	@echo "Rolling back migration..."
	./bin/migrate -path ./migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@echo "Creating migration..."
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	./bin/migrate create -ext sql -dir ./migrations -seq $(NAME)

# Docker
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d

docker-down:
	@echo "Stopping Docker services..."
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-build:
	@echo "Building Docker images..."
	docker build -t qtest/api:$(VERSION) -f Dockerfile.api .
	docker build -t qtest/worker:$(VERSION) -f Dockerfile.worker .

# Setup
setup: deps
	@echo "Setting up development environment..."
	@mkdir -p $(BINARY_DIR)
	@echo "Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Setup complete!"

# Clean
clean:
	@echo "Cleaning..."
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# SQLC
sqlc-generate:
	@echo "Generating SQL code..."
	sqlc generate

# Help
help:
	@echo "QTest Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all            Run lint, test, and build (default)"
	@echo "  build          Build all binaries"
	@echo "  build-api      Build API server"
	@echo "  build-worker   Build worker"
	@echo "  build-cli      Build CLI"
	@echo "  run-api        Run API server"
	@echo "  run-worker     Run worker"
	@echo "  run-cli        Run CLI (use ARGS='...' for arguments)"
	@echo "  dev-api        Run API in development mode"
	@echo "  dev-worker     Run worker in development mode"
	@echo "  test           Run all tests with coverage"
	@echo "  test-short     Run short tests only"
	@echo "  test-coverage  Generate HTML coverage report"
	@echo "  lint           Run linters"
	@echo "  lint-fix       Fix lint issues"
	@echo "  fmt            Format code"
	@echo "  deps           Download dependencies"
	@echo "  tidy           Tidy modules"
	@echo "  migrate-up     Run database migrations"
	@echo "  migrate-down   Rollback last migration"
	@echo "  migrate-create Create new migration (NAME=...)"
	@echo "  docker-up      Start Docker services"
	@echo "  docker-down    Stop Docker services"
	@echo "  docker-logs    Follow Docker logs"
	@echo "  docker-build   Build Docker images"
	@echo "  setup          Setup development environment"
	@echo "  clean          Clean build artifacts"
	@echo "  sqlc-generate  Generate SQL code"
	@echo "  help           Show this help"
