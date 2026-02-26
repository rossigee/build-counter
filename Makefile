# Define the binary and image names
BINARY_NAME=build-counter
IMAGE_NAME=rossigee/build-counter

# Get version from git tags, or use 'dev' if no tags exist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
FULL_IMAGE_NAME=${IMAGE_NAME}:${VERSION}

# Default make command builds the binary
all: build

# Build binary from Go source
build:
	go build -ldflags "-X main.version=${VERSION}" -o ${BINARY_NAME} .

# Run the server
run: build
	./${BINARY_NAME}

# Run tests
test:
	go test -v ./...

# Run linting
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Run security scanner
sec:
	gosec ./...

# Build Docker image
image:
	docker build -t ${FULL_IMAGE_NAME} .
	docker tag ${FULL_IMAGE_NAME} ${IMAGE_NAME}:latest

# Push Docker image
push:
	docker push ${FULL_IMAGE_NAME}
	docker push ${IMAGE_NAME}:latest

# Clean up the binary
clean:
	go clean
	rm -f ${BINARY_NAME}

# Install development dependencies
dev-deps:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Create a new release tag
tag:
	@echo "Current version: ${VERSION}"
	@read -p "Enter new version (e.g., v1.0.0): " NEW_VERSION; \
	git tag -a $$NEW_VERSION -m "Release $$NEW_VERSION"; \
	git push origin $$NEW_VERSION

# Show current version
version:
	@echo ${VERSION}

# Phony targets for commands that don't represent files
.PHONY: all build run test lint fmt sec clean image push dev-deps tag version
