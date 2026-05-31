#!/usr/bin/env bash
# docs-regen-staged.sh — mooncake-specific tail of the fast pre-commit gate.
#
# Regenerate dist/docs/ + the JSON/TS schema and auto-stage them, but only when
# the staged changes touch the generators' inputs (action metadata, config
# types, docgen/schemagen, the docs/schema CLI commands). The generic fast
# checks (vet + gofmt + ai-lint + budget) come from the go-quality module
# (goq/ci-fast); this is the bit that's unique to mooncake.
#
# First failure stops the pipeline.
set -euo pipefail

BIN="${BIN:-out/mooncake}"

cd "$(git rev-parse --show-toplevel)"

echo "docs regen (gated on action/config/docgen changes)"
gated=$(git diff --cached --name-only --diff-filter=ACMR \
  -- 'internal/actions/**/*.go' \
     'internal/config/*.go' \
     'internal/docgen/**/*.go' \
     'internal/schemagen/**/*.go' \
     'cmd/docs.go' 'cmd/schema.go' 2>/dev/null || true)

if [ -z "$gated" ]; then
  echo "  (no action/config/docgen Go files staged — skipping)"
  exit 0
fi

if ! go build -o "$BIN" ./cmd >/dev/null 2>&1; then
  echo "  ✗ build failed — fix before committing." >&2
  exit 1
fi

rm -rf dist/docs && mkdir -p dist/docs
if ! "$BIN" docs generate --section all-into-dir --output dist/docs >/dev/null 2>&1; then
  echo "  ✗ docs generate failed — fix the generator before committing." >&2
  exit 1
fi

if ! "$BIN" schema generate --format json       --output internal/config/schema.json --strict >/dev/null 2>&1 || \
   ! "$BIN" schema generate --format typescript --output internal/config/schema.d            >/dev/null 2>&1 || \
   ! "$BIN" schema generate --format typescript --output mooncake.d.ts                       >/dev/null 2>&1; then
  echo "  ✗ schema generate failed — fix the generator before committing." >&2
  exit 1
fi

changed=$(git diff --name-only -- \
  dist/docs/ \
  internal/config/schema.json \
  internal/config/schema.d \
  mooncake.d.ts)

if [ -z "$changed" ]; then
  echo "  ✓ generated docs + schema already in sync"
  exit 0
fi

echo "$changed" | sed 's/^/  ↑ auto-staging: /'
git add -- \
  dist/docs/ \
  internal/config/schema.json \
  internal/config/schema.d \
  mooncake.d.ts
