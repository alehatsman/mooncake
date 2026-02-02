#!/bin/bash
# Run integration tests using actual mooncake binary

set -e

BINARY="./out/mooncake"
FIXTURES="./testing/fixtures/configs/integration"

# Detect Windows
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
  BINARY="./out/mooncake.exe"
fi

echo "=========================================="
echo "Integration Tests"
echo "=========================================="
echo ""

# Build binary if not exists
if [ ! -f "$BINARY" ]; then
  echo "Building mooncake binary..."
  go build -v -o "$BINARY" ./cmd
  echo ""
fi

echo "Binary: $BINARY"
echo "Fixtures: $FIXTURES"
echo ""

# Check if fixtures directory exists
if [ ! -d "$FIXTURES" ]; then
  echo "Warning: Fixtures directory not found: $FIXTURES"
  echo "Skipping integration tests"
  exit 0
fi

# Run each integration test config
PASSED=0
FAILED=0

for config in "$FIXTURES"/*.yml; do
  if [ ! -f "$config" ]; then
    echo "No integration test files found"
    exit 0
  fi

  test_name=$(basename "$config")
  echo "Running: $test_name"

  if "$BINARY" run -c "$config" > /dev/null 2>&1; then
    echo "  ✓ $test_name passed"
    ((PASSED++))
  else
    echo "  ✗ $test_name failed"
    echo "  Running with verbose output:"
    "$BINARY" run -c "$config" || true
    ((FAILED++))
  fi
  echo ""
done

echo "=========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "=========================================="

if [ $FAILED -eq 0 ]; then
  echo "✓ All integration tests passed"
  exit 0
else
  echo "✗ Some integration tests failed"
  exit 1
fi
