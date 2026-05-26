#!/usr/bin/env bash
# docs-check — regenerate docs into a temp dir and diff against the
# committed copies in docs-next/generated/. Exit non-zero if any
# differ. The "Generated: <timestamp>" line is stripped before diff
# so cosmetic mtime drift doesn't trip the check.
#
# Assumes the mooncake binary is already built at $BIN (default
# out/mooncake). The caller is responsible for `mooncake task build`
# beforehand — we don't auto-build because this script also runs from
# inside ci:full.sh where the build is a separate phase.
set -euo pipefail

BIN="${BIN:-out/mooncake}"
TMP=".tmp/docs-check"

mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

"$BIN" docs generate --section all                --output "$TMP/actions.md"    >/dev/null
"$BIN" docs generate --section schema             --output "$TMP/schema.md"     >/dev/null
"$BIN" docs generate --section action-properties  --output "$TMP/properties.md" >/dev/null

failed=0
for file in actions.md schema.md properties.md; do
  grep -v "Generated: " "docs-next/generated/$file" > "$TMP/current_$file" 2>/dev/null || true
  grep -v "Generated: " "$TMP/$file"               > "$TMP/new_$file"     2>/dev/null || true
  if ! diff -q "$TMP/current_$file" "$TMP/new_$file" >/dev/null 2>&1; then
    if [ $failed -eq 0 ]; then
      echo "✗ Documentation is out of sync!"
      echo
      echo "The following files have changed:"
      failed=1
    fi
    echo "  docs-next/generated/$file"
  fi
done

if [ $failed -eq 1 ]; then
  echo
  echo "Run 'mooncake task docs-generate' to update documentation."
  exit 1
fi

echo "✓ Documentation is up to date"
