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
	@gosec -exclude-generated -exclude=G104,G115,G117,G204,G301,G306,G702,G703,G704 ./...
	@echo ""
	@echo "Running govulncheck..."
	@govulncheck ./...

# ==============================================================================
# CI Target (matches GitHub Actions)
# ==============================================================================

.PHONY: ci
ci: lint test-race scan docs-check schema-check ## Run full CI suite (lint + test-race + scan + docs-check + schema-check)
	@echo ""
	@echo "✓ All CI checks passed!"

.PHONY: ubuntu-ci
ubuntu-ci: ## Run full CI suite in Ubuntu Docker container (cross-platform verification)
	@echo "Running CI in Ubuntu Docker container..."
	@docker run --rm -v "$$(pwd)":/workspace -w /workspace golang:1.25 bash -c " \
		set -e && \
		echo '==> Installing CI dependencies...' && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin latest >/dev/null 2>&1 && \
		go install golang.org/x/vuln/cmd/govulncheck@latest 2>/dev/null && \
		go install github.com/securego/gosec/v2/cmd/gosec@latest 2>/dev/null && \
		echo '==> Running make ci...' && \
		make ci \
	"
	@echo ""
	@echo "✓ Ubuntu CI checks passed!"

# ==============================================================================
# Documentation
# ==============================================================================

.PHONY: docs
docs: ## Build and serve documentation site
	@echo "Documentation built in site/ directory"
	pipenv run mkdocs build
	pipenv run mkdocs serve

.PHONY: docs-generate
docs-generate: build ## Generate documentation from code (actions, presets, schema, properties)
	@echo "Generating documentation from code..."
	@mkdir -p docs-next/generated
	@./out/mooncake docs generate --section all --output docs-next/generated/actions.md
	@./out/mooncake docs generate --section preset-examples --presets-dir presets --output docs-next/generated/presets.md
	@./out/mooncake docs generate --section schema --output docs-next/generated/schema.md
	@./out/mooncake docs generate --section action-properties --output docs-next/generated/properties.md
	@echo "✓ Generated documentation:"
	@echo "  - docs-next/generated/actions.md     (platform matrix, capabilities, action summaries)"
	@echo "  - docs-next/generated/presets.md     (all preset examples)"
	@echo "  - docs-next/generated/schema.md      (YAML schema reference)"
	@echo "  - docs-next/generated/properties.md  (action properties from schema.json)"

.PHONY: docs-check
docs-check: build ## Check if generated docs are up to date
	@echo "Checking if generated documentation is up to date..."
	@mkdir -p .tmp/docs-check
	@./out/mooncake docs generate --section all --output .tmp/docs-check/actions.md >/dev/null 2>&1
	@./out/mooncake docs generate --section preset-examples --presets-dir presets --output .tmp/docs-check/presets.md >/dev/null 2>&1
	@./out/mooncake docs generate --section schema --output .tmp/docs-check/schema.md >/dev/null 2>&1
	@./out/mooncake docs generate --section action-properties --output .tmp/docs-check/properties.md >/dev/null 2>&1
	@failed=0; \
	for file in actions.md presets.md schema.md properties.md; do \
		grep -v "Generated: " docs-next/generated/$$file > .tmp/docs-check/current_$$file 2>/dev/null || true; \
		grep -v "Generated: " .tmp/docs-check/$$file > .tmp/docs-check/new_$$file 2>/dev/null || true; \
		if ! diff -q .tmp/docs-check/current_$$file .tmp/docs-check/new_$$file >/dev/null 2>&1; then \
			if [ $$failed -eq 0 ]; then \
				echo "✗ Documentation is out of sync!"; \
				echo ""; \
				echo "The following files have changed:"; \
				failed=1; \
			fi; \
			echo "docs-next/generated/$$file"; \
		fi; \
	done; \
	rm -rf .tmp/docs-check; \
	if [ $$failed -eq 1 ]; then \
		echo ""; \
		echo "Run 'make docs-generate' to update documentation."; \
		exit 1; \
	else \
		echo "✓ Documentation is up to date"; \
	fi

.PHONY: docs-clean
docs-clean: ## Remove generated documentation
	@echo "Cleaning generated documentation..."
	@rm -rf docs-next/generated/
	@echo "✓ Cleaned generated docs"

# ==============================================================================
# Schema
# ==============================================================================

.PHONY: schema-generate
schema-generate: build ## Generate JSON Schema from code (internal/config/schema.json)
	@echo "Generating JSON Schema from action metadata..."
	@./out/mooncake schema generate --format json --output internal/config/schema.json --strict
	@echo "✓ Generated internal/config/schema.json"
	@echo "  Schema is embedded in binary for runtime validation"
	@echo "Generating Typescript types from JSON Schema..."
	@./out/mooncake schema generate --format typescript --output internal/config/schema.d

.PHONY: schema-check
schema-check: build ## Check if generated schema is up to date
	@echo "Checking if JSON Schema is up to date..."
	@mkdir -p .tmp/schema-check
	@./out/mooncake schema generate --format json --output .tmp/schema-check/schema.json --strict >/dev/null 2>&1
	@if diff -q internal/config/schema.json .tmp/schema-check/schema.json >/dev/null 2>&1; then \
		rm -rf .tmp/schema-check; \
		echo "✓ Schema is up to date"; \
	else \
		rm -rf .tmp/schema-check; \
		echo "✗ Schema is out of sync!"; \
		echo ""; \
		echo "Run 'make schema-generate' to update schema."; \
		exit 1; \
	fi

# ==============================================================================
# Release
# ==============================================================================

.PHONY: release
release: ## Create a new release (runs release script)
	@bash ./scripts/release.sh

