.PHONY: test test-unit test-integration test-coverage build clean docker-build docker-up docker-down

test: test-unit

test-unit:
	@echo "Running unit tests..."
	go test -short -v -race -cover ./...

test-integration:
	@echo "Running integration tests..."
	go test -v -race -cover ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

build:
	@echo "Building voxly..."
	go build -o bin/voxly ./cmd/voxly

clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

lint:
	@echo "Running linters..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

help:
	@echo "Available targets:"
	@echo "  test              - Run unit tests"
	@echo "  test-unit         - Run unit tests only"
	@echo "  test-integration  - Run all tests including integration"
	@echo "  test-coverage     - Run tests with coverage report"
	@echo "  build             - Build bot and worker"
	@echo "  clean             - Remove build artifacts"
	@echo "  docker-build      - Build Docker images"
	@echo "  docker-up         - Start Docker containers"
	@echo "  docker-down       - Stop Docker containers"
	@echo "  lint              - Run linters"
	@echo "  fmt               - Format code"
	@echo "  vet               - Run go vet"
	@echo "  deps              - Download dependencies"
