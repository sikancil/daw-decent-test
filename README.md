# API Quest - 3-Stack REST API Challenge

A high-performance REST API implementation across three different tech stacks, completing all 8 levels of the API Quest backend challenge.

## The Challenge

Complete all 8 levels of progressively complex API endpoints, with Level 8 being a concurrent speed run benchmark.

| Level | Endpoint(s) | Description |
|-------|-------------|-------------|
| 1 | GET /ping | Returns "pong" |
| 2 | POST /echo | Echoes JSON body |
| 3 | POST /books, GET /books, GET /books/:id | CRUD create/read |
| 4 | PUT /books/:id, DELETE /books/:id | CRUD update/delete |
| 5 | POST /auth/token | Dynamic auth guard (protects GET endpoints) |
| 6 | GET /books?author=X, GET /books?page=1&limit=2 | Search & pagination |
| 7 | Error handling | 400, 404 responses |
| 8 | All endpoints | Concurrent operations benchmark |

## Tech Stacks

1. **Golang + Gin** (Primary) - Fastest compiled runtime with Gin framework
2. **Bun + ElysiaJS** (Challenger 1) - V8-powered JavaScript runtime with ultra-fast routing
3. **Python + FastAPI** (Challenger 2) - Robust validation via Pydantic with uvloop

## Project Structure

```
api-quest/
├── golang-gin/             # Go 1.22 + Gin
│   ├── main.go
│   ├── main_test.go        # Unit tests
│   ├── e2e_test.go         # E2E tests
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile
│   └── fly.toml
├── bun-elysia/             # Bun + ElysiaJS
│   ├── index.ts
│   ├── index.test.ts       # Tests
│   ├── package.json
│   ├── bun.lockb
│   ├── Dockerfile
│   └── fly.toml
├── python-fastapi/         # Python 3.11 + FastAPI
│   ├── main.py
│   ├── test_main.py        # Tests
│   ├── requirements.txt
│   ├── Dockerfile
│   └── fly.toml
├── benchmark.js            # k6 load testing script
├── Makefile                # Orchestration
├── docker-compose.yml      # Local development
└── README.md
```

## Quick Start

### Prerequisites

- Go 1.22+
- Bun 1.0+
- Python 3.11+ with uv
- Docker (optional)
- Fly.io CLI (`flyctl`)
- k6 (for benchmarking)

### Setup

```bash
# Install dependencies for all stacks
make setup

# Or setup individually:
make setup-go    # Golang
make setup-bun   # Bun
make setup-py    # Python
```

### Local Testing

```bash
# Test Go (port 3082 by default)
cd golang-gin && go run main.go
curl http://localhost:3082/ping

# Test Bun (port 3082 by default)
cd bun-elysia && bun run index.ts
curl http://localhost:3082/ping

# Test Python (port 3000 by default)
cd python-fastapi && uv run uvicorn main:app
curl http://localhost:3000/ping
```

### Running Tests

```bash
# Run all tests
make test

# Run individual stack tests
make test-go    # Go: 18 tests (11 E2E + 7 unit)
make test-bun   # Bun: 14 E2E tests
make test-py    # Python: 16 unit tests
```

### Deployment to Fly.io

1. **Update `fly.toml` files** with unique app names:
   - `golang-gin/fly.toml`: Change `api-quest-go-yourname`
   - `bun-elysia/fly.toml`: Change `api-quest-bun-yourname`
   - `python-fastapi/fly.toml`: Change `api-quest-py-yourname`

2. **Deploy all services:**
   ```bash
   make deploy
   ```

3. **Or deploy individually:**
   ```bash
   make deploy-go
   make deploy-bun
   make deploy-py
   ```

4. **Copy your public URLs** and test at [API Quest](https://apiquest.dev/)

### Benchmark

```bash
# Install k6 first (brew install k6)

# Start all services locally:
docker-compose-up

# Benchmark each service:
make benchmark BASE_URL=http://localhost:3000  # Gin
make benchmark BASE_URL=http://localhost:3001  # Bun
make benchmark BASE_URL=http://localhost:3002  # Python
```

## Benchmark Results (Local Machine)

**Configuration:** 50 concurrent VUs, 10 second duration

| Stack | Throughput | p(95) Latency | p(99) Latency | Check Success |
|-------|-----------|---------------|---------------|---------------|
| **Golang Gin** | 32,842 req/s | 3.41ms | 5.96ms | 100% ✅ |
| **Bun Elysia** | 14,353 req/s | 6.36ms | 11.95ms | 100% ✅ |
| **Python FastAPI** | 715 req/s | 109.49ms | 144.29ms | 100% ✅ |

**Winner:** Golang Gin - 2.3x faster than Bun, 46x faster than Python

## Test Coverage

| Stack | Unit Tests | E2E Tests | Total | Status |
|-------|-----------|-----------|-------|--------|
| **Gin** | 7 | 11 | 18 | ✅ All Passing |
| **Bun** | 0 | 14 | 14 | ✅ All Passing |
| **Python** | 16 | 0 | 16 | ✅ All Passing |
| **Total** | 23 | 25 | **48** | ✅ **100%** |

## Key Implementation Details

### Book Schema

```json
{
  "id": "uuid-string",
  "title": "string",
  "author": "string",
  "year": 2026
}
```

**Note:** `year` defaults to current year (2026) if not provided.

### Dynamic Auth Guard

The API uses a **dynamic auth guard** that activates when `POST /auth/token` is called:

1. Initially, `GET /books` and `GET /books/:id` work without auth (passes Level 3)
2. When `POST /auth/token` is called (Level 5), auth guard is activated
3. After activation, `GET /books` and `GET /books/:id` require valid `Bearer quest-token-xyz`
4. Write operations (POST, PUT, DELETE) remain open per API Quest requirements

### Thread-Safe Storage

- **Go**: `sync.RWMutex` protecting `map[string]*Book` with proper lock synchronization
- **Bun**: Single-threaded event loop, `Map` is naturally safe
- **Python**: GIL makes dict operations thread-safe

### Pagination

- Query parameters: `?page=1&limit=10`
- Defaults: `page=1`, `limit=10`
- Bounds: `page >= 1`, `1 <= limit <= 100` (max 100 items per page for production safety)

### Search

- Query parameter: `?author=alice`
- Case-insensitive substring match
- Example: `?author=alice` matches "Alice", "Alice Smith", "alice@example.com"

### Error Handling

- **400 Bad Request**: Invalid schema, missing required fields, empty strings
- **404 Not Found**: Resource not found
- **401 Unauthorized**: Missing or invalid auth token (after activation)
- **204 No Content**: Successful deletion

## Production-Grade Features

### Security
- Dynamic auth guard protecting read operations after Level 5
- Input validation with proper type checking
- SQL injection prevention (in-memory storage)
- XSS prevention (proper JSON handling)

### Performance
- Pagination with upper limits (max 100 per page)
- Efficient in-memory storage
- Thread-safe operations (Go: `sync.RWMutex`, Bun: event loop, Python: GIL)

### Reliability
- Proper error handling (400, 404, 401, 204)
- Deterministic pagination (sorted keys in Go)
- Year validation (0-9999 range)
- Non-empty string validation

### Maintainability
- Consistent code formatting
- Comprehensive test coverage
- Clear separation of concerns
- Type safety (Go: compile-time, Bun: TypeScript, Python: Pydantic)

## Makefile Commands

```bash
# Setup
make setup              # Setup all stacks
make setup-go           # Setup Go stack
make setup-bun          # Setup Bun stack
make setup-py           # Setup Python stack

# Testing
make test               # Run all tests
make test-go            # Run Go tests
make test-bun           # Run Bun tests
make test-py            # Run Python tests

# Smoke Tests (server must be running)
make smoke              # Test all running servers
make smoke-go           # Test Go server
make smoke-bun          # Test Bun server
make smoke-py           # Test Python server

# Docker
make docker-compose-up  # Start all services with Docker
make docker-compose-down
make docker-compose-logs

# Build & Deploy
make build              # Build all Docker images
make deploy             # Deploy all to Fly.io
make deploy-go          # Deploy Go to Fly.io
make deploy-bun         # Deploy Bun to Fly.io
make deploy-py          # Deploy Python to Fly.io

# Linting
make lint               # Lint all stacks
make lint-go            # Format and lint Go code
make lint-bun           # Type check Bun/TypeScript
make lint-py            # Type check Python

# Utility
make clean              # Clean build artifacts
make help               # Show help message
```

## License

MIT
