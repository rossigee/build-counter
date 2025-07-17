# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Counter Service

This is a modernized Go HTTP service that tracks build start/finish times with dual storage modes. It provides REST endpoints, web UI, metrics, and comprehensive monitoring for CI/CD systems.

## Architecture

**Dual-storage architecture** with modular design:
- **Database mode**: PostgreSQL storage using `lib/pq` driver
- **Lightweight mode**: Kubernetes ConfigMap storage using `client-go`
- Storage interface abstraction allows switching between modes

**Multi-file modular structure**:
- `main.go`: HTTP server, handlers, web UI
- `database.go`: PostgreSQL storage implementation  
- `configmap.go`: Kubernetes ConfigMap storage implementation
- `tracing.go`: OpenTelemetry/OTLP distributed tracing
- Storage selection based on `DATABASE_URL` environment variable

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

**Storage mode selection**:
- **Database mode**: Set `DATABASE_URL` (e.g., `postgres://user:pass@host:5432/dbname?sslmode=disable`)
- **Lightweight mode**: Leave `DATABASE_URL` unset, uses Kubernetes ConfigMap

**Environment variables**:
- `DATABASE_URL`: PostgreSQL connection (optional, triggers database mode)
- `KUBERNETES_NAMESPACE`: K8s namespace for ConfigMap (default: "default")
- `PORT`: HTTP server port (default: 8080)
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry collector endpoint (optional)
- `OTEL_EXPORTER_OTLP_HEADERS`: OTLP authentication headers (optional)

## API Endpoints

**Core build tracking**:
- `POST /start`: Register build start (`name`, `build_id` params)
- `POST /finish`: Register build completion (`name`, `build_id` params)

**Monitoring & health**:
- `GET /healthz`: Kubernetes liveness probe
- `GET /readyz`: Kubernetes readiness probe  
- `GET /metrics`: Prometheus metrics endpoint
- `GET /version`: Service version info

**Web interface & API**:
- `GET /`: HTML dashboard with project table (dark mode)
- `GET /api/projects`: JSON list of all projects
- `GET /api/projects/{name}/builds`: JSON list of builds for project

## Testing

**Unit tests** (26.7% coverage):
```bash
make test           # Run all tests
go test -v ./...    # Run with verbose output
go test -cover      # Run with coverage
```

**Test coverage includes**:
- Storage interface implementations (ConfigMap & Database)
- Input validation and security middleware
- HTTP handlers with mock storage
- Environment configuration and setup

**Manual testing**:
```bash
# Start a build
curl -X POST "http://localhost:8080/start?name=my-project&build_id=abc123"

# Finish a build  
curl -X POST "http://localhost:8080/finish?name=my-project&build_id=abc123"

# View web dashboard
open http://localhost:8080

# Check metrics
curl http://localhost:8080/metrics

# Health checks
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Deployment

**Docker**: Multi-stage build with Alpine runtime for health checks and Kubernetes compatibility

**Kubernetes**: Complete Helm chart with:
- Deployment with configurable storage mode
- ServiceAccount and RBAC for ConfigMap access
- Service, Ingress, NetworkPolicy
- HPA, PodDisruptionBudget, ServiceMonitor

**Docker Compose**: Demo environment with PostgreSQL, Grafana, Prometheus, and data generator

**CI/CD**: GitHub Actions pipeline with:
- Automated testing and security scanning
- Docker image publishing to GHCR
- Helm chart publishing to GitHub Pages