# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Counter Service

This is a simple Go HTTP service that tracks build start/finish times in a PostgreSQL database. It provides REST endpoints for CI/CD systems to register build events.

## Architecture

**Single-file application**: All code is in `main.go` with minimal dependencies
- HTTP server with two endpoints: `/start` and `/finish`
- Direct PostgreSQL database connection using `lib/pq` driver
- Simple JSON responses for build tracking

**Database schema**: Single `builds` table (see `builds.sql`)
- Tracks build name, build_id, started timestamp, and finished timestamp
- Uses SERIAL primary key for unique build records

## Development Commands

**Build and run**:
```bash
make build          # Build binary
make run            # Build and run server
go run main.go      # Direct run without build
```

**Docker**:
```bash
make image          # Build Docker image
make push           # Push to registry
```

**Database setup**:
```bash
# Create database schema
psql -d your_db -f builds.sql
```

## Environment Configuration

**Required environment variables**:
- `DATABASE_URL`: PostgreSQL connection string (e.g., `postgres://user:pass@host:5432/dbname?sslmode=disable`)

**Server configuration**:
- Listens on port 8080
- No configuration files - all settings via environment variables

## API Endpoints

**POST /start**: Register build start
- Parameters: `name` (string), `build_id` (string)  
- Returns: `{"next_id": 123}` with database record ID

**POST /finish**: Register build completion
- Parameters: `name` (string), `build_id` (string)
- Updates existing record with finish timestamp

## Testing

**Unit tests**: 
```bash
make test           # Run all tests
go test -v ./...    # Run with verbose output
go test -cover      # Run with coverage
```

**Test coverage**:
- Input validation functions
- HTTP middleware (security headers, method filtering)
- Handler validation logic
- Health check endpoint

**Manual testing via curl**:
```bash
# Start a build
curl -X POST "http://localhost:8080/start?name=my-project&build_id=abc123"

# Finish a build  
curl -X POST "http://localhost:8080/finish?name=my-project&build_id=abc123"

# Health check
curl http://localhost:8080/health
```

## Deployment

Uses multi-stage Docker build with Alpine Linux runtime. Binary compiled with CGO disabled for static linking.