# Test Scripts

This directory contains scripts for running tests in various environments.

## Ubuntu Docker Testing

These scripts reproduce the GitHub Actions Ubuntu CI environment locally:

### Quick Test (matches CI exactly)

```bash
# Run the exact command used in GitHub Actions
make test-ubuntu-docker

# Or run the script directly
./scripts/test-ubuntu-quick.sh
```

This runs: `go test -v ./...` inside a `golang:1.25-bookworm` Docker container.

### Full Test Suite

```bash
# Run all test variations (unit, coverage, race detector)
make test-ubuntu-docker-full

# Or run the script directly
./scripts/test-ubuntu-docker.sh
```

This runs:
- `go build -v ./...` - Build verification
- `go test -v ./...` - Unit tests
- `go test -coverprofile=coverage.out -covermode=atomic ./...` - Coverage
- `go test -race -v ./...` - Race detector

## Local Testing

```bash
# Quick unit tests
make test

# With race detector
make test-race

# Full CI suite (lint + test-race + scan)
make ci
```

## Requirements

- Docker installed and running
- Scripts are executable (`chmod +x scripts/*.sh`)

## Troubleshooting

### Test failures only in CI

If tests pass locally but fail in GitHub Actions:

1. Run `make test-ubuntu-docker` to reproduce the CI environment
2. Check for platform-specific issues (file paths, permissions)
3. Run with race detector: `make test-race`

### Docker not pulling image

The scripts use `golang:1.25-bookworm` which matches the CI. If the image doesn't exist:

```bash
docker pull golang:1.25-bookworm
```

### Permission issues

Ensure scripts are executable:

```bash
chmod +x scripts/test-ubuntu-quick.sh
chmod +x scripts/test-ubuntu-docker.sh
```
