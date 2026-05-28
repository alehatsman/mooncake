#!/usr/bin/env bash
# docs-check — regenerate the dist/docs/ tree into a temp dir and diff
# it against the committed copy. Exit non-zero if anything differs.
#
# The "Generated: <timestamp>" footer is stripped before comparison so
# cosmetic mtime drift doesn't trip the check; same for gomarkdoc's
# "DO NOT EDIT" preamble.
#
# Assumes the mooncake binary is already built at $BIN (default
# out/mooncake). The caller is responsible for `mooncake task build`
# beforehand — we don't auto-build because this script also runs from
# inside ci:full.sh where the build is a separate phase.
set -euo pipefail

BIN="${BIN:-out/mooncake}"
DIST="dist/docs"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

"$BIN" docs generate --section all-into-dir --output "$TMP" >/dev/null

# Strip volatile lines (timestamps + gomarkdoc's DO-NOT-EDIT preamble
# header which embeds a version stamp) from both trees before diff.
strip_volatile() {
  local src="$1"
  local dst="$2"
  mkdir -p "$dst"
  if [ ! -d "$src" ]; then
    return
  fi
  (cd "$src" && find . -type f -print0) | while IFS= read -r -d '' rel; do
    mkdir -p "$dst/$(dirname "$rel")"
    grep -vE '^<!-- (Generated: |Version: .* \| Generated: )' "$src/$rel" > "$dst/$rel" 2>/dev/null || true
  done
}

strip_volatile "$DIST" "$TMP/current"
strip_volatile "$TMP" "$TMP/new"
# strip_volatile recursed into its own output; clean that up to keep
# diff focused on real content rather than the strip-temp twins.
rm -rf "$TMP/new/current" "$TMP/new/new"

if ! diff -r -q "$TMP/current" "$TMP/new" >/dev/null 2>&1; then
  echo "✗ Documentation is out of sync!"
  echo
  echo "Differences (current vs regenerated):"
  diff -r -q "$TMP/current" "$TMP/new" || true
  echo
  echo "Run 'mooncake task docs-generate' to update documentation."
  exit 1
fi

echo "✓ Documentation is up to date"
