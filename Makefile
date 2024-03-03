# Define the binary and image names
BINARY_NAME=build-counter
IMAGE_NAME=rossigee/build-counter:v0.2.0

# Default make command builds the binary
all: build

# Build binary from Go source
build:
	go build -o ${BINARY_NAME} main.go

# Run the server
run: build
	./${BINARY_NAME}

# Build Docker image
image:
	docker build -t ${IMAGE_NAME} .

# Push Docker image
push:
	docker push ${IMAGE_NAME}

# Clean up the binary
clean:
	go clean
	rm -f ${BINARY_NAME}

# Phony targets for commands that don't represent files
.PHONY: all build run clean image
