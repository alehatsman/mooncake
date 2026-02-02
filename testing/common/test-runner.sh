#!/bin/bash
set -e -o pipefail

TEST_SUITE=${1:-smoke}
RESULTS_DIR=${RESULTS_DIR:-/workspace/results}
TEST_EXIT_CODE=0

mkdir -p "$RESULTS_DIR"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

run_smoke_tests() {
    log_info "Running smoke tests..."

    local test_dir="/fixtures/configs/smoke"
    local passed=0
    local failed=0

    if [ ! -d "$test_dir" ]; then
        log_warning "Smoke test directory not found: $test_dir"
        return 0
    fi

    # Check if any yml files exist
    shopt -s nullglob
    local test_files=("$test_dir"/*.yml)
    log_info "Found ${#test_files[@]} smoke test files"

    if [ ${#test_files[@]} -eq 0 ]; then
        log_warning "No smoke test files found"
        return 0
    fi

    for test_file in "${test_files[@]}"; do
        local test_name=$(basename "$test_file")
        log_info "Running: $test_name"

        if mooncake run -c "$test_file" > "$RESULTS_DIR/smoke-${test_name}.log" 2>&1; then
            echo "  ✓ $test_name passed"
            passed=$((passed + 1))
        else
            log_error "✗ $test_name failed"
            cat "$RESULTS_DIR/smoke-${test_name}.log"
            failed=$((failed + 1))
            TEST_EXIT_CODE=1
        fi
    done

    log_info "Smoke tests: $passed passed, $failed failed"
}

run_integration_tests() {
    log_info "Running integration tests..."

    local test_dir="/fixtures/configs/integration"
    local passed=0
    local failed=0

    if [ ! -d "$test_dir" ]; then
        log_warning "Integration test directory not found: $test_dir"
        return 0
    fi

    # Check if any yml files exist
    shopt -s nullglob
    local test_files=("$test_dir"/*.yml)
    if [ ${#test_files[@]} -eq 0 ]; then
        log_warning "No integration test files found"
        return 0
    fi

    for test_file in "${test_files[@]}"; do
        local test_name=$(basename "$test_file")
        log_info "Running: $test_name"

        if mooncake run -c "$test_file" > "$RESULTS_DIR/integration-${test_name}.log" 2>&1; then
            echo "  ✓ $test_name passed"
            passed=$((passed + 1))
        else
            log_error "✗ $test_name failed"
            cat "$RESULTS_DIR/integration-${test_name}.log"
            failed=$((failed + 1))
            TEST_EXIT_CODE=1
        fi
    done

    log_info "Integration tests: $passed passed, $failed failed"
}

# Verify mooncake is installed
if ! command -v mooncake &> /dev/null; then
    log_error "mooncake binary not found in PATH"
    exit 1
fi

log_info "Mooncake installed at: $(which mooncake)"
log_info "Test suite: $TEST_SUITE"
log_info "Results directory: $RESULTS_DIR"

case "$TEST_SUITE" in
  smoke)
    run_smoke_tests
    ;;
  integration)
    run_integration_tests
    ;;
  all)
    run_smoke_tests
    run_integration_tests
    ;;
  *)
    log_error "Unknown test suite: $TEST_SUITE"
    log_info "Available suites: smoke, integration, all"
    exit 1
    ;;
esac

# Exit with appropriate code
if [ $TEST_EXIT_CODE -eq 0 ]; then
    log_info "All tests passed!"
else
    log_error "Some tests failed"
fi

exit $TEST_EXIT_CODE
