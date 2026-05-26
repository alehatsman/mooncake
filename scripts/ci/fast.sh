#!/usr/bin/env bash
# ci/fast.sh — pre-commit gate. Cheap checks only (<5s on warm cache).
#
#   [1/5] go vet
#   [2/5] gofmt on staged Go files
#   [3/5] ai-lint on staged Go files
#   [4/5] CLAUDE.md soft-cap budget
#   [5/5] docs + schema regen (gated on action/config/docgen staged changes)
#
# First failure stops the pipeline.
set -euo pipefail

BIN="${BIN:-out/mooncake}"
PKG="${PKG:-./...}"

cd "$(git rev-parse --show-toplevel)"

# ── [1/5] go vet ───────────────────────────────────────────────────────
echo "[1/5] go vet"
go vet "$PKG"

# ── [2/5] gofmt on staged Go files ────────────────────────────────────
echo "[2/5] gofmt (staged Go files)"
staged=$(git diff --cached --name-only --diff-filter=ACMR -- '*.go' || true)
if [ -z "$staged" ]; then
  echo "  (no staged Go files)"
else
  bad=$(echo "$staged" | xargs gofmt -l 2>/dev/null || true)
  if [ -n "$bad" ]; then
    echo "  ✗ gofmt would change:" >&2
    echo "$bad" | sed 's/^/    /' >&2
    echo "  fix with: gofmt -w <files>" >&2
    exit 1
  fi
  echo "  ✓ all staged files formatted"
fi

# ── [3/5] ai-lint on staged Go files ──────────────────────────────────
echo "[3/5] ai-lint (staged Go files)"
bash ./scripts/ai-lint.sh

# ── [4/5] soft-cap budget ─────────────────────────────────────────────
echo "[4/5] CLAUDE.md soft-cap budget"
bash ./scripts/budget-status.sh | sed 's/^/  /'

# ── [5/5] docs + schema regen (gated) ─────────────────────────────────
echo "[5/5] docs regen (gated on action/config/docgen changes)"
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

if ! "$BIN" docs generate --section all                --output docs-next/generated/actions.md    >/dev/null 2>&1 || \
   ! "$BIN" docs generate --section schema             --output docs-next/generated/schema.md     >/dev/null 2>&1 || \
   ! "$BIN" docs generate --section action-properties  --output docs-next/generated/properties.md >/dev/null 2>&1; then
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
  docs-next/generated/ \
  internal/config/schema.json \
  internal/config/schema.d \
  mooncake.d.ts)

if [ -z "$changed" ]; then
  echo "  ✓ generated docs + schema already in sync"
  exit 0
fi

echo "$changed" | sed 's/^/  ↑ auto-staging: /'
git add -- \
  docs-next/generated/ \
  internal/config/schema.json \
  internal/config/schema.d \
  mooncake.d.ts

echo
echo "✓ Fast checks green — full 'mooncake task ci' gate runs on push."
