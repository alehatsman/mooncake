# Mooncake Makefile
# Simple, focused targets for development and CI

.PHONY: help
help: ## Show this help message
	@echo "Mooncake - Development Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Common targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# ==============================================================================
# Build Targets
# ==============================================================================

.PHONY: build
build: ## Build the mooncake binary
	@echo "Building mooncake..."
	@go build -v -o out/mooncake ./cmd

.PHONY: install
install: build ## Build and install mooncake to /usr/local/bin
	@echo "Installing mooncake to /usr/local/bin..."
	@sudo cp ./out/mooncake /usr/local/bin/mooncake
	@sudo chmod +x /usr/local/bin/mooncake
	@echo "✓ Installed successfully"

.PHONY: clean
clean: ## Remove build artifacts
	@rm -rf out/
	@rm -rf testing-output/
	@echo "✓ Cleaned"

# ==============================================================================
# Development & Testing
# ==============================================================================

.PHONY: test
test: ## Run unit tests
	@echo "Running tests..."
	@go test -v ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	@go test -race ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@mkdir -p testing-output
	@go test -coverprofile=testing-output/coverage.out -covermode=atomic ./...
	@go tool cover -html=testing-output/coverage.out -o testing-output/coverage.html
	@echo "✓ Coverage report: testing-output/coverage.html"

# ==============================================================================
# Docker Testing (Linux environment)
# ==============================================================================

.PHONY: test-docker
test-docker: ## Run tests in Docker (Ubuntu, matches CI)
	@echo "Running tests in Ubuntu Docker (matches CI environment)..."
	@./scripts/test-ubuntu-quick.sh

.PHONY: test-docker-full
test-docker-full: ## Run full test suite in Docker (build + test + coverage + race)
	@echo "Running full test suite in Ubuntu Docker..."
	@./scripts/test-ubuntu-docker.sh

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@./scripts/run-integration-tests.sh

.PHONY: test-linux
test-linux: ## Build Linux binary and run smoke tests in Docker
	@echo "Building Linux binary and running smoke tests..."
	@./scripts/test-docker.sh ubuntu-22.04 smoke

# ==============================================================================
# Code Quality
# ==============================================================================

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -s -w .
	@echo "✓ Code formatted"

.PHONY: scan
scan: lint ## Run security scans (gosec + govulncheck)
	@echo "Running gosec security scan..."
	@gosec -exclude-generated ./...
	@echo ""
	@echo "Running govulncheck..."
	@govulncheck ./...

# ==============================================================================
# CI Target (matches GitHub Actions)
# ==============================================================================

.PHONY: ci
ci: lint test-race scan ## Run full CI suite (lint + test-race + scan)
	@echo ""
	@echo "✓ All CI checks passed!"

# ==============================================================================
# Documentation
# ==============================================================================

.PHONY: docs-gen
docs-gen: ## Generate preset documentation
	@echo "Generating preset documentation..."
	@python3 scripts/generate-preset-docs.py
	@echo "✓ Preset docs generated"

.PHONY: docs-serve
docs-serve: docs-gen ## Build and serve documentation locally
	@echo "Serving documentation at http://127.0.0.1:8000"
	@mkdocs serve

.PHONY: docs-build
docs-build: docs-gen ## Build documentation for deployment
	@echo "Building documentation..."
	@mkdocs build

# ==============================================================================
# Release
# ==============================================================================

.PHONY: release
release: ## Create a new release (runs release script)
	@bash ./scripts/release.sh

# ==============================================================================
# Preset Testing
# ==============================================================================

.PHONY: test-presets
test-presets: ## Test all presets in Docker Ubuntu
	@echo "Testing all presets in Docker Ubuntu..."
	@./scripts/test-presets-docker.sh

.PHONY: test-presets-advanced
test-presets-advanced: ## Test all presets with advanced configuration
	@echo "Running advanced preset tests..."
	@./scripts/test-presets-advanced.sh

.PHONY: test-presets-quick
test-presets-quick: ## Quick preset tests (skip slow presets)
	@echo "Running quick preset tests..."
	@./scripts/test-presets-advanced.sh --quick

.PHONY: test-preset
test-preset: ## Test specific preset locally (usage: make test-preset PRESET=docker)
	@if [ -z "$(PRESET)" ]; then \
		echo "Error: PRESET variable is required"; \
		echo "Usage: make test-preset PRESET=docker"; \
		echo "       make test-preset PRESET=postgres PARAMS='version=14'"; \
		exit 1; \
	fi
	@echo "Testing preset: $(PRESET)"
	@./scripts/test-preset-local.sh $(PRESET) $(PARAMS)
