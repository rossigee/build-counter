# Build stage
FROM golang:1.24-alpine AS builder

# Install git and ca-certificates for private modules
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for build
RUN adduser -D -g '' appuser

WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Build the binary with security flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o build-counter .

# Final stage - using alpine for Kubernetes tools
FROM alpine:3.19

# Install ca-certificates and curl for health checks
RUN apk add --no-cache ca-certificates curl tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Copy the binary
COPY --from=builder /build/build-counter /build-counter

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check using the /healthz endpoint
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

# Set environment variables for Kubernetes
ENV KUBERNETES_NAMESPACE=default
ENV PORT=8080

# Run the binary
ENTRYPOINT ["/build-counter"]
