#!/usr/bin/env bash
# ci/full.sh — pre-push gate. First failure stops the pipeline.
#
#   [1/9] build
#   [2/9] test
#   [3/9] golangci-lint (cache cleaned first; sibling worktrees leak)
#   [4/9] govulncheck
#   [5/9] regen docs + schema, then verify clean
#   [6/9] arch-snapshot summary
#   [7/9] CLAUDE.md soft-cap budget
#   [8/9] dupl production duplication report (informational)
#   [9/9] escalation-lint (spec-72 §G1; fail mode)
#
# Run -race tests with `mooncake task test-race`; we intentionally
# don't include them in the default ci pipeline because they roughly
# double the wall-clock time and the production lint catches most of
# what -race would flag at this codebase's scale.
set -euo pipefail

BIN="${BIN:-out/mooncake}"
PKG="${PKG:-./...}"

cd "$(git rev-parse --show-toplevel)"

echo "[1/9] build"
go build -o "$BIN" ./cmd

echo "[2/9] test"
go test "$PKG"

echo "[3/9] golangci-lint (clean cache first)"
# Cache leaks across sibling worktrees and surfaces phantom findings
# from ../mooncake-*/ paths. Clean before each run so results reflect
# this tree.
golangci-lint cache clean
golangci-lint run "$PKG"

echo "[4/9] govulncheck"
govulncheck "$PKG"

echo "[5/9] regen docs + schema, then verify clean"
rm -rf dist/docs && mkdir -p dist/docs
"$BIN" docs generate --section all-into-dir --output dist/docs                                >/dev/null
"$BIN" schema generate --format json       --output internal/config/schema.json --strict      >/dev/null
"$BIN" schema generate --format typescript --output internal/config/schema.d                  >/dev/null
"$BIN" schema generate --format typescript --output mooncake.d.ts                             >/dev/null
BIN="$BIN" bash ./scripts/docs-check.sh
BIN="$BIN" bash ./scripts/schema-check.sh

echo "[6/9] arch-snapshot (regenerated; file is gitignored)"
bash ./scripts/arch-snapshot.sh >/dev/null
snap=docs-working/ARCH_SNAPSHOT.md
if [ -f "$snap" ]; then
  # awk | head closes the pipe early and SIGPIPEs awk; under pipefail
  # that fails the script. Scope pipefail off for these summary pipes;
  # they're best-effort report output, not gating logic.
  set +o pipefail
  echo "  Top packages by LOC:"
  awk '/^\| `/{print "    " $0}' "$snap" | head -5
  god=$(awk '/^## God files/,/^## [^G]/' "$snap" | grep -cE '^[[:space:]]*[0-9]+ ' || true)
  echo "  God files (>500 LOC, non-test): $god"
  echo "  Top cyclomatic hotspots:"
  awk '/^## Cyclomatic hotspots/,/^## [^C]/' "$snap" | grep -E '^[[:space:]]*[0-9]+ ' | head -3 | sed 's/^/    /'
  set -o pipefail
fi

echo "[7/9] CLAUDE.md soft-cap budget"
bash ./scripts/budget-status.sh | sed 's/^/  /'

echo "[8/9] dupl (production-code duplication report)"
bash ./scripts/dupl-report.sh

echo "[9/9] escalation-lint (spec-72 §G1)"
bash ./scripts/escalation-lint.sh --fail

echo
echo "✓ All checks green — safe to commit."
