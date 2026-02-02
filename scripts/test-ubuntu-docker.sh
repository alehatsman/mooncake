#!/bin/bash
# Test script to run unit tests in Ubuntu Docker container
# This reproduces the GitHub Actions Ubuntu environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "==> Running tests in Ubuntu Docker container"
echo "Project root: $PROJECT_ROOT"

docker run --rm \
    -v "$PROJECT_ROOT:/workspace" \
    -w /workspace \
    golang:1.25-bookworm \
    bash -c "
        set -e
        echo '==> Go version:'
        go version
        echo ''
        echo '==> Building...'
        go build -v ./...
        echo ''
        echo '==> Running unit tests...'
        go test -v ./...
        echo ''
        echo '==> Running tests with coverage...'
        go test -coverprofile=coverage.out -covermode=atomic ./...
        echo ''
        echo '==> Running race detector...'
        go test -race -v ./...
        echo ''
        echo '==> All tests passed!'
    "
