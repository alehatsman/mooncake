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
# Preset Testing (see testing-next/ directory)
# ==============================================================================

.PHONY: test-presets-help
test-presets-help: ## Show preset testing help
	@echo "Preset testing has moved to testing-next/"
	@echo ""
	@echo "Quick start:"
	@echo "  cd testing-next && make help"
	@echo ""
	@echo "Common commands:"
	@echo "  cd testing-next && make test-ubuntu          # Test all presets (native arch)"
	@echo "  cd testing-next && make test-preset PRESET=jq  # Test single preset"
	@echo "  cd testing-next && make clean-all            # Cleanup"

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
ci: lint test-race scan docs-check ## Run full CI suite (lint + test-race + scan + docs-check)
	@echo ""
	@echo "✓ All CI checks passed!"

# ==============================================================================
# Documentation
# ==============================================================================

.PHONY: docs
docs: ## Build and serve documentation site
	@echo "Documentation built in site/ directory"
	pipenv run mkdocs build
	pipenv run mkdocs serve

.PHONY: docs-generate
docs-generate: build ## Generate documentation from code (actions, presets, schema)
	@echo "Generating documentation from code..."
	@mkdir -p docs-next/generated
	@./out/mooncake docs generate --section all --output docs-next/generated/actions.md
	@./out/mooncake docs generate --section preset-examples --presets-dir presets --output docs-next/generated/presets.md
	@./out/mooncake docs generate --section schema --output docs-next/generated/schema.md
	@echo "✓ Generated documentation:"
	@echo "  - docs-next/generated/actions.md  (platform matrix, capabilities, action summaries)"
	@echo "  - docs-next/generated/presets.md  (all preset examples)"
	@echo "  - docs-next/generated/schema.md   (YAML schema reference)"

.PHONY: docs-check
docs-check: docs-generate ## Check if generated docs are up to date
	@echo "Checking if generated documentation is up to date..."
	@if git diff --quiet docs-next/generated/; then \
		echo "✓ Documentation is up to date"; \
	else \
		echo "✗ Documentation is out of sync!"; \
		echo ""; \
		echo "The following files have changed:"; \
		git diff --name-only docs-next/generated/; \
		echo ""; \
		echo "Run 'make docs-generate' to update documentation."; \
		exit 1; \
	fi

.PHONY: docs-clean
docs-clean: ## Remove generated documentation
	@echo "Cleaning generated documentation..."
	@rm -rf docs-next/generated/
	@echo "✓ Cleaned generated docs"

# ==============================================================================
# Release
# ==============================================================================

.PHONY: release
release: ## Create a new release (runs release script)
	@bash ./scripts/release.sh

