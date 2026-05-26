#!/usr/bin/env bash
# schema-check — regenerate the JSON schema into a temp file and diff
# against the committed internal/config/schema.json. Exit non-zero on
# divergence. TypeScript declarations (schema.d, mooncake.d.ts) are
# implicitly covered by ci:regen-and-check, which writes all three.
#
# Assumes the mooncake binary is built at $BIN (default out/mooncake).
set -euo pipefail

BIN="${BIN:-out/mooncake}"
TMP=".tmp/schema-check"

mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

"$BIN" schema generate --format json --output "$TMP/schema.json" --strict >/dev/null

if diff -q internal/config/schema.json "$TMP/schema.json" >/dev/null 2>&1; then
  echo "✓ Schema is up to date"
  exit 0
fi

echo "✗ Schema is out of sync!"
echo
echo "Run 'mooncake task schema-generate' to update schema."
exit 1
