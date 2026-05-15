#!/usr/bin/env bash
# Setup git hooks for mooncake development.
#
# Installs:
#   pre-commit  — fast: regen docs/schema/arch-snapshot if code/YAML staged, stage results.
#   pre-push    — slow: full `make ci` + arch-snapshot dirty-check.
#
# Bypass: --no-verify on commit or push.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$(git -C "$REPO_ROOT" rev-parse --git-path hooks)"
case "$HOOKS_DIR" in
  /*) ;;                              # already absolute
  *) HOOKS_DIR="$REPO_ROOT/$HOOKS_DIR" ;;
esac

mkdir -p "$HOOKS_DIR"
echo "Setting up mooncake development hooks in: $HOOKS_DIR"

# ----- pre-commit ------------------------------------------------------------
cat > "$HOOKS_DIR/pre-commit" << 'HOOK_EOF'
#!/usr/bin/env bash
# Auto-regenerate derived artifacts when code/config files are staged.
# Bypass: git commit --no-verify
set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

# Skip cheaply if nothing relevant is staged.
if ! git diff --cached --name-only | grep -qE '\.(go|yml|yaml)$'; then
  echo "pre-commit: no Go/YAML changes — skipping regen."
  exit 0
fi

echo "pre-commit: code changes detected, regenerating derived artifacts..."

# Build once (docs-generate and schema-generate need the binary).
if ! make build >/dev/null 2>&1; then
  echo "pre-commit: 'make build' failed. Fix the build, then re-commit." >&2
  exit 1
fi

# Track what we touched so we only print/stage the ones that actually changed.
regen_targets=(
  "docs-next/generated/"
  "internal/config/schema.json"
  "internal/config/schema.d/"
  "mooncake.d.ts"
  "docs-working/ARCH_SNAPSHOT.md"
)

make docs-generate >/dev/null 2>&1   || { echo "pre-commit: docs-generate failed"   >&2; exit 1; }
make schema-generate >/dev/null 2>&1 || { echo "pre-commit: schema-generate failed" >&2; exit 1; }
make arch-snapshot >/dev/null 2>&1   || { echo "pre-commit: arch-snapshot failed"   >&2; exit 1; }

changed=0
for target in "${regen_targets[@]}"; do
  if [ -e "$target" ] && ! git diff --quiet -- "$target" 2>/dev/null; then
    git add "$target"
    echo "  + staged: $target"
    changed=1
  fi
done

if [ $changed -eq 0 ]; then
  echo "pre-commit: derived artifacts already up to date."
else
  echo "pre-commit: regenerated artifacts have been staged into this commit."
fi
HOOK_EOF
chmod +x "$HOOKS_DIR/pre-commit"

# ----- pre-push --------------------------------------------------------------
cat > "$HOOKS_DIR/pre-push" << 'HOOK_EOF'
#!/usr/bin/env bash
# Run the full CI gate plus arch-snapshot dirty-check before push.
# Bypass: git push --no-verify
set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "pre-push: running 'make ci' (lint + test-race + scan + docs-check + schema-check)..."
if ! make ci; then
  echo "pre-push: 'make ci' failed. Fix and re-push (or --no-verify to bypass)." >&2
  exit 1
fi

echo "pre-push: regenerating arch-snapshot to verify freshness..."
make arch-snapshot >/dev/null 2>&1 || { echo "pre-push: arch-snapshot failed" >&2; exit 1; }

if ! git diff --quiet -- docs-working/ARCH_SNAPSHOT.md; then
  echo "pre-push: docs-working/ARCH_SNAPSHOT.md is out of date." >&2
  echo "          Run 'make arch-snapshot', commit the result, and re-push." >&2
  exit 1
fi

echo "pre-push: ✓ all gates passed."
HOOK_EOF
chmod +x "$HOOKS_DIR/pre-push"

cat <<EOM

Installed:
  pre-commit  → $HOOKS_DIR/pre-commit
                  Regenerates docs + schema + arch-snapshot on Go/YAML changes
                  and stages them. ~5-15s on typical commits.
  pre-push    → $HOOKS_DIR/pre-push
                  Runs 'make ci' (lint, test-race, scan, docs-check,
                  schema-check) and an arch-snapshot freshness check.
                  ~1-2 min — pre-push only, so it doesn't slow commits.

Bypass either with --no-verify.
EOM
