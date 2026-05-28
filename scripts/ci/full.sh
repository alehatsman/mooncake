#!/usr/bin/env bash
# ci/full.sh — pre-push gate. First failure stops the pipeline.
#
#   [1/10] build
#   [2/10] test
#   [3/10] golangci-lint (cache cleaned first; sibling worktrees leak)
#   [4/10] govulncheck
#   [5/10] regen docs + schema, then verify clean
#   [6/10] mkdocs build --strict (skipped if pipenv/mkdocs absent)
#   [7/10] arch-snapshot summary
#   [8/10] CLAUDE.md soft-cap budget
#   [9/10] dupl production duplication report (informational)
#   [10/10] escalation-lint (spec-72 §G1; fail mode)
#
# Run -race tests with `mooncake task test-race`; we intentionally
# don't include them in the default ci pipeline because they roughly
# double the wall-clock time and the production lint catches most of
# what -race would flag at this codebase's scale.
set -euo pipefail

BIN="${BIN:-out/mooncake}"
PKG="${PKG:-./...}"

cd "$(git rev-parse --show-toplevel)"

echo "[1/10] build"
go build -o "$BIN" ./cmd

echo "[2/10] test"
go test "$PKG"

echo "[3/10] golangci-lint (clean cache first)"
# Cache leaks across sibling worktrees and surfaces phantom findings
# from ../mooncake-*/ paths. Clean before each run so results reflect
# this tree.
golangci-lint cache clean
golangci-lint run "$PKG"

echo "[4/10] govulncheck"
govulncheck "$PKG"

echo "[5/10] regen docs + schema, then verify clean"
rm -rf dist/docs && mkdir -p dist/docs
"$BIN" docs generate --section all-into-dir --output dist/docs                                >/dev/null
"$BIN" schema generate --format json       --output internal/config/schema.json --strict      >/dev/null
"$BIN" schema generate --format typescript --output internal/config/schema.d                  >/dev/null
"$BIN" schema generate --format typescript --output mooncake.d.ts                             >/dev/null
BIN="$BIN" bash ./scripts/docs-check.sh
BIN="$BIN" bash ./scripts/schema-check.sh

# Build the MkDocs site so a broken mkdocs.yml, missing literate-nav
# plugin, or broken-link in a generated page surfaces at push time
# instead of after merge. We skip silently when pipenv/mkdocs aren't
# installed — the dev's local checkout may not have run
# `mooncake task docs-tools-install`. CI environments should preinstall
# the deps so the gate is enforced there.
echo "[6/10] mkdocs build --strict (skipped when pipenv/mkdocs absent)"
if command -v pipenv >/dev/null 2>&1 && pipenv run mkdocs --version >/dev/null 2>&1; then
  DISABLE_MKDOCS_2_WARNING=true pipenv run mkdocs build --strict --quiet
  echo "  ✓ mkdocs site built"
else
  echo "  ⚠ pipenv/mkdocs not on PATH — skipping (run \`mooncake task docs-tools-install\` to enable)"
fi

echo "[7/10] arch-snapshot (regenerated; file is gitignored)"
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

echo "[8/10] CLAUDE.md soft-cap budget"
bash ./scripts/budget-status.sh | sed 's/^/  /'

echo "[9/10] dupl (production-code duplication report)"
bash ./scripts/dupl-report.sh

echo "[10/10] escalation-lint (spec-72 §G1)"
bash ./scripts/escalation-lint.sh --fail

echo
echo "✓ All checks green — safe to commit."
