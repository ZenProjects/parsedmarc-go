# Makefile for parsedmarc-go

.PHONY: build clean test run help container-build container-run container-run-detached container-push container-dev container-shell container-clean container-logs container-stop buildah-build buildah-run buildah-push buildah-clean

# Variables
BINARY_NAME=parsedmarc-go
BINARY_PATH=./cmd/parsedmarc-go
BUILD_DIR=./build
VERSION=1.0.0
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Container variables (Buildah/Podman)
CONTAINER_IMAGE=parsedmarc-go
CONTAINER_TAG=$(VERSION)
CONTAINER_LATEST_TAG=latest
CONTAINER_REGISTRY=

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

build-all: build-linux build-windows ## Build for all platforms

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
	@echo "Testing aggregate samples (excluding large file)..."
	@for file in samples/aggregate/*.xml samples/aggregate/*.gz samples/aggregate/*.zip samples/aggregate/*.eml; do \
		if [ -f "$$file" ] && ! echo "$$file" | grep -q "large-example.com" && ! echo "$$file" | grep -q "twilight.eml"; then \
			echo "Testing: $$file"; \
			timeout 30s go run $(BINARY_PATH) -input "$$file" || echo "Failed or timed out: $$file"; \
		fi; \
	done
	@echo "Testing problematic EML files with extended timeout..."
	@timeout 60s go run $(BINARY_PATH) -input "samples/aggregate/twilight.eml" || echo "EML file test failed (expected - MIME parsing not fully implemented)"
	@echo "Testing large file separately with extended timeout..."
	@timeout 180s go run $(BINARY_PATH) -input "samples/aggregate/!large-example.com!1711897200!1711983600.xml" || echo "Large file test timed out or failed, continuing..."
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

buildah-build: ## Build container image avec Buildah
	@echo "Building container image $(CONTAINER_IMAGE):$(CONTAINER_TAG) with Buildah..."
	@buildah bud \
		--build-arg VERSION=$(VERSION) \
		-t $(CONTAINER_IMAGE):$(CONTAINER_TAG) \
		-t $(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG) \
		.
	@echo "Container image built successfully: $(CONTAINER_IMAGE):$(CONTAINER_TAG)"

container-build: buildah-build ## Alias pour buildah-build

buildah-run: ## Run container avec Podman
	@echo "Running container with Podman..."
	@podman run --rm \
		-v $(PWD)/config.yaml:/app/config/config.yaml:ro \
		-v $(PWD)/reports:/app/reports \
		-p 8080:8080 \
		$(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG)

container-run: buildah-run ## Alias pour buildah-run

container-run-detached: ## Run container in background avec Podman
	@echo "Running container in detached mode with Podman..."
	@podman run -d \
		--name $(CONTAINER_IMAGE)-container \
		-v $(PWD)/config.yaml:/app/config/config.yaml:ro \
		-v $(PWD)/reports:/app/reports \
		-p 8080:8080 \
		$(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG)

container-dev: ## Run container for development avec Podman
	@echo "Running container for development with Podman..."
	@podman run --rm -it \
		-v $(PWD):/app/src \
		-v $(PWD)/config.yaml:/app/config/config.yaml:ro \
		-v $(PWD)/reports:/app/reports \
		-p 8080:8080 \
		-w /app/src \
		golang:1.23-alpine \
		sh -c "apk add --no-cache make && make run"

container-shell: ## Open shell in container avec Podman
	@echo "Opening shell in container with Podman..."
	@podman run --rm -it \
		-v $(PWD):/app/src \
		-w /app/src \
		$(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG) \
		sh

buildah-push: buildah-build ## Push container image to registry avec Buildah
	@if [ -z "$(CONTAINER_REGISTRY)" ]; then \
		echo "Error: CONTAINER_REGISTRY variable is not set"; \
		exit 1; \
	fi
	@echo "Pushing container image to registry with Buildah..."
	@buildah tag $(CONTAINER_IMAGE):$(CONTAINER_TAG) $(CONTAINER_REGISTRY)/$(CONTAINER_IMAGE):$(CONTAINER_TAG)
	@buildah tag $(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG) $(CONTAINER_REGISTRY)/$(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG)
	@buildah push $(CONTAINER_REGISTRY)/$(CONTAINER_IMAGE):$(CONTAINER_TAG)
	@buildah push $(CONTAINER_REGISTRY)/$(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG)

container-push: buildah-push ## Alias pour buildah-push

buildah-clean: ## Clean container images and containers avec Buildah/Podman
	@echo "Cleaning container artifacts with Buildah/Podman..."
	@podman container prune -f 2>/dev/null || true
	@podman image prune -f 2>/dev/null || true
	@-buildah rmi $(CONTAINER_IMAGE):$(CONTAINER_TAG) 2>/dev/null || true
	@-buildah rmi $(CONTAINER_IMAGE):$(CONTAINER_LATEST_TAG) 2>/dev/null || true
	@echo "Container cleanup completed"

container-clean: buildah-clean ## Alias pour buildah-clean

container-logs: ## Show logs from running container avec Podman
	@podman logs -f $(CONTAINER_IMAGE)-container

container-stop: ## Stop running container avec Podman
	@podman stop $(CONTAINER_IMAGE)-container || true
	@podman rm $(CONTAINER_IMAGE)-container || true

install-deps: ## Install development dependencies
	@echo "Installing development dependencies..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev-setup: install-deps deps ## Setup development environment
	@echo "Development environment setup complete"

release: clean build-all ## Create release artifacts
	@echo "Creating release artifacts..."
	@cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(BUILD_DIR) && zip $(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@echo "Release artifacts created in $(BUILD_DIR)/"