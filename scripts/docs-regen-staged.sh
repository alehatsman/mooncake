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

echo "docs regen (gated on action/config/docgen + documented-API-package changes)"
# The pathspecs below MUST cover every package in docgen.DefaultAPIPackages
# (internal/docgen/api.go) — each renders to dist/docs/api/<slug>.md, so a .go
# change there drifts the committed docs. They also cover the action cards,
# schema, and the docgen/schemagen generators themselves. DefaultAPIPackages
# coverage is enforced by TestDocsRegenGate_CoversAPIPackages
# (internal/docgen/api_test.go) so this list can't silently fall behind.
#
# NOTE: single-star '<pkg>/*.go' is deliberate, not '<pkg>/**/*.go'. In a git
# pathspec (no :(glob) magic) '*' matches '/', so '<pkg>/*.go' matches both
# top-level and nested files. The trailing '/' in '**/*.go' forces a
# subdirectory, which silently MISSED every top-level file (e.g. actions
# handler.go/registry.go, and all of the flat docgen/schemagen packages —
# matching zero files). Keep the single star.
gated=$(git diff --cached --name-only --diff-filter=ACMR \
  -- 'internal/actions/*.go' \
     'internal/config/*.go' \
     'internal/docgen/*.go' \
     'internal/effects/*.go' \
     'internal/events/*.go' \
     'internal/executor/*.go' \
     'internal/facts/*.go' \
     'internal/logger/*.go' \
     'internal/modules/*.go' \
     'internal/plan/*.go' \
     'internal/presets/*.go' \
     'internal/schemagen/*.go' \
     'cmd/docs.go' 'cmd/schema.go' 2>/dev/null || true)

if [ -z "$gated" ]; then
  echo "  (no documented-package / generator Go files staged — skipping)"
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
