# Build Counter Helm Chart

This Helm chart deploys the Build Counter application on Kubernetes with support for both database and lightweight (ConfigMap) storage modes.

## Installation

### From Helm Repository

```bash
helm repo add build-counter https://rossigee.github.io/build-counter
helm repo update
helm install my-build-counter build-counter/build-counter
```

### From OCI Registry

```bash
helm install my-build-counter oci://ghcr.io/rossigee/helm/build-counter --version 1.0.0
```

### From Source

```bash
git clone https://github.com/rossigee/build-counter.git
cd build-counter
helm install my-build-counter ./helm/build-counter
```

## Configuration

### Storage Modes

The chart supports two storage modes:

#### 1. Database Mode (Default)

```yaml
storage:
  mode: "database"
  database:
    url: "postgres://user:password@postgres:5432/buildcounter"
    # Or use existing secret
    secretName: "my-database-secret"
    secretKey: "database-url"
```

#### 2. Lightweight Mode (Kubernetes ConfigMap)

```yaml
storage:
  mode: "lightweight"
  configmap:
    name: "build-counter"
    namespace: ""  # Uses release namespace if empty
```

### Key Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `ghcr.io/rossigee/build-counter` |
| `image.tag` | Image tag | Chart appVersion |
| `storage.mode` | Storage mode (`database` or `lightweight`) | `database` |
| `storage.database.url` | PostgreSQL connection URL | `""` |
| `storage.configmap.name` | ConfigMap name for lightweight mode | `build-counter` |
| `replicaCount` | Number of replicas | `1` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `ingress.enabled` | Enable ingress | `false` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `autoscaling.enabled` | Enable HPA | `false` |
| `monitoring.enabled` | Enable metrics endpoint | `true` |
| `monitoring.serviceMonitor.enabled` | Create ServiceMonitor | `false` |

### Example Values Files

#### Production with PostgreSQL

```yaml
# values-production.yaml
replicaCount: 3

storage:
  mode: "database"
  database:
    secretName: "postgres-credentials"
    secretKey: "connection-string"

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: build-counter.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: build-counter-tls
      hosts:
        - build-counter.example.com

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 250m
    memory: 256Mi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

monitoring:
  serviceMonitor:
    enabled: true
```

#### Development with Lightweight Mode

```yaml
# values-dev.yaml
storage:
  mode: "lightweight"

resources:
  limits:
    cpu: 200m
    memory: 128Mi
  requests:
    cpu: 50m
    memory: 64Mi
```

## Security

The chart includes security best practices by default:

- Non-root user (UID 65534)
- Read-only root filesystem
- No privilege escalation
- Dropped all capabilities
- Seccomp profile: RuntimeDefault

## RBAC

When using lightweight mode, the chart automatically creates:
- ServiceAccount
- Role with ConfigMap permissions
- RoleBinding

## Monitoring

The chart supports Prometheus monitoring:

```yaml
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
    namespace: monitoring
    labels:
      prometheus: kube-prometheus
```

## Upgrades

```bash
# Upgrade to latest version
helm repo update
helm upgrade my-build-counter build-counter/build-counter

# Upgrade with new values
helm upgrade my-build-counter build-counter/build-counter \
  --set replicaCount=5
```

## Uninstall

```bash
helm uninstall my-build-counter
```

## Development

### Testing the Chart

```bash
# Lint the chart
helm lint ./helm/build-counter

# Dry run installation
helm install my-build-counter ./helm/build-counter --dry-run --debug

# Template rendering
helm template my-build-counter ./helm/build-counter
```

### Package the Chart

```bash
helm package ./helm/build-counter
```