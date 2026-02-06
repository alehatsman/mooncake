#!/bin/bash
set -e

# Quick local preset testing (runs on host, not in Docker)
# Usage: ./scripts/test-preset-local.sh <preset-name> [parameter=value ...]
#
# Examples:
#   ./scripts/test-preset-local.sh docker
#   ./scripts/test-preset-local.sh postgres version=14
#   ./scripts/test-preset-local.sh ollama service=true pull='["llama3.1"]'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log() {
    echo -e "$1"
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

log_header() {
    log "${BLUE}========================================${NC}"
    log "${BLUE}$1${NC}"
    log "${BLUE}========================================${NC}"
}

usage() {
    cat <<EOF
Usage: $0 <preset-name> [parameter=value ...]

Quick local testing of Mooncake presets (runs on host, not Docker).

Arguments:
  preset-name     Name of the preset to test (required)
  parameter=value Optional parameters to pass to preset

Options:
  --dry-run       Run in dry-run mode
  --verbose       Show verbose output
  --help          Show this help

Examples:
  # Test docker preset with defaults
  $0 docker

  # Test postgres with specific version
  $0 postgres version=14

  # Test ollama with service enabled
  $0 ollama service=true

  # Test with multiple parameters
  $0 docker install_compose=false install_buildx=false

  # Dry-run mode
  $0 --dry-run ollama

Available presets:
EOF
    # List available presets
    find "${PROJECT_ROOT}/presets" -name "preset.yml" -exec dirname {} \; | xargs -n1 basename | sort | sed 's/^/  - /'
}

# Parse arguments
if [ $# -eq 0 ] || [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    usage
    exit 0
fi

PRESET_NAME=""
PARAMS=()
DRY_RUN=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            if [ -z "$PRESET_NAME" ]; then
                PRESET_NAME="$1"
            else
                PARAMS+=("$1")
            fi
            shift
            ;;
    esac
done

if [ -z "$PRESET_NAME" ]; then
    log_error "Preset name is required"
    echo ""
    usage
    exit 1
fi

# Check if preset exists
PRESET_DIR="${PROJECT_ROOT}/presets/${PRESET_NAME}"
if [ ! -d "$PRESET_DIR" ]; then
    log_error "Preset not found: ${PRESET_NAME}"
    log_info "Available presets:"
    find "${PROJECT_ROOT}/presets" -name "preset.yml" -exec dirname {} \; | xargs -n1 basename | sort | sed 's/^/  - /'
    exit 1
fi

# Build mooncake if needed
log_header "Building Mooncake"

if [ ! -f "${PROJECT_ROOT}/mooncake" ] || [ "${PROJECT_ROOT}/cmd/mooncake.go" -nt "${PROJECT_ROOT}/mooncake" ]; then
    log_info "Building mooncake binary..."
    cd "${PROJECT_ROOT}"
    go build -o mooncake cmd/*.go
    log_success "Build complete"
else
    log_info "Binary is up-to-date"
fi

# Create temporary test playbook
TEMP_DIR=$(mktemp -d)
PLAYBOOK="${TEMP_DIR}/test-${PRESET_NAME}.yml"

log ""
log_header "Creating Test Playbook"
log_info "Preset: ${PRESET_NAME}"

# Start playbook
cat > "${PLAYBOOK}" <<EOF
---
name: Test ${PRESET_NAME} preset (local)
steps:
  - name: Run ${PRESET_NAME} preset
    preset:
      name: ${PRESET_NAME}
EOF

# Add parameters if provided
if [ ${#PARAMS[@]} -gt 0 ]; then
    log_info "Parameters:"
    echo "      with:" >> "${PLAYBOOK}"

    for param in "${PARAMS[@]}"; do
        # Parse parameter (key=value)
        if [[ "$param" =~ ^([^=]+)=(.+)$ ]]; then
            key="${BASH_REMATCH[1]}"
            value="${BASH_REMATCH[2]}"

            log "  - ${key}: ${value}"

            # Handle boolean values
            if [ "$value" = "true" ] || [ "$value" = "false" ]; then
                echo "        ${key}: ${value}" >> "${PLAYBOOK}"
            # Handle array values (JSON/YAML syntax)
            elif [[ "$value" =~ ^\[.*\]$ ]]; then
                echo "        ${key}: ${value}" >> "${PLAYBOOK}"
            # Handle already-quoted strings (strip outer quotes, re-quote properly)
            elif [[ "$value" =~ ^\".*\"$ ]]; then
                echo "        ${key}: ${value}" >> "${PLAYBOOK}"
            # String values - always quote for YAML safety
            else
                echo "        ${key}: \"${value}\"" >> "${PLAYBOOK}"
            fi
        else
            log_warning "Invalid parameter format: ${param} (expected key=value)"
        fi
    done
else
    log_info "Using default parameters"
fi

# Add result registration and display
cat >> "${PLAYBOOK}" <<'EOF'
    register: preset_result

  - name: Display result
    print:
      msg: |
        ✓ Preset execution completed
        Changed: {{ preset_result.changed }}
EOF

# Show playbook if verbose
if [ "$VERBOSE" = true ]; then
    log ""
    log_info "Generated playbook:"
    cat "${PLAYBOOK}" | sed 's/^/  /'
fi

# Run the playbook
log ""
log_header "Running Preset"

MOONCAKE_CMD="${PROJECT_ROOT}/mooncake run --config ${PLAYBOOK}"

if [ "$DRY_RUN" = true ]; then
    MOONCAKE_CMD="${MOONCAKE_CMD} --dry-run"
    log_info "Mode: Dry-run"
else
    log_info "Mode: Execute"
fi

if [ "$VERBOSE" = true ]; then
    MOONCAKE_CMD="${MOONCAKE_CMD} --log-level debug"
fi

log ""
log_info "Command: ${MOONCAKE_CMD}"
log ""

# Execute
if eval "$MOONCAKE_CMD"; then
    EXIT_CODE=0
    log ""
    log_success "Test passed"
else
    EXIT_CODE=$?
    log ""
    log_error "Test failed (exit code: ${EXIT_CODE})"
fi

# Cleanup
rm -rf "${TEMP_DIR}"

log ""
log_info "Test playbook: ${PLAYBOOK} (cleaned up)"

exit $EXIT_CODE
