#!/bin/bash
set -e

# Test all presets in a Docker Ubuntu environment
# Usage: ./scripts/test-presets-docker.sh [preset-name]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DOCKER_IMAGE="mooncake-test:latest"
RESULTS_DIR="${PROJECT_ROOT}/testing-output"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counter for stats
TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0

# Create results directory
mkdir -p "${RESULTS_DIR}"
REPORT_FILE="${RESULTS_DIR}/preset-test-${TIMESTAMP}.txt"

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
    log "${BLUE}ℹ $1${NC}"
}

# Build Docker image with mooncake
build_docker_image() {
    log_header "Building Docker Test Image"

    cat > "${PROJECT_ROOT}/Dockerfile.test" <<'EOF'
FROM ubuntu:22.04

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    sudo \
    systemd \
    ca-certificates \
    gnupg \
    lsb-release \
    && rm -rf /var/lib/apt/lists/*

# Create test user with sudo access
RUN useradd -m -s /bin/bash testuser && \
    echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Copy mooncake binary
COPY mooncake /usr/local/bin/mooncake
RUN chmod +x /usr/local/bin/mooncake

# Copy presets
COPY presets /usr/share/mooncake/presets

WORKDIR /home/testuser
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

# Discover all presets
discover_presets() {
    find "${PROJECT_ROOT}/presets" -name "preset.yml" | sort
}

# Get preset name from path
get_preset_name() {
    local preset_path="$1"
    basename "$(dirname "$preset_path")"
}

# Create test playbook for a preset
create_test_playbook() {
    local preset_name="$1"
    local playbook_path="$2"

    cat > "${playbook_path}" <<EOF
---
name: Test ${preset_name} preset
steps:
  - name: Run ${preset_name} preset
    preset: ${preset_name}
    register: preset_result

  - name: Show preset result
    print:
      msg: |
        Preset: ${preset_name}
        Changed: {{ preset_result.changed }}
EOF
}

# Run a preset test
test_preset() {
    local preset_path="$1"
    local preset_name=$(get_preset_name "$preset_path")

    TOTAL=$((TOTAL + 1))

    log ""
    log_info "Testing preset: ${preset_name}"

    # Create temporary test playbook
    local test_playbook="/tmp/test-${preset_name}.yml"
    create_test_playbook "$preset_name" "$test_playbook"

    # Run in Docker container
    local container_name="mooncake-test-${preset_name}-$$"
    local log_file="${RESULTS_DIR}/${preset_name}-${TIMESTAMP}.log"

    if docker run --rm \
        --name "$container_name" \
        --privileged \
        -v "${test_playbook}:/test.yml:ro" \
        "${DOCKER_IMAGE}" \
        mooncake run --config /test.yml > "$log_file" 2>&1; then

        log_success "PASSED: ${preset_name}"
        PASSED=$((PASSED + 1))

        # Show summary from log
        if grep -q "changed" "$log_file"; then
            local changed=$(grep -o "changed: [0-9]*" "$log_file" | head -1)
            log "  ${changed}"
        fi
    else
        log_error "FAILED: ${preset_name}"
        FAILED=$((FAILED + 1))

        # Show last 10 lines of error
        log "  Last 10 lines of output:"
        tail -10 "$log_file" | sed 's/^/    /'
    fi

    # Cleanup
    rm -f "$test_playbook"

    # Stop container if still running
    docker rm -f "$container_name" 2>/dev/null || true
}

# Test dry-run mode for a preset
test_preset_dryrun() {
    local preset_path="$1"
    local preset_name=$(get_preset_name "$preset_path")

    log_info "  Testing dry-run mode..."

    # Create temporary test playbook
    local test_playbook="/tmp/test-dryrun-${preset_name}.yml"
    create_test_playbook "$preset_name" "$test_playbook"

    # Run in Docker container with --dry-run
    local container_name="mooncake-dryrun-${preset_name}-$$"
    local log_file="${RESULTS_DIR}/${preset_name}-dryrun-${TIMESTAMP}.log"

    if docker run --rm \
        --name "$container_name" \
        -v "${test_playbook}:/test.yml:ro" \
        "${DOCKER_IMAGE}" \
        mooncake run --config /test.yml --dry-run > "$log_file" 2>&1; then

        log_success "  Dry-run passed"
    else
        log_warning "  Dry-run failed"
    fi

    # Cleanup
    rm -f "$test_playbook"
    docker rm -f "$container_name" 2>/dev/null || true
}

# Print summary
print_summary() {
    log ""
    log_header "Test Summary"
    log "Total presets: ${TOTAL}"
    log_success "Passed: ${PASSED}"
    log_error "Failed: ${FAILED}"

    if [ $SKIPPED -gt 0 ]; then
        log_warning "Skipped: ${SKIPPED}"
    fi

    log ""
    log_info "Full report saved to: ${REPORT_FILE}"

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

# Main execution
main() {
    local specific_preset="$1"

    log_header "Mooncake Preset Testing"
    log "Timestamp: ${TIMESTAMP}"
    log "Project: ${PROJECT_ROOT}"
    log ""

    # Build Docker image
    build_docker_image

    log ""
    log_header "Discovering Presets"

    local presets=$(discover_presets)
    local preset_count=$(echo "$presets" | wc -l)

    log_info "Found ${preset_count} presets"

    # If specific preset requested, filter
    if [ -n "$specific_preset" ]; then
        log_info "Testing only: ${specific_preset}"
        presets=$(echo "$presets" | grep "/${specific_preset}/preset.yml" || true)

        if [ -z "$presets" ]; then
            log_error "Preset not found: ${specific_preset}"
            exit 1
        fi
    fi

    log ""
    log_header "Running Tests"

    # Test each preset
    while IFS= read -r preset_path; do
        test_preset "$preset_path"

        # Also test dry-run mode
        if [ $FAILED -eq 0 ]; then
            test_preset_dryrun "$preset_path"
        fi
    done <<< "$presets"

    # Print summary and exit with appropriate code
    print_summary
}

# Cleanup on exit
cleanup() {
    log_info "Cleaning up..."
    docker rm -f $(docker ps -a -q -f name=mooncake-test) 2>/dev/null || true
}

trap cleanup EXIT

# Run main
main "$@"
