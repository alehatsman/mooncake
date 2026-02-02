#!/bin/bash
# Run tests on all supported Linux distributions

set -e

DISTROS=("ubuntu-22.04" "ubuntu-20.04" "alpine-3.19" "debian-12" "fedora-39")
TEST_SUITE=${1:-smoke}
FAILED=()

echo "=========================================="
echo "Multi-Distro Testing"
echo "=========================================="
echo "Test suite: $TEST_SUITE"
echo "Distros: ${DISTROS[*]}"
echo ""

echo "Building mooncake binary for Linux..."
env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake-linux-amd64 ./cmd

for distro in "${DISTROS[@]}"; do
  echo ""
  echo "=========================================="
  echo "Testing on $distro"
  echo "=========================================="

  DOCKERFILE="testing/docker/${distro}.Dockerfile"

  if [ ! -f "$DOCKERFILE" ]; then
    echo "✗ $distro: SKIPPED (Dockerfile not found)"
    FAILED+=("$distro")
    continue
  fi

  echo "Building Docker image for $distro..."
  if ! docker build -f "$DOCKERFILE" -t "mooncake-test-${distro}" . > /dev/null 2>&1; then
    echo "✗ $distro: FAILED (Docker build failed)"
    FAILED+=("$distro")
    continue
  fi

  echo "Running $TEST_SUITE tests on $distro..."
  if docker run --rm \
    -v "$(pwd)/testing/results:/workspace/results" \
    "mooncake-test-${distro}" "$TEST_SUITE"; then
    echo "✓ $distro: PASSED"
  else
    echo "✗ $distro: FAILED"
    FAILED+=("$distro")
  fi
done

echo ""
echo "=========================================="
echo "Summary"
echo "=========================================="

PASSED=$((${#DISTROS[@]} - ${#FAILED[@]}))
echo "Tested: ${#DISTROS[@]} distros"
echo "Passed: $PASSED"
echo "Failed: ${#FAILED[@]}"

if [ ${#FAILED[@]} -eq 0 ]; then
  echo ""
  echo "✓ All tests passed!"
  exit 0
else
  echo ""
  echo "✗ Failed on: ${FAILED[*]}"
  exit 1
fi
