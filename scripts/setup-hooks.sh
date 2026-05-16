#!/usr/bin/env bash
# Setup git hooks for mooncake development.
#
# Installs:
#   pre-commit — runs `task ci` (build + test-race + lint + scan +
#                docs/schema regen-and-check + arch-snapshot + dupl).
#                One command, one report, before the commit lands. If
#                any stage fails, the commit is rejected; agents fix
#                the issue before re-attempting.
#
# Removes any existing pre-push hook — the gate now runs at commit time.
#
# Bypass: `git commit --no-verify` (use sparingly; the point of the
#         hook is to catch quality issues before they hit master).
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
# Run the full CI pipeline before letting the commit land.
# Bypass: git commit --no-verify
set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

# Make sure go-task is installed (the only hard prereq of this hook).
TASK_BIN=""
if command -v task >/dev/null 2>&1; then
  TASK_BIN="task"
elif [ -x "$(go env GOPATH 2>/dev/null)/bin/task" ]; then
  TASK_BIN="$(go env GOPATH)/bin/task"
else
  echo "pre-commit: 'task' (go-task) is required but not installed." >&2
  echo "            Install with: go install github.com/go-task/task/v3/cmd/task@latest" >&2
  echo "            Then: task install-tools" >&2
  exit 1
fi

echo "pre-commit: running 'task ci' (build + test-race + lint + scan + docs/schema + arch + dupl)..."
if ! "$TASK_BIN" ci; then
  echo "" >&2
  echo "pre-commit: ✗ CI gate failed. Fix the issue above and re-commit," >&2
  echo "            or 'git commit --no-verify' to bypass (not recommended)." >&2
  exit 1
fi
HOOK_EOF
chmod +x "$HOOKS_DIR/pre-commit"
echo "  ✓ pre-commit → $HOOKS_DIR/pre-commit"

# ----- remove pre-push -------------------------------------------------------
# The full gate now runs at commit time; pre-push is redundant and
# slows the push step for no extra coverage.
if [ -f "$HOOKS_DIR/pre-push" ]; then
  rm -f "$HOOKS_DIR/pre-push"
  echo "  ✓ pre-push removed (gate moved to pre-commit)"
fi

cat <<EOM

Installed:
  pre-commit  → runs 'task ci' (full pipeline). ~1-2 min depending on the
                test surface. Fails the commit on any failing stage so
                quality issues are caught before they land.

Bypass with 'git commit --no-verify' if you really need it.

If 'task' isn't on PATH yet:
  go install github.com/go-task/task/v3/cmd/task@latest
  task install-tools
EOM
