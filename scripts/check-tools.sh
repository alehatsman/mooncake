#!/usr/bin/env bash
# check-tools — report which static-analysis tools are on PATH (or in
# GOPATH/bin) and exit non-zero when any are missing.
#
# Mirrors the dev convenience target that used to live inline in
# Taskfile.yml. Kept narrow on purpose — install-tools.sh is the
# remediation, this script is the diagnosis.
set -euo pipefail

tools=(golangci-lint govulncheck gosec gocyclo goda dupl)
missing=0
gopath_bin="$(go env GOPATH)/bin"

for tool in "${tools[@]}"; do
  if command -v "$tool" >/dev/null 2>&1 || [ -x "$gopath_bin/$tool" ]; then
    printf "  ✓ %s\n" "$tool"
  else
    printf "  ✗ %s — MISSING\n" "$tool"
    missing=$((missing + 1))
  fi
done

if [ "$missing" -gt 0 ]; then
  echo
  echo "$missing tool(s) missing. Run: mooncake task install-tools"
  exit 1
fi
