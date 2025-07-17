# GitHub Container Registry Usage

This project publishes Docker images to GitHub Container Registry (ghcr.io) and Helm charts to both GitHub Pages and GHCR.

## Docker Images

Docker images are automatically published to GitHub Container Registry on every push to master and on release tags.

### Pull the latest image:
```bash
docker pull ghcr.io/rossigee/build-counter:latest
```

### Pull a specific version:
```bash
docker pull ghcr.io/rossigee/build-counter:v1.0.0
```

### Pull by branch:
```bash
docker pull ghcr.io/rossigee/build-counter:master
```

## Helm Charts

Helm charts are published in two ways:

### 1. GitHub Pages Helm Repository

Add the repository:
```bash
helm repo add build-counter https://rossigee.github.io/build-counter
helm repo update
```

Install the chart:
```bash
helm install my-build-counter build-counter/build-counter
```

### 2. OCI Registry (GHCR)

Pull and install directly from GHCR:
```bash
helm install my-build-counter oci://ghcr.io/rossigee/helm/build-counter --version 1.0.0
```

## Authentication

For private repositories, authenticate with GitHub:

### Docker:
```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USER --password-stdin
```

### Helm OCI:
```bash
echo $GITHUB_TOKEN | helm registry login ghcr.io -u $GITHUB_USER --password-stdin
```

## Multi-Architecture Support

Images are built for both `linux/amd64` and `linux/arm64` platforms.

## Image Tags

- `latest`: Latest build from master branch
- `master`: Latest build from master branch
- `v1.0.0`: Specific version release
- `1.0`: Latest patch version of 1.0.x
- `1`: Latest minor version of 1.x.x
- `master-sha-abc123`: Specific commit on master branch

## Usage in Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: build-counter
spec:
  template:
    spec:
      containers:
      - name: build-counter
        image: ghcr.io/rossigee/build-counter:latest
        imagePullPolicy: Always
```

For private repositories, create an image pull secret:
```bash
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=$GITHUB_USER \
  --docker-password=$GITHUB_TOKEN \
  --docker-email=$EMAIL
```