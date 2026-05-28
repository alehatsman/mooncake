#!/usr/bin/env bash
# docs-check — regenerate the dist/docs/ tree into a temp dir and diff
# it byte-for-byte against the committed copy. Exit non-zero on any
# difference.
#
# Generated output is deterministic by design (no timestamps, no
# environment-dependent values), so a plain recursive diff is enough.
# If a future emitter introduces a non-deterministic field, this script
# will flag it immediately on the next regen, which is exactly what we
# want.
#
# Assumes the mooncake binary is already built at $BIN (default
# out/mooncake). The caller is responsible for `mooncake task build`
# beforehand — we don't auto-build because this script also runs from
# inside ci/full.sh where the build is a separate phase.
set -euo pipefail

BIN="${BIN:-out/mooncake}"
DIST="dist/docs"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

"$BIN" docs generate --section all-into-dir --output "$TMP" >/dev/null

if ! diff -r -q "$DIST" "$TMP" >/dev/null 2>&1; then
  echo "✗ Documentation is out of sync!"
  echo
  echo "Differences (committed dist/docs/ vs fresh regeneration):"
  diff -r -q "$DIST" "$TMP" || true
  echo
  echo "Run 'mooncake task docs-generate' to update documentation."
  exit 1
fi

echo "✓ Documentation is up to date"
