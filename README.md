# Build Counter

A modern HTTP service for tracking build start and finish times in CI/CD pipelines with web dashboard, metrics, and dual storage modes.

[![CI/CD Pipeline](https://github.com/rossigee/build-counter/workflows/CI%2FCD%20Pipeline/badge.svg)](https://github.com/rossigee/build-counter/actions)
[![Go Version](https://img.shields.io/badge/Go-1.25.7-blue.svg)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/rossigee/build-counter)](https://hub.docker.com/r/rossigee/build-counter)
[![Helm Chart](https://img.shields.io/badge/Helm-v3-blue.svg)](https://rossigee.github.io/build-counter)

## ✨ Features

### 🎯 Core Functionality
- **Build Tracking**: Record build start and finish times with unique IDs
- **Dual Storage Modes**: PostgreSQL database or Kubernetes ConfigMap
- **Web Dashboard**: Interactive HTML interface showing project status
- **REST API**: JSON endpoints for programmatic access

### 📊 Monitoring & Observability  
- **Prometheus Metrics**: `/metrics` endpoint with detailed application metrics
- **Health Checks**: `/health`, `/healthz`, `/readyz` endpoints for Kubernetes
- **OpenTelemetry Tracing**: Optional OTLP tracing support
- **Structured Logging**: Comprehensive request and error logging

### 🚀 Production Ready
- **Security Hardened**: Input validation, security headers, timeouts
- **Docker Support**: Multi-stage builds with security best practices
- **CI/CD Pipeline**: GitHub Actions with automated testing and publishing
- **Pre-commit Hooks**: Automated code quality and security checks

## 🖥️ Web Interface

The build counter includes a modern web dashboard accessible at `http://localhost:8080/`:

**Main Dashboard:**
- Shows all projects with their latest build status
- Real-time status indicators (Running/Completed)
- Build duration and timestamps
- Click-through navigation to detailed build history (database mode)

**Features by Mode:**
- **Database Mode**: Full build history with clickable rows
- **Lightweight Mode**: Latest build status only

## ⚡ Quick Start

### Option 1: Database Mode (Full Features)

**Prerequisites:**
- PostgreSQL database
- Go 1.25.7+ or Docker

**Setup:**
```bash
# 1. Create database schema
psql -d your_db -f builds.sql

# 2. Set database connection
export DATABASE_URL="postgres://user:password@localhost:5432/builddb?sslmode=disable"

# 3. Run the service
./build-counter
# or
make run
```

### Option 2: Lightweight Mode (Kubernetes ConfigMap)

**Prerequisites:**
- Kubernetes cluster access
- Proper RBAC permissions for ConfigMap access

**Setup:**
```bash
# Set Kubernetes namespace (optional)
export NAMESPACE="your-namespace"
export CONFIGMAP_NAME="build-counter"

# Run in lightweight mode
./build-counter --lightweight
```

### Option 3: Docker

```bash
# Pull from GitHub Container Registry (recommended)
docker pull ghcr.io/rossigee/build-counter:latest

# Or from Docker Hub
docker pull rossigee/build-counter:latest

# Run in database mode
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@host:5432/db" \
  ghcr.io/rossigee/build-counter:latest

# Run in lightweight mode  
docker run -p 8080:8080 \
  -v ~/.kube/config:/root/.kube/config \
  -e NAMESPACE="default" \
  ghcr.io/rossigee/build-counter:latest --lightweight
```

## 📖 Usage

### Command Line Options

```bash
build-counter [options]

Options:
  --version          Show version information
  --help             Show help message  
  --lightweight      Use Kubernetes ConfigMap storage
  --health-check     Check if service is healthy
```

### API Endpoints

#### Build Management
```bash
# Start a build
curl -X POST "http://localhost:8080/start?name=my-project&build_id=abc123"
# Response: {"next_id": 42}

# Finish a build  
curl -X POST "http://localhost:8080/finish?name=my-project&build_id=abc123"
# Response: HTTP 201 Created
```

#### Data Access
```bash
# Get all projects (JSON)
curl http://localhost:8080/api/projects

# Get builds for a project (JSON)  
curl "http://localhost:8080/api/projects?name=my-project"

# Web dashboard
open http://localhost:8080/
```

#### Health & Monitoring
```bash
# Health check
curl http://localhost:8080/health

# Kubernetes probes
curl http://localhost:8080/healthz   # Liveness
curl http://localhost:8080/readyz    # Readiness

# Prometheus metrics
curl http://localhost:8080/metrics
```

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Yes (database mode) | - |
| `NAMESPACE` | Kubernetes namespace | No | `default` |
| `CONFIGMAP_NAME` | ConfigMap name | No | `build-counter` |

### OpenTelemetry Tracing (Optional)

```bash
# Enable OTLP tracing
export OTEL_EXPORTER_OTLP_ENDPOINT="http://jaeger:4318"
export OTEL_SERVICE_NAME="build-counter"
export OTEL_SERVICE_VERSION="1.0.0"

# For insecure connections
export OTEL_EXPORTER_OTLP_INSECURE="true"
```

## 🛠️ Development

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run with hot reload
make run

# Build Docker image
make image
```

### Code Quality

```bash
# Install development dependencies
make dev-deps

# Run linting
make lint

# Format code
make fmt

# Security scan
make sec
```

### Pre-commit Setup

```bash
# Install pre-commit
pip install pre-commit

# Install hooks
pre-commit install

# Run manually
pre-commit run --all-files
```

## 📊 Metrics

The `/metrics` endpoint provides Prometheus-compatible metrics:

- `build_counter_requests_total` - Total HTTP requests
- `build_counter_builds_started_total` - Total builds started  
- `build_counter_builds_finished_total` - Total builds finished
- `build_counter_errors_total` - Total errors
- `build_counter_uptime_seconds` - Service uptime
- `build_counter_memory_usage_bytes` - Memory usage
- Standard Go runtime metrics

## 🐳 Kubernetes Deployment

### Using Helm (Recommended)

```bash
# Add Helm repository
helm repo add build-counter https://rossigee.github.io/build-counter
helm repo update

# Install with database mode (default)
helm install my-build-counter build-counter/build-counter \
  --set storage.database.url="postgres://user:pass@host:5432/db"

# Install with lightweight mode
helm install my-build-counter build-counter/build-counter \
  --set storage.mode=lightweight

# Install from GitHub Container Registry (OCI)
helm install my-build-counter oci://ghcr.io/rossigee/helm/build-counter \
  --version 1.0.0
```

See the [Helm chart documentation](helm/build-counter/README.md) for all configuration options.

### Manual Deployment

#### Database Mode

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: build-counter
spec:
  replicas: 2
  selector:
    matchLabels:
      app: build-counter
  template:
    metadata:
      labels:
        app: build-counter
    spec:
      containers:
      - name: build-counter
        image: ghcr.io/rossigee/build-counter:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: build-counter-secret
              key: database-url
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
```

### Lightweight Mode (ConfigMap)

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: build-counter
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: build-counter
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "create", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: build-counter
subjects:
- kind: ServiceAccount
  name: build-counter
roleRef:
  kind: Role
  name: build-counter
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: build-counter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: build-counter
  template:
    metadata:
      labels:
        app: build-counter
    spec:
      serviceAccountName: build-counter
      containers:
      - name: build-counter
        image: ghcr.io/rossigee/build-counter:latest
        args: ["--lightweight"]
        ports:
        - containerPort: 8080
        env:
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Run linting (`make lint`) 
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## 📋 Roadmap

- [ ] Authentication and authorization
- [ ] Build artifacts tracking
- [ ] Slack/Teams notifications
- [ ] Grafana dashboard templates
- [ ] Helm chart
- [ ] Multi-cluster support

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/)
- Kubernetes integration via [client-go](https://github.com/kubernetes/client-go)
- Observability with [OpenTelemetry](https://opentelemetry.io/)
- Styled with modern CSS and responsive design