#!/bin/bash
# Quick test script to run unit tests in Ubuntu Docker container
# This matches the GitHub Actions test command exactly

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "==> Running tests in Ubuntu Docker container (quick mode)"

docker run --rm \
    -v "$PROJECT_ROOT:/workspace" \
    -w /workspace \
    golang:1.25-bookworm \
    bash -c "go test -v ./..."
