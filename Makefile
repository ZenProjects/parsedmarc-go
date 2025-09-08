# Makefile for parsedmarc-go

.PHONY: build clean test run docker help

# Variables
BINARY_NAME=parsedmarc-go
BINARY_PATH=./cmd/parsedmarc-go
BUILD_DIR=./build
VERSION=1.0.0
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(BINARY_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux: ## Build for Linux
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BINARY_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

build-windows: ## Build for Windows
	@echo "Building $(BINARY_NAME) for Windows..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(BINARY_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

build-darwin: ## Build for macOS
	@echo "Building $(BINARY_NAME) for macOS..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(BINARY_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"

build-all: build-linux build-windows build-darwin ## Build for all platforms

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@go clean

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-parser: ## Run parser tests only
	@echo "Running parser tests..."
	@go test -v ./internal/parser/

test-http: ## Run HTTP server tests only
	@echo "Running HTTP server tests..."
	@go test -v ./internal/http/

test-integration: ## Run integration tests (requires ClickHouse)
	@echo "Running integration tests..."
	@go test -v ./internal/storage/clickhouse/

test-short: ## Run tests excluding integration tests
	@echo "Running short tests..."
	@go test -v -short ./...

benchmark: ## Run benchmark tests
	@echo "Running benchmark tests..."
	@go test -v -bench=. ./...

test-samples: ## Verify all sample files can be parsed
	@echo "Testing sample files..."
	@go run $(BINARY_PATH) -input samples/aggregate/
	@echo "Sample files test completed"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

run: build ## Build and run with example config
	@echo "Running $(BINARY_NAME) with example config..."
	@$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml.example

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t parsedmarc-go:$(VERSION) .
	@docker tag parsedmarc-go:$(VERSION) parsedmarc-go:latest

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run --rm -v $(PWD)/config.yaml:/app/config.yaml -v $(PWD)/reports:/app/reports parsedmarc-go:latest

install-deps: ## Install development dependencies
	@echo "Installing development dependencies..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev-setup: install-deps deps ## Setup development environment
	@echo "Development environment setup complete"

release: clean build-all ## Create release artifacts
	@echo "Creating release artifacts..."
	@cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd $(BUILD_DIR) && zip $(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@echo "Release artifacts created in $(BUILD_DIR)/"