.PHONY: setup setup-go setup-bun setup-py build deploy deploy-go deploy-bun deploy-py test test-go test-bun test-py benchmark lint lint-go lint-bun lint-py docker-test docker-test-go docker-test-bun docker-test-py docker-compose-up docker-compose-down docker-compose-logs smoke smoke-go smoke-bun smoke-py clean help

# Default target
.DEFAULT_GOAL := help

## -----------------------------------------------------------------------
## Setup Commands
## -----------------------------------------------------------------------

setup: setup-go setup-bun setup-py ## Setup all projects

setup-go: ## Setup Golang Gin project
	@echo "Setting up Golang Gin..."
	cd golang-gin && go mod init api-quest-go 2>/dev/null || true
	cd golang-gin && go get github.com/gin-gonic/gin github.com/google/uuid
	cd golang-gin && go mod tidy

setup-bun: ## Setup Bun ElysiaJS project
	@echo "Setting up Bun ElysiaJS..."
	cd bun-elysia && bun install

setup-py: ## Setup Python FastAPI project with uv
	@echo "Setting up Python FastAPI with uv..."
	cd python-fastapi && uv venv .venv 2>/dev/null || true
	cd python-fastapi && uv pip install -r requirements.txt

## -----------------------------------------------------------------------
## Build Commands
## -----------------------------------------------------------------------

build: ## Build all Docker images
	@echo "Building all Docker images..."
	docker build -t api-quest-go ./golang-gin
	docker build -t api-quest-bun ./bun-elysia
	docker build -t api-quest-py ./python-fastapi

## -----------------------------------------------------------------------
## Deploy Commands
## -----------------------------------------------------------------------

deploy: deploy-go deploy-bun deploy-py ## Deploy all services to Fly.io

deploy-go: ## Deploy Golang Gin to Fly.io
	@echo "Deploying Golang Gin to Fly.io..."
	cd golang-gin && flyctl deploy

deploy-bun: ## Deploy Bun ElysiaJS to Fly.io
	@echo "Deploying Bun ElysiaJS to Fly.io..."
	cd bun-elysia && flyctl deploy

deploy-py: ## Deploy Python FastAPI to Fly.io
	@echo "Deploying Python FastAPI to Fly.io..."
	cd python-fastapi && flyctl deploy

## -----------------------------------------------------------------------
## Test Commands
## -----------------------------------------------------------------------

test: test-go test-bun test-py ## Run all unit and E2E tests

test-go: ## Run Go unit tests
	@echo "Running Go tests..."
	cd golang-gin && go test -v ./...

test-bun: ## Run Bun unit tests
	@echo "Running Bun tests..."
	cd bun-elysia && bun test

test-py: ## Run Python unit tests
	@echo "Running Python tests..."
	cd python-fastapi && uv run pytest test_main.py -v

## -----------------------------------------------------------------------
## Smoke Tests (Server Running)
## -----------------------------------------------------------------------

smoke: smoke-go smoke-bun smoke-py ## Run smoke tests against all running servers

smoke-go: ## Smoke test Go server (port 3082)
	@echo "Testing Golang Gin (http://localhost:3082)..."
	@curl -s http://localhost:3082/ping && echo " - Go OK" || echo " - Go NOT running"

smoke-bun: ## Smoke test Bun server (port 3082)
	@echo "Testing Bun ElysiaJS (http://localhost:3082)..."
	@curl -s http://localhost:3082/ping && echo " - Bun OK" || echo " - Bun NOT running"

smoke-py: ## Smoke test Python server (port 3083)
	@echo "Testing Python FastAPI (http://localhost:3083)..."
	@curl -s http://localhost:3083/ping && echo " - Python OK" || echo " - Python NOT running"

## -----------------------------------------------------------------------
## Docker Compose Commands
## -----------------------------------------------------------------------

docker-compose-up: ## Start all services with Docker Compose
	@echo "Starting all services with Docker Compose..."
	docker compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@echo "Services started. Use 'make smoke' to verify."

docker-compose-down: ## Stop all services with Docker Compose
	@echo "Stopping all services..."
	docker compose down

docker-compose-logs: ## View logs from all Docker Compose services
	docker compose logs -f

docker-compose-ps: ## List Docker Compose services
	docker compose ps

docker-compose-restart: ## Restart all Docker Compose services
	docker compose restart

benchmark: ## Run k6 benchmark (requires BASE_URL env var)
	@if [ -z "$(BASE_URL)" ]; then \
		echo "Error: BASE_URL environment variable not set"; \
		echo "Usage: make benchmark BASE_URL=https://your-app.fly.dev"; \
		exit 1; \
	fi
	@echo "Running benchmark against $(BASE_URL)..."
	k6 run -e BASE_URL=$(BASE_URL) benchmark.js

## -----------------------------------------------------------------------
## Utility Commands
## -----------------------------------------------------------------------

clean: ## Clean build artifacts
	@echo "Cleaning..."
	cd golang-gin && go clean
	cd bun-elysia && rm -rf node_modules bun.lock
	cd python-fastapi && rm -rf .venv __pycache__

## -----------------------------------------------------------------------
## Lint & Type Check Commands
## -----------------------------------------------------------------------

lint: lint-go lint-bun lint-py ## Run all linters and type checkers

lint-go: ## Format and lint Go code
	@echo "Linting Golang Gin..."
	cd golang-gin && gofmt -l .
	cd golang-gin && go vet ./...

lint-bun: ## Type check Bun/TypeScript code
	@echo "Linting Bun ElysiaJS..."
	cd bun-elysia && bun build --check index.ts

lint-py: ## Type check Python code
	@echo "Linting Python FastAPI..."
	cd python-fastapi && python3 -m py_compile main.py
	cd python-fastapi && .venv/bin/python -c "import main; print('Python imports: OK')"

## -----------------------------------------------------------------------
## Docker Test Commands
## -----------------------------------------------------------------------

docker-test: docker-test-go docker-test-bun docker-test-py ## Test all Docker builds

docker-test-go: ## Test Go Docker build
	@echo "Testing Go Docker build..."
	cd golang-gin && docker build -t api-quest-go-test .
	@echo "Go Docker image built successfully"
	@docker images api-quest-go-test

docker-test-bun: ## Test Bun Docker build
	@echo "Testing Bun Docker build..."
	cd bun-elysia && docker build -t api-quest-bun-test .
	@echo "Bun Docker image built successfully"
	@docker images api-quest-bun-test

docker-test-py: ## Test Python Docker build
	@echo "Testing Python Docker build..."
	cd python-fastapi && docker build -t api-quest-py-test .
	@echo "Python Docker image built successfully"
	@docker images api-quest-py-test

docker-clean: ## Remove test Docker images
	@echo "Cleaning test Docker images..."
	-docker rmi api-quest-go-test api-quest-bun-test api-quest-py-test 2>/dev/null || true

logs-go: ## View logs for Golang app
	cd golang-gin && flyctl logs

logs-bun: ## View logs for Bun app
	cd bun-elysia && flyctl logs

logs-py: ## View logs for Python app
	cd python-fastapi && flyctl logs

## -----------------------------------------------------------------------
## Help
## -----------------------------------------------------------------------

help: ## Show this help message
	@echo "API Quest - Makefile Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
