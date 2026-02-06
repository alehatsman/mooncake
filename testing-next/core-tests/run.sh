#!/usr/bin/env bash
# Mooncake Core Functionality Tests
# Tests mooncake binary itself (not presets)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../lib/common.sh"

ARTIFACTS_DIR="${1:-/artifacts/core-tests}"
mkdir -p "${ARTIFACTS_DIR}"

test_count=0
passed=0
failed=0

run_test() {
    local test_name="$1"
    local test_cmd="$2"

    ((test_count++))
    log_test "${test_name}"

    if eval "${test_cmd}" > "${ARTIFACTS_DIR}/${test_name}.log" 2>&1; then
        log_success "${test_name}"
        ((passed++))
        return 0
    else
        log_error "${test_name}"
        ((failed++))
        return 1
    fi
}

log_info "Running Mooncake Core Tests"
echo ""

# Test 1: Binary exists and is executable
run_test "binary-exists" "which mooncake"

# Test 2: Version command works
run_test "version-command" "mooncake --version"

# Test 3: Help command works
run_test "help-command" "mooncake --help"

# Test 4: Facts command works
run_test "facts-command" "mooncake facts"

# Test 5: Presets list works
run_test "presets-list" "mooncake presets list"

# Test 6: Run simple config
cat > /tmp/test-simple.yml <<'EOF'
steps:
  - name: Test print action
    print:
      msg: "Hello from Mooncake"
EOF
run_test "run-simple-config" "mooncake run /tmp/test-simple.yml"

# Test 7: Dry-run mode
run_test "dry-run-mode" "mooncake run --dry-run /tmp/test-simple.yml"

echo ""
log_info "========================================="
log_info "Core Tests Summary"
log_info "========================================="
log_info "Total: ${test_count}"
log_info "Passed: ${passed} ✅"
log_info "Failed: ${failed} ❌"

if [[ ${failed} -gt 0 ]]; then
    exit 1
fi

exit 0
