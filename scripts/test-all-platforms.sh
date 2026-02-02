#!/bin/bash
# Test on all platforms (local + Docker)

set -e

NATIVE_FAILED=0
DOCKER_FAILED=0

echo "=========================================="
echo "Multi-Platform Testing (Local)"
echo "=========================================="
echo ""

echo "=========================================="
echo "Step 1: Native Platform Tests"
echo "=========================================="
echo "Running Go unit tests on current platform..."
echo ""

if go test -v ./...; then
  echo ""
  echo "✓ Native tests: PASSED"
else
  echo ""
  echo "✗ Native tests: FAILED"
  NATIVE_FAILED=1
fi

echo ""
echo "=========================================="
echo "Step 2: Linux Docker Tests"
echo "=========================================="
echo "Running smoke tests on multiple Linux distros..."
echo ""

if ./scripts/test-docker-all.sh smoke; then
  echo ""
  echo "✓ Docker tests: PASSED"
else
  echo ""
  echo "✗ Docker tests: FAILED"
  DOCKER_FAILED=1
fi

echo ""
echo "=========================================="
echo "Final Summary"
echo "=========================================="

if [ $NATIVE_FAILED -eq 0 ] && [ $DOCKER_FAILED -eq 0 ]; then
  echo "✓ Native tests: PASSED"
  echo "✓ Linux tests: PASSED"
  echo ""
  echo "✓✓✓ All platform tests passed! ✓✓✓"
  echo ""
  echo "Note: For Windows testing, push to GitHub and check Actions"
  exit 0
else
  [ $NATIVE_FAILED -eq 0 ] && echo "✓ Native tests: PASSED" || echo "✗ Native tests: FAILED"
  [ $DOCKER_FAILED -eq 0 ] && echo "✓ Linux tests: PASSED" || echo "✗ Linux tests: FAILED"
  echo ""
  echo "✗ Some tests failed"
  exit 1
fi
