# golangci-lint - Fast Go Linters Runner

Fast linters runner for Go that runs linters in parallel, uses caching, supports YAML configuration, and integrates seamlessly with CI/CD pipelines.

## Quick Start

```yaml
- preset: golangci-lint
```

```bash
# Run linters on project
golangci-lint run

# Run with auto-fix
golangci-lint run --fix

# Generate configuration
golangci-lint config init
```

## Features

- **Fast**: Runs linters in parallel with smart caching
- **Comprehensive**: Bundles 60+ linters including staticcheck, errcheck, govet
- **Configurable**: YAML-based configuration with preset profiles
- **CI-friendly**: Integrates with GitHub Actions, GitLab CI, CircleCI
- **Smart defaults**: Works out of the box with sensible settings
- **Auto-fix**: Automatically fixes issues when possible

## Basic Usage

```bash
# Run all enabled linters
golangci-lint run

# Run on specific directory
golangci-lint run ./pkg/...

# Run with auto-fix
golangci-lint run --fix

# List all linters
golangci-lint linters

# Run specific linters
golangci-lint run --enable=errcheck,govet

# Fast mode (fewer linters)
golangci-lint run --fast

# Verbose output
golangci-lint run -v
```

## Advanced Configuration

```yaml
- preset: golangci-lint
  with:
    state: present
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove golangci-lint |

## Platform Support

- ✅ Linux (binary install via script)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported)

## Configuration

Create `.golangci.yml` in project root:

```yaml
# .golangci.yml
run:
  timeout: 5m
  tests: true
  skip-dirs:
    - vendor
    - testdata

linters:
  enable:
    - errcheck      # Check error handling
    - govet         # Go vet
    - staticcheck   # Advanced static analysis
    - gosimple      # Simplify code
    - unused        # Find unused code
    - ineffassign   # Detect ineffectual assignments
    - misspell      # Spell checker
    - gofmt         # Format check
    - goimports     # Import management

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true

  govet:
    check-shadowing: true

  staticcheck:
    checks: ["all"]

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

## Real-World Examples

### CI/CD Integration (GitHub Actions)

```yaml
# .github/workflows/lint.yml
name: Lint
on: [push, pull_request]

jobs:
  golangci:
    name: lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
          args: --timeout=5m
```

### Pre-commit Hook

```bash
# .git/hooks/pre-commit
#!/bin/bash
golangci-lint run --fix
if [ $? -ne 0 ]; then
  echo "Linting failed. Fix issues before committing."
  exit 1
fi
```

### Minimal Configuration (Essential Linters)

```yaml
# .golangci.yml - minimal but effective
linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - gosimple

run:
  timeout: 3m
```

### Strict Configuration (All Checks)

```yaml
# .golangci.yml - maximum strictness
linters:
  enable-all: true
  disable:
    - exhaustivestruct  # Too strict
    - funlen           # Subjective
    - gochecknoglobals # Sometimes needed

run:
  timeout: 10m

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

## Agent Use

- Enforce code quality in CI/CD pipelines
- Block pull requests with linting errors
- Generate code quality reports
- Automatically fix issues in development
- Standardize code style across teams
- Detect bugs before code review

## Troubleshooting

### "context deadline exceeded" errors

Increase timeout:
```bash
golangci-lint run --timeout=10m
```

### Too many false positives

Disable specific linters:
```yaml
# .golangci.yml
linters:
  disable:
    - linter-name
```

### "Out of memory" errors

Run fewer linters or reduce concurrency:
```bash
golangci-lint run --fast
golangci-lint run --concurrency=2
```

### Ignoring specific issues

```yaml
# .golangci.yml
issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
    - text: "G404:"  # Ignore weak random generator in tests
      linters:
        - gosec
```

## Uninstall

```yaml
- preset: golangci-lint
  with:
    state: absent
```

## Resources

- Official docs: https://golangci-lint.run/
- GitHub: https://github.com/golangci/golangci-lint
- Linters list: https://golangci-lint.run/usage/linters/
- Search: "golangci-lint tutorial", "golangci-lint configuration", "golang ci best practices"
