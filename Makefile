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
lint: ## Run golangci-lint on the full codebase
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

.PHONY: lint-new
lint-new: ## Run golangci-lint on lines changed since HEAD~1 (refactor-friendly)
	@echo "Running golangci-lint (changes only)..."
	@golangci-lint run --new-from-rev=HEAD~1 ./...

.PHONY: lint-fix
lint-fix: ## Auto-fix what golangci-lint can fix (misspell, unconvert, gocritic, staticcheck QF/S, revive)
	@echo "Running golangci-lint --fix..."
	@golangci-lint run --fix ./...

.PHONY: fmt
fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -s -w .
	@echo "✓ Code formatted"

.PHONY: scan
scan: lint ## Run security scans (gosec via golangci-lint + govulncheck)
	# gosec runs as part of `make lint` via golangci-lint; excludes live in
	# .golangci.yml under linters.settings.gosec.excludes (single source).
	@echo "Running govulncheck..."
	@govulncheck ./...

# ==============================================================================
# Agent DX — focused, sub-second feedback
# ==============================================================================
#
# Package-scoped targets accept PKG=relative/path (no ./ prefix).
# Example: `make check-pkg PKG=internal/apply` runs build+test+lint against
# just that package — much faster than `make ci` for tight edit loops.

.PHONY: build-pkg
build-pkg: ## Build a single package — usage: make build-pkg PKG=internal/apply
	@test -n "$(PKG)" || { echo "usage: make build-pkg PKG=internal/foo" >&2; exit 2; }
	@go build ./$(PKG)/...

.PHONY: test-pkg
test-pkg: ## Test a single package with -race — usage: make test-pkg PKG=internal/apply
	@test -n "$(PKG)" || { echo "usage: make test-pkg PKG=internal/foo" >&2; exit 2; }
	@go test -race -count=1 ./$(PKG)/...

.PHONY: test-fn
test-fn: ## Run a single test function — usage: make test-fn FN=TestName PKG=internal/apply
	@test -n "$(FN)" || { echo "usage: make test-fn FN=TestName PKG=internal/foo" >&2; exit 2; }
	@test -n "$(PKG)" || { echo "usage: make test-fn FN=TestName PKG=internal/foo" >&2; exit 2; }
	@go test -race -count=1 -run '$(FN)' -v ./$(PKG)/...

.PHONY: lint-pkg
lint-pkg: ## Lint a single package — usage: make lint-pkg PKG=internal/apply
	@test -n "$(PKG)" || { echo "usage: make lint-pkg PKG=internal/foo" >&2; exit 2; }
	@golangci-lint run ./$(PKG)/...

.PHONY: check-pkg
check-pkg: build-pkg test-pkg lint-pkg ## Build + test + lint a single package — usage: make check-pkg PKG=internal/apply
	@echo "✓ check-pkg $(PKG)"

# ------------------------------------------------------------------------------
# gopls-backed structural lookups — collapse grep+Read cycles
# ------------------------------------------------------------------------------
#
# Typical flow: `make sym Q='Runner'` → pick a hit's file:line:col →
# `make refs LOC=that:loc:here`. `make doc` works on plain names.

.PHONY: sym
sym: ## Fuzzy workspace symbol search — usage: make sym Q='Runner'
	@test -n "$(Q)" || { echo "usage: make sym Q='SymbolName'" >&2; exit 2; }
	@gopls workspace_symbol "$(Q)"

.PHONY: doc
doc: ## Print docs for a symbol — usage: make doc SYM=fmt.Sprintf
	@test -n "$(SYM)" || { echo "usage: make doc SYM=pkg.Symbol" >&2; exit 2; }
	@go doc "$(SYM)"

.PHONY: refs
refs: ## Find references — usage: make refs LOC=internal/apply/runner.go:12:6
	@test -n "$(LOC)" || { echo "usage: make refs LOC=file:line:col (find via 'make sym')" >&2; exit 2; }
	@gopls references "$(LOC)"

.PHONY: callers
callers: ## Show call hierarchy — usage: make callers LOC=internal/apply/runner.go:12:6
	@test -n "$(LOC)" || { echo "usage: make callers LOC=file:line:col" >&2; exit 2; }
	@gopls call_hierarchy "$(LOC)"

.PHONY: impl
impl: ## Find interface implementations — usage: make impl LOC=internal/actions/interfaces.go:15:6
	@test -n "$(LOC)" || { echo "usage: make impl LOC=file:line:col" >&2; exit 2; }
	@gopls implementation "$(LOC)"

.PHONY: agent-help
agent-help: ## Agent-focused shortcut reference (see also: AGENT.md)
	@cat AGENT.md 2>/dev/null || echo "AGENT.md not found"

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
	@docker run --rm -v "$$(pwd)":/workspace -w /workspace golang:1.26.3 bash -c " \
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
docs-generate: build ## Generate documentation from code (actions, schema, properties)
	@echo "Generating documentation from code..."
	@mkdir -p docs-next/generated
	@./out/mooncake docs generate --section all --output docs-next/generated/actions.md
	@./out/mooncake docs generate --section schema --output docs-next/generated/schema.md
	@./out/mooncake docs generate --section action-properties --output docs-next/generated/properties.md
	@echo "✓ Generated documentation:"
	@echo "  - docs-next/generated/actions.md     (platform matrix, capabilities, action summaries)"
	@echo "  - docs-next/generated/schema.md      (YAML schema reference)"
	@echo "  - docs-next/generated/properties.md  (action properties from schema.json)"

.PHONY: docs-check
docs-check: build ## Check if generated docs are up to date
	@echo "Checking if generated documentation is up to date..."
	@mkdir -p .tmp/docs-check
	@./out/mooncake docs generate --section all --output .tmp/docs-check/actions.md >/dev/null 2>&1
	@./out/mooncake docs generate --section schema --output .tmp/docs-check/schema.md >/dev/null 2>&1
	@./out/mooncake docs generate --section action-properties --output .tmp/docs-check/properties.md >/dev/null 2>&1
	@failed=0; \
	for file in actions.md schema.md properties.md; do \
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
	@./out/mooncake schema generate --format typescript --output mooncake.d.ts

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
# Architecture
# ==============================================================================

.PHONY: arch-snapshot
arch-snapshot: ## Generate docs-working/ARCH_SNAPSHOT.md (package graph + metrics for LLM review)
	@bash ./scripts/arch-snapshot.sh

.PHONY: budget-status
budget-status: ## Show current state of the three CLAUDE.md soft caps (handler LOC, gocyclo, Step fields)
	@bash ./scripts/budget-status.sh

.PHONY: arch-tools
arch-tools: ## Install optional analyzers used by arch-snapshot (gocyclo, goda)
	@echo "Installing gocyclo..."
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@echo "Installing goda..."
	@go install github.com/loov/goda@latest
	@echo "✓ Done. Re-run 'make arch-snapshot' to pick them up."

# ==============================================================================
# Release
# ==============================================================================

.PHONY: release
release: ## Create a new release (runs release script)
	@bash ./scripts/release.sh

