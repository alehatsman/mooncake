#!/bin/bash
set -e

# Advanced preset testing with configurable test cases
# Usage: ./scripts/test-presets-advanced.sh [preset-name] [--quick] [--no-build]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DOCKER_IMAGE="mooncake-test:latest"
RESULTS_DIR="${PROJECT_ROOT}/testing-output"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
CONFIG_FILE="${SCRIPT_DIR}/preset-test-config.yml"

# Parse arguments
SPECIFIC_PRESET=""
QUICK_MODE=false
NO_BUILD=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --quick)
            QUICK_MODE=true
            shift
            ;;
        --no-build)
            NO_BUILD=true
            shift
            ;;
        *)
            SPECIFIC_PRESET="$1"
            shift
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Stats
TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0

# Create results directory
mkdir -p "${RESULTS_DIR}"
REPORT_FILE="${RESULTS_DIR}/preset-test-advanced-${TIMESTAMP}.txt"
REPORT_JSON="${RESULTS_DIR}/preset-test-advanced-${TIMESTAMP}.json"

# Initialize JSON report
echo '{"timestamp": "'${TIMESTAMP}'", "tests": []}' > "${REPORT_JSON}"

log() {
    echo -e "$1" | tee -a "${REPORT_FILE}"
}

log_header() {
    log "${BLUE}========================================${NC}"
    log "${BLUE}$1${NC}"
    log "${BLUE}========================================${NC}"
}

log_success() {
    log "${GREEN}✓ $1${NC}"
}

log_error() {
    log "${RED}✗ $1${NC}"
}

log_warning() {
    log "${YELLOW}⚠ $1${NC}"
}

log_info() {
    log "${CYAN}ℹ $1${NC}"
}

# Add result to JSON report
add_json_result() {
    local preset="$1"
    local test_case="$2"
    local status="$3"
    local duration="$4"
    local log_file="$5"

    # Use jq to append result if available, otherwise skip
    if command -v jq &> /dev/null; then
        local temp_json=$(mktemp)
        jq --arg preset "$preset" \
           --arg test_case "$test_case" \
           --arg status "$status" \
           --arg duration "$duration" \
           --arg log "$log_file" \
           '.tests += [{
               preset: $preset,
               test_case: $test_case,
               status: $status,
               duration_seconds: ($duration | tonumber),
               log_file: $log
           }]' "${REPORT_JSON}" > "$temp_json"
        mv "$temp_json" "${REPORT_JSON}"
    fi
}

# Build Docker image
build_docker_image() {
    if [ "$NO_BUILD" = true ]; then
        log_info "Skipping Docker build (--no-build)"
        return
    fi

    log_header "Building Docker Test Image"

    cat > "${PROJECT_ROOT}/Dockerfile.test" <<'EOF'
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    sudo \
    ca-certificates \
    gnupg \
    lsb-release \
    build-essential \
    software-properties-common \
    apt-transport-https \
    && rm -rf /var/lib/apt/lists/*

# Create test user with sudo access
RUN useradd -m -s /bin/bash testuser && \
    echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Copy mooncake binary
COPY mooncake /usr/local/bin/mooncake
RUN chmod +x /usr/local/bin/mooncake

# Copy presets
COPY presets /usr/share/mooncake/presets

# Set up home directory
WORKDIR /home/testuser
RUN chown -R testuser:testuser /home/testuser

USER testuser

# Verify mooncake is working
RUN mooncake help > /dev/null
EOF

    log_info "Building mooncake binary for Linux..."
    cd "${PROJECT_ROOT}"
    GOOS=linux GOARCH=amd64 go build -o mooncake cmd/*.go

    log_info "Building Docker image..."
    docker build -f Dockerfile.test -t "${DOCKER_IMAGE}" .

    rm -f Dockerfile.test
    log_success "Docker image built successfully"
}

# Generate test playbook with parameters
create_test_playbook() {
    local preset_name="$1"
    local test_case_name="$2"
    local playbook_path="$3"
    shift 3
    local params=("$@")

    cat > "${playbook_path}" <<EOF
---
name: Test ${preset_name} - ${test_case_name}
steps:
  - name: Run ${preset_name} preset
    preset:
      name: ${preset_name}
EOF

    # Add parameters if provided
    if [ ${#params[@]} -gt 0 ]; then
        echo "      with:" >> "${playbook_path}"
        for param in "${params[@]}"; do
            echo "        ${param}" >> "${playbook_path}"
        done
    fi

    cat >> "${playbook_path}" <<EOF
    register: preset_result

  - name: Show preset result
    print:
      msg: |
        Preset: ${preset_name}
        Test Case: ${test_case_name}
        Changed: {{ preset_result.changed }}
EOF
}

# Parse YAML config and extract test cases for a preset
# This is a simple parser - in production you might use yq or a proper YAML parser
get_test_cases() {
    local preset_name="$1"

    if [ ! -f "$CONFIG_FILE" ]; then
        log_warning "Config file not found: ${CONFIG_FILE}"
        echo "default"
        return
    fi

    # Simple grep-based parsing (limited but works for our config format)
    # Look for preset name in config and extract test case names
    if grep -A 100 "^  ${preset_name}:" "$CONFIG_FILE" | grep -m 1 "^  [a-z]" -B 100 | grep "    - name:" | awk '{print $3}' | grep -v "^$"; then
        return 0
    else
        echo "default"
    fi
}

# Check if preset should be skipped
should_skip_preset() {
    local preset_name="$1"

    if [ ! -f "$CONFIG_FILE" ]; then
        return 1
    fi

    # Check skip_presets list
    if grep -A 10 "^skip_presets:" "$CONFIG_FILE" | grep -q "  - ${preset_name}"; then
        return 0
    fi

    return 1
}

# Check if preset is slow
is_slow_preset() {
    local preset_name="$1"

    if [ ! -f "$CONFIG_FILE" ]; then
        return 1
    fi

    if grep -A 20 "^slow_presets:" "$CONFIG_FILE" | grep -q "  - ${preset_name}"; then
        return 0
    fi

    return 1
}

# Run a test case
run_test_case() {
    local preset_name="$1"
    local test_case="$2"

    TOTAL=$((TOTAL + 1))

    log ""
    log_info "Testing: ${preset_name} / ${test_case}"

    # Skip slow presets in quick mode
    if [ "$QUICK_MODE" = true ] && is_slow_preset "$preset_name"; then
        log_warning "  Skipped (slow preset, use without --quick to run)"
        SKIPPED=$((SKIPPED + 1))
        return
    fi

    # Create test playbook
    local test_playbook="/tmp/test-${preset_name}-${test_case}-$$.yml"
    create_test_playbook "$preset_name" "$test_case" "$test_playbook"

    # Run in Docker
    local container_name="mooncake-test-${preset_name}-${test_case}-$$"
    local log_file="${RESULTS_DIR}/${preset_name}-${test_case}-${TIMESTAMP}.log"

    local start_time=$(date +%s)

    if docker run --rm \
        --name "$container_name" \
        --privileged \
        -v "${test_playbook}:/test.yml:ro" \
        "${DOCKER_IMAGE}" \
        mooncake run --config /test.yml > "$log_file" 2>&1; then

        local end_time=$(date +%s)
        local duration=$((end_time - start_time))

        log_success "  PASSED (${duration}s)"
        PASSED=$((PASSED + 1))

        add_json_result "$preset_name" "$test_case" "passed" "$duration" "$log_file"

        # Show summary
        if grep -q "changed: [0-9]*" "$log_file"; then
            local changed=$(grep -o "changed: [0-9]*" "$log_file" | head -1)
            log "    ${changed}"
        fi
    else
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))

        log_error "  FAILED (${duration}s)"
        FAILED=$((FAILED + 1))

        add_json_result "$preset_name" "$test_case" "failed" "$duration" "$log_file"

        # Show error details
        log "    Error details (last 15 lines):"
        tail -15 "$log_file" | sed 's/^/      /'
    fi

    # Cleanup
    rm -f "$test_playbook"
    docker rm -f "$container_name" 2>/dev/null || true
}

# Test a preset
test_preset() {
    local preset_path="$1"
    local preset_name=$(basename "$(dirname "$preset_path")")

    log ""
    log_header "Preset: ${preset_name}"

    # Check if should skip
    if should_skip_preset "$preset_name"; then
        log_warning "Skipped (configured in test config)"
        SKIPPED=$((SKIPPED + 1))
        return
    fi

    # Get test cases
    local test_cases=$(get_test_cases "$preset_name")

    # Run each test case
    while IFS= read -r test_case; do
        run_test_case "$preset_name" "$test_case"
    done <<< "$test_cases"
}

# Print summary
print_summary() {
    log ""
    log_header "Test Summary"
    log "Total tests: ${TOTAL}"
    log_success "Passed: ${PASSED}"

    if [ $FAILED -gt 0 ]; then
        log_error "Failed: ${FAILED}"
    fi

    if [ $SKIPPED -gt 0 ]; then
        log_warning "Skipped: ${SKIPPED}"
    fi

    log ""
    log_info "Reports:"
    log "  Text: ${REPORT_FILE}"
    log "  JSON: ${REPORT_JSON}"

    if [ $FAILED -eq 0 ]; then
        log ""
        log_success "All tests passed! 🎉"
        return 0
    else
        log ""
        log_error "Some tests failed"
        return 1
    fi
}

# Discover presets
discover_presets() {
    find "${PROJECT_ROOT}/presets" -name "preset.yml" | sort
}

# Main
main() {
    log_header "Mooncake Advanced Preset Testing"
    log "Timestamp: ${TIMESTAMP}"
    log "Project: ${PROJECT_ROOT}"

    if [ "$QUICK_MODE" = true ]; then
        log_info "Quick mode enabled (skipping slow presets)"
    fi

    log ""

    # Build Docker image
    build_docker_image

    log ""
    log_header "Discovering Presets"

    local presets=$(discover_presets)
    local preset_count=$(echo "$presets" | wc -l)

    log_info "Found ${preset_count} presets"

    # Filter if specific preset requested
    if [ -n "$SPECIFIC_PRESET" ]; then
        log_info "Testing only: ${SPECIFIC_PRESET}"
        presets=$(echo "$presets" | grep "/${SPECIFIC_PRESET}/preset.yml" || true)

        if [ -z "$presets" ]; then
            log_error "Preset not found: ${SPECIFIC_PRESET}"
            exit 1
        fi
    fi

    log ""
    log_header "Running Tests"

    # Test each preset
    while IFS= read -r preset_path; do
        test_preset "$preset_path"
    done <<< "$presets"

    # Print summary and exit
    print_summary
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."
    docker rm -f $(docker ps -a -q -f name=mooncake-test) 2>/dev/null || true
}

trap cleanup EXIT

# Run
main
