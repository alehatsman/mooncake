#!/usr/bin/env bash
# Setup git hooks for mooncake development.
#
# Installs two hooks, both thin wrappers over `mooncake task`:
#
#   pre-commit → mooncake task ci-fast
#     Fast gate (target <5s on warm cache): go vet, gofmt on staged files,
#     ai-lint on staged files, soft-cap budget report, docs/schema regen
#     auto-stage when handler/config Go files are staged. Catches the
#     cheap mistakes that should never reach a commit (unformatted code,
#     stub panics, agent-tagged TODOs, generated-doc drift) without
#     blocking iteration speed.
#
#   pre-push → mooncake task ci
#     Full gate: build + test + lint + scan + docs/schema regen +
#     arch-snapshot + budget + dupl + escalation-lint. Slow (~1-2 min),
#     runs once per push.
#
# Bypass: `git commit --no-verify` or `git push --no-verify`. Use sparingly —
# both hooks exist because the issues they catch are easier to fix at commit
# time than during review.
#
# Requires mooncake to be on PATH. Install with `mooncake task install`
# (or `go build -o ~/.local/bin/mooncake ./cmd` on first checkout).
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
# Fast gate before the commit lands. Bypass: git commit --no-verify.
set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if ! command -v mooncake >/dev/null 2>&1; then
  echo "pre-commit: 'mooncake' is required but not on PATH." >&2
  echo "            Install: go build -o ~/.local/bin/mooncake ./cmd" >&2
  exit 1
fi

echo "pre-commit: running 'mooncake task ci-fast' (vet + gofmt + ai-lint + budget + docs-regen)..."
if ! mooncake task ci-fast; then
  echo "" >&2
  echo "pre-commit: ✗ fast gate failed. Fix the issue above and re-commit," >&2
  echo "            or 'git commit --no-verify' to bypass (not recommended)." >&2
  exit 1
fi
HOOK_EOF
chmod +x "$HOOKS_DIR/pre-commit"
echo "  ✓ pre-commit → mooncake task ci-fast"

# ----- pre-push --------------------------------------------------------------
cat > "$HOOKS_DIR/pre-push" << 'HOOK_EOF'
#!/usr/bin/env bash
# Full gate before the push leaves the machine. Bypass: git push --no-verify.
set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if ! command -v mooncake >/dev/null 2>&1; then
  echo "pre-push: 'mooncake' is required but not on PATH." >&2
  echo "          Install: go build -o ~/.local/bin/mooncake ./cmd" >&2
  exit 1
fi

echo "pre-push: running 'mooncake task ci' (build + test + lint + scan + docs/schema + arch + budget + dupl)..."
if ! mooncake task ci; then
  echo "" >&2
  echo "pre-push: ✗ full gate failed. Fix the issue above and re-push," >&2
  echo "          or 'git push --no-verify' to bypass (not recommended)." >&2
  exit 1
fi
HOOK_EOF
chmod +x "$HOOKS_DIR/pre-push"
echo "  ✓ pre-push → mooncake task ci"

cat <<EOM

Installed:
  pre-commit  → 'mooncake task ci-fast' (~seconds). Catches stub panics,
                agent TODOs, unformatted code, soft-cap regressions, and
                generated-doc drift before they land in a commit.
                Auto-stages dist/docs/* + schema.json when a
                handler/config Go file is staged and the generator
                emits new content.
  pre-push    → 'mooncake task ci'      (~1-2 min). Full build + test +
                lint + scan + docs/schema regen + arch + budget + dupl
                + escalation-lint before the commits leave the machine.

Bypass either with --no-verify when you really need it.

If 'mooncake' isn't on PATH yet:
  go build -o ~/.local/bin/mooncake ./cmd
  mooncake task install-tools
EOM
