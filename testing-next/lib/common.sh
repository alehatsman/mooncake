#!/usr/bin/env bash
# Common functions for Mooncake testing

set -euo pipefail

# Colors
export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export NC='\033[0m'

# Logging functions (output to stderr to not interfere with JSON output)
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*" >&2
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*" >&2
}

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $*" >&2
}

# Discover presets from filesystem
discover_presets() {
    local presets_dir="${1:-/mooncake/presets}"
    local presets=()

    if [[ ! -d "${presets_dir}" ]]; then
        log_error "Presets directory not found: ${presets_dir}"
        return 1
    fi

    while IFS= read -r preset_path; do
        local preset_name
        preset_name=$(basename "$(dirname "${preset_path}")")
        presets+=("${preset_name}")
    done < <(find "${presets_dir}" -mindepth 2 -maxdepth 2 -name "preset.yml" -type f | sort)

    if [[ ${#presets[@]} -eq 0 ]]; then
        log_error "No presets found in ${presets_dir}"
        return 1
    fi

    printf '%s\n' "${presets[@]}" | sort
}

# Run a single preset test
run_preset_test() {
    local preset_name="$1"
    local artifacts_dir="${2:-/artifacts}"
    local start_time end_time duration exit_code
    local output_file="${artifacts_dir}/${preset_name}.log"

    log_test "Running preset: ${preset_name}"

    start_time=$(date +%s)

    set +e
    # Only capture stderr (errors), discard stdout (READMEs, verbose output)
    if mooncake presets install "${preset_name}" 2> "${output_file}" > /dev/null; then
        exit_code=0
        log_success "${preset_name}"
    else
        exit_code=$?
        log_error "${preset_name} (exit code: ${exit_code})"
    fi
    set -e

    end_time=$(date +%s)
    duration=$((end_time - start_time))

    # Return JSON result (single line)
    local error_snippet=""
    if [[ ${exit_code} -ne 0 ]]; then
        error_snippet=$(tail -n 50 "${output_file}" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
    fi

    cat <<EOF
{"preset":"${preset_name}","exit_code":${exit_code},"start_time":${start_time},"end_time":${end_time},"duration":${duration},"log":"${output_file}","error":"${error_snippet}"}
EOF
}

# Generate test summary
generate_summary() {
    local results_file="$1"
    local summary_file="$2"
    local total passed failed

    # Parse results (without jq, using grep/wc)
    total=$(grep -o '"preset"' "${results_file}" | wc -l | tr -d ' ')
    passed=$(grep -o '"exit_code":0' "${results_file}" | wc -l | tr -d ' ')
    failed=$((total - passed))

    local success_rate=0
    if [[ ${total} -gt 0 ]]; then
        success_rate=$(awk "BEGIN {printf \"%.1f\", (${passed}/${total})*100}")
    fi

    cat > "${summary_file}" <<EOF
# Mooncake Test Results

**Date**: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
**OS**: ${OS_NAME:-unknown}

## Summary

- **Total**: ${total} tests
- **Passed**: ${passed} ✅
- **Failed**: ${failed} ❌
- **Success Rate**: ${success_rate}%

## Failed Tests

EOF

    if [[ ${failed} -eq 0 ]]; then
        echo "None! 🎉" >> "${summary_file}"
    else
        grep -B1 '"exit_code":[^0]' "${results_file}" | \
            grep '"preset"' | \
            sed 's/.*"preset":"\([^"]*\)".*/- `\1`/' >> "${summary_file}" || true
    fi

    echo -e "\n---\nFull results: \`${results_file}\`" >> "${summary_file}"
}

# Export functions for use in subshells
export -f log_info log_success log_error log_warn log_test
export -f discover_presets run_preset_test generate_summary
