# Fastwork Challenge Notification Service

> A production-grade notification service built with Go, following clean architecture principles with support for multiple notification providers, intelligent caching, circuit breaker patterns, and comprehensive observability.

**Copyright (c) 2025 Jirasak Jungburapa. All Rights Reserved.**

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [API Endpoints](#api-endpoints)
- [Configuration](#configuration)
- [Database Schema](#database-schema)
- [Metrics & Observability](#metrics--observability)
- [Development](#development)
- [Testing](#testing)

## Overview

This notification service provides a robust, scalable solution for sending notifications to buyers and sellers through multiple channels (Email, Push Notifications). The service implements clean architecture principles with clear separation of concerns across layers: handlers, services, repositories, and clients.

The service is designed with production reliability in mind, featuring:
- **Circuit breaker pattern** for resilient external service calls
- **Multi-layer caching** to reduce database load
- **Priority-based fallback** across notification providers
- **Comprehensive metrics** using OpenTelemetry and Prometheus
- **Structured logging** with Zap
- **Dependency injection** using Uber Fx

## Architecture

The service follows **clean architecture** principles with clear layer separation:

![Architecture Diagram](./resources/arch.png)

### Layer Overview

1. **Handler Layer** - HTTP request handling and validation using Gin framework
2. **Service Layer** - Business logic for notification orchestration
3. **Repository Layer** - Data access with cache-aside pattern
   - **Cache**: Ristretto in-memory cache (10-minute TTL)
   - **Persistent**: PostgreSQL with GORM ORM
4. **Client Layer** - External service integration with circuit breaker
5. **Sidecar Components**:
   - **Metrics**: OpenTelemetry + Prometheus exporters
   - **Log**: Zap structured logging
   - **Trace**: OpenTelemetry tracing (prepared for future implementation)

### Request Flow

```
Incoming Request
    → Handler (validation)
    → Service (business logic)
    → Repository (cache → database fallback)
    → Client (circuit breaker → external provider)
```

## Features

- **Multiple Notification Channels**: Support for Email and Push Notifications
- **Intelligent Routing**:
  - Buyer notifications: Email only
  - Seller notifications: Email + Push (parallel execution)
- **High Availability**:
  - Priority-based provider fallback
  - Circuit breaker per-host isolation
  - Automatic recovery mechanisms
- **Performance Optimization**:
  - In-memory caching with Ristretto
  - Database query optimization with indexes
  - Parallel notification sending for sellers
- **Observability**:
  - Prometheus metrics for HTTP server/client
  - Circuit breaker state tracking
  - Structured logging at all layers
- **Production Ready**:
  - Graceful shutdown handling
  - Health check endpoint
  - Docker deployment with migrations
  - Non-root container execution

## Installation

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- PostgreSQL 18.0+ (if running locally)

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone <repository-url>
cd fw-challenge-notification-service

# Start all services
docker compose up -d

# Check service health
curl http://localhost:8080/healthz
```

This will start:
- Notification service on port 8080
- PostgreSQL database on port 5432
- Mock HTTP server on ports 9090-9093
- Automatic database migration

### Local Development

```bash
# Install dependencies
go mod download

# Set environment variables (see Configuration section)
export DB_HOST=localhost
export DB_PORT=5432
# ... other variables

# Run database migrations
migrate -path ./migrations -database "postgres://myuser:mypassword@localhost:5432/postgres?sslmode=disable" up

# Build and run
go build -o server ./cmd/api/
./server
```

## Usage

### Send Notification to Buyer

```bash
curl -X POST http://localhost:8080/api/v1.0/recipient/buyer/notify \
  -H "Content-Type: application/json" \
  -d '{
    "to": "buyer@example.com",
    "title": "Order Confirmation",
    "message": "Your order has been confirmed"
  }'
```

### Send Notification to Seller

```bash
curl -X POST http://localhost:8080/api/v1.0/recipient/seller/notify \
  -H "Content-Type: application/json" \
  -d '{
    "to": "seller@example.com",
    "title": "New Order",
    "message": "You have a new order"
  }'
```

### Check Health

```bash
curl http://localhost:8080/healthz
```

### View Metrics

```bash
curl http://localhost:8080/metrics
```

## API Endpoints

### POST /api/v1.0/recipient/:recipient/notify

Send a notification to a specific recipient type.

**Path Parameters:**
- `recipient` (string, required): Recipient type - `buyer` or `seller` (case-sensitive, lowercase)

**Request Body:**
```json
{
  "to": "user@example.com",
  "title": "Notification Title",
  "message": "Notification message content"
}
```

**Success Response:**
- **Code**: 200 OK
- **Content**: `{ "message": "notification sent successfully" }`

**Error Responses:**
- **Code**: 422 Unprocessable Entity
  ```json
  {
    "error": {
      "code": "E101",
      "message": "validation error message"
    }
  }
  ```
- **Code**: 500 Internal Server Error
  ```json
  {
    "error": {
      "code": "E102",
      "message": "internal server error"
    }
  }
  ```

### GET /healthz

Health check endpoint.

**Response:**
```json
{
  "message": "server is running"
}
```

### GET /metrics

Prometheus-compatible metrics endpoint. Returns metrics in Prometheus exposition format.

## Configuration

All configuration is done via environment variables. The service uses `.env.example` as a template.

### HTTP Server
- `HTTP_SERVER_PORT` - Server port (default: `8080`)

### HTTP Client
- `HTTP_CLIENT_TIMEOUT` - Client request timeout (default: `5s`)

### Circuit Breaker
- `CIRCUIT_BREAKER_MAX_HALF_OPEN_REQUESTS` - Max requests in half-open state (default: `5`)
- `CIRCUIT_BREAKER_OPEN_STATE_TIMEOUT` - Time before retry from open state (default: `60s`)
- `CIRCUIT_BREAKER_MIN_REQUESTS_BEFORE_TRIP` - Min requests before tripping (default: `3`)
- `CIRCUIT_BREAKER_FAILURE_THRESHOLD_PERCENT` - Failure percentage to trip (default: `60`)

### Cache (Ristretto)
- `CACHE_EXPIRED_TIME` - Cache entry TTL (default: `10m`)
- `CACHE_NUM_COUNTERS` - Number of keys to track frequency (default: `10000000`)
- `CACHE_MAX_COST` - Max cache size in bytes (default: `1073741824` = 1GB)
- `CACHE_BUFFER_ITEMS` - Buffer size for set operations (default: `64`)

### Database (PostgreSQL)
- `DB_HOST` - Database host (required)
- `DB_PORT` - Database port (required)
- `DB_NAME` - Database name (required)
- `DB_USERNAME` - Database user (required)
- `DB_PASSWORD` - Database password (required)
- `DB_SSLMODE` - SSL mode (default: `disable`)

## Database Schema

The service uses PostgreSQL with the following schema:

### notification_preferences table

```sql
CREATE TYPE notification_provider_type AS ENUM ('Email', 'PushNotification');

CREATE TABLE notification_preferences (
    id SERIAL PRIMARY KEY,
    provider_type notification_provider_type NOT NULL,
    provider_name VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    priority INTEGER NOT NULL,
    secret_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_notification_preferences_provider_priority
ON notification_preferences(provider_type, priority)
WHERE deleted_at IS NULL;
```

### Migrations

Database migrations are managed using [migrate/migrate](https://github.com/golang-migrate/migrate) and are automatically applied on startup via Docker Compose.

Manual migration:
```bash
migrate -path ./migrations -database "postgres://user:pass@host:5432/db?sslmode=disable" up
```

## Metrics & Observability

### HTTP Server Metrics

- `http.server.requests` (Counter) - Total HTTP requests
  - Labels: `http.method`, `http.route`, `http.status_code`
- `http.server.duration` (Histogram) - Request duration in seconds
  - Labels: `http.method`, `http.route`, `http.status_code`

### HTTP Client Metrics

- `http.client.requests` (Counter) - Total outbound HTTP requests
  - Labels: `http.method`, `http.host`, `http.status_code`, `error`
- `http.client.duration` (Histogram) - Request duration in seconds
  - Labels: `http.method`, `http.host`, `http.status_code`, `error`
- `circuit_breaker.state` (Gauge) - Current circuit breaker state (0=Closed, 1=Open, 2=HalfOpen)
  - Labels: `host`, `state`
- `circuit_breaker.state_changes` (Counter) - Circuit breaker state transitions
  - Labels: `host`, `from_state`, `to_state`

### Logging

The service uses [Zap](https://github.com/uber-go/zap) for structured logging with the following levels:
- **DEBUG**: Cache hits/misses, circuit breaker state checks
- **INFO**: Request initiation, successful operations
- **WARN**: Non-200 responses, cache misses
- **ERROR**: Database errors, HTTP failures, circuit breaker failures

## Development

### Project Structure

```
.
├── cmd/api/              # Application entrypoint
├── internal/
│   ├── handler/          # HTTP handlers
│   ├── service/          # Business logic
│   ├── repository/       # Data access layer
│   ├── client/           # External service clients
│   ├── metrics/          # Metrics collection
│   └── server/           # HTTP server setup
├── migrations/           # Database migrations
├── resources/            # Documentation resources
├── compose.yml           # Docker Compose configuration
├── Dockerfile            # Multi-stage build
└── .env.example          # Environment template
```

### Code Generation

The project uses `go generate` for mock generation:

```bash
# Generate all mocks
go generate ./...
```

### Dependency Injection

The service uses [Uber Fx](https://github.com/uber-go/fx) for dependency injection. Module organization:

```
fx.New(
    metrics.Module,      // OpenTelemetry setup
    server.Module,       // HTTP server
    handler.Module,      // Request handlers
    service.Module,      // Business logic
    repository.Module,   // Data access
    client.Module,       // External clients
)
```

## Testing

### Run All Tests

```bash
go test ./... -v -race -count=1
```

### Run Tests with Coverage

```bash
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```


### Mock Testing

The project uses [mockgen](https://github.com/uber-go/mock) for interface mocking:

```bash
# Install mockgen
go install go.uber.org/mock/mockgen@latest

# Generate mocks
go generate ./...
```
