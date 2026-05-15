#!/usr/bin/env bash
# PostToolUse hook: run goimports on edited Go files.
#
# Claude Code passes hook event JSON on stdin. We extract the edited file
# from .tool_input.file_path; if it's a .go file that exists, run
# `goimports -w` on it. Silent on the common case; only complains if
# goimports itself errors. Bypass: remove or rename this script.
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  exit 0
fi

file=$(jq -r '.tool_input.file_path // empty')

[[ -z "$file" ]] && exit 0
[[ "$file" != *.go ]] && exit 0
[[ ! -f "$file" ]] && exit 0

if command -v goimports >/dev/null 2>&1; then
  goimports -w "$file"
fi
