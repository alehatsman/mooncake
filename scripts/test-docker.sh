#!/bin/bash
# Usage: test-docker.sh <distro> [test-suite]
# Example: test-docker.sh ubuntu-22.04 smoke

set -e

DISTRO=$1
TEST_SUITE=${2:-smoke}

if [ -z "$DISTRO" ]; then
  echo "Usage: $0 <distro> [test-suite]"
  echo ""
  echo "Available distros:"
  echo "  - ubuntu-22.04"
  echo "  - ubuntu-20.04"
  echo "  - alpine-3.19"
  echo "  - debian-12"
  echo "  - fedora-39"
  echo ""
  echo "Available test suites: smoke, integration, all"
  exit 1
fi

DOCKERFILE="testing/docker/${DISTRO}.Dockerfile"

if [ ! -f "$DOCKERFILE" ]; then
  echo "Error: Dockerfile not found: $DOCKERFILE"
  exit 1
fi

echo "=========================================="
echo "Testing on $DISTRO"
echo "=========================================="

echo "Building mooncake binary for Linux..."
env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake-linux-amd64 ./cmd

echo ""
echo "Building Docker image for $DISTRO..."
docker build -f "$DOCKERFILE" -t "mooncake-test-${DISTRO}" .

echo ""
echo "Running $TEST_SUITE tests on $DISTRO..."
docker run --rm \
  -v "$(pwd)/testing/results:/workspace/results" \
  "mooncake-test-${DISTRO}" "$TEST_SUITE"

echo ""
echo "✓ Tests passed on $DISTRO"
