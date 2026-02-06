#!/usr/bin/env bash
# Mooncake Preset Test Runner
# Usage: ./test-presets.sh --os ubuntu [--preset <name>] [--artifacts <dir>]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

# Defaults
OS_NAME="ubuntu"
ARTIFACTS_DIR="/artifacts"
SPECIFIC_PRESET=""
PRESETS_DIR="/mooncake/presets"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --os)
            OS_NAME="$2"
            shift 2
            ;;
        --preset)
            SPECIFIC_PRESET="$2"
            shift 2
            ;;
        --artifacts)
            ARTIFACTS_DIR="$2"
            shift 2
            ;;
        --help)
            cat <<EOF
Mooncake Preset Test Runner

Usage: $0 [OPTIONS]

Options:
  --os <name>          OS to test (ubuntu, alpine, fedora) [default: ubuntu]
  --preset <name>      Test specific preset only
  --artifacts <dir>    Artifacts directory [default: /artifacts]
  --help               Show this help

Examples:
  $0 --os ubuntu
  $0 --os alpine --preset docker
  $0 --os fedora --artifacts /tmp/results
EOF
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

export OS_NAME

# Create artifacts directory
mkdir -p "${ARTIFACTS_DIR}"

RESULTS_JSON="${ARTIFACTS_DIR}/results.json"
SUMMARY_MD="${ARTIFACTS_DIR}/summary.md"

log_info "Mooncake Preset Test Runner"
log_info "OS: ${OS_NAME}"
log_info "Artifacts: ${ARTIFACTS_DIR}"
echo ""

# Prepare system for package installations
log_info "Preparing system package manager..."
case "${OS_NAME}" in
    ubuntu)
        apt-get update -qq > /dev/null 2>&1 || log_error "Failed to update apt"
        log_success "apt package lists updated"
        ;;
    alpine)
        apk update -q > /dev/null 2>&1 || log_error "Failed to update apk"
        log_success "apk package lists updated"
        ;;
    fedora)
        dnf check-update -q > /dev/null 2>&1 || true  # exit code 100 is normal for dnf
        log_success "dnf package lists updated"
        ;;
    *)
        log_warn "Unknown OS, skipping package manager update"
        ;;
esac
echo ""

# Discover presets
if [[ -n "${SPECIFIC_PRESET}" ]]; then
    log_info "Testing single preset: ${SPECIFIC_PRESET}"
    presets=("${SPECIFIC_PRESET}")
else
    log_info "Discovering all presets..."
    mapfile -t presets < <(discover_presets "${PRESETS_DIR}")
    log_info "Found ${#presets[@]} presets"
fi
echo ""

# Run tests
start_time=$(date +%s)
total=${#presets[@]}
passed=0
failed=0

# Trap to ensure JSON is closed on exit
cleanup_json() {
    if [[ -f "${RESULTS_JSON}" ]]; then
        # Check if JSON needs closing bracket
        if ! tail -1 "${RESULTS_JSON}" | grep -q "^\]$"; then
            echo "]" >> "${RESULTS_JSON}"
        fi
    fi
}
trap cleanup_json EXIT

echo "[" > "${RESULTS_JSON}"
first=true

for preset in "${presets[@]}"; do
    if [[ "${first}" == "false" ]]; then
        echo "," >> "${RESULTS_JSON}"
    fi
    first=false

    result=$(run_preset_test "${preset}" "${ARTIFACTS_DIR}") || true
    echo "${result}" >> "${RESULTS_JSON}"

    exit_code=$(echo "${result}" | grep -o '"exit_code":[0-9]*' | cut -d':' -f2 || echo "1")
    if [[ "${exit_code:-1}" -eq 0 ]]; then
        ((passed++)) || true
    else
        ((failed++)) || true
    fi
done

echo "]" >> "${RESULTS_JSON}"

end_time=$(date +%s)
total_duration=$((end_time - start_time))

# Generate summary
generate_summary "${RESULTS_JSON}" "${SUMMARY_MD}"

echo ""
log_info "========================================="
log_info "Test Summary (${total_duration}s)"
log_info "========================================="
cat "${SUMMARY_MD}"

# Exit status
if [[ ${failed} -gt 0 ]]; then
    echo ""
    log_error "FAILED: ${failed}/${total} presets failed"
    exit 1
else
    echo ""
    log_success "SUCCESS: All ${total} presets passed"
    exit 0
fi
