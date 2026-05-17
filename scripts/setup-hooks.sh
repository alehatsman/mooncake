#!/usr/bin/env bash
# Setup git hooks for mooncake development.
#
# Installs two hooks, both thin wrappers over `task`:
#
#   pre-commit → task ci:fast
#     Fast gate (target <5s on warm cache): go vet, gofmt on staged files,
#     ai-lint on staged files, soft-cap budget report. Catches the cheap
#     mistakes that should never reach a commit (unformatted code, stub
#     panics, agent-tagged TODOs) without blocking iteration speed.
#
#   pre-push → task ci
#     Full gate: build + test-race + lint + scan + docs/schema regen +
#     arch-snapshot + budget + dupl. Slow (~1-2 min), runs once per push.
#
# Bypass: `git commit --no-verify` or `git push --no-verify`. Use sparingly —
# both hooks exist because the issues they catch are easier to fix at commit
# time than during review.
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

TASK_BIN=""
if command -v task >/dev/null 2>&1; then
  TASK_BIN="task"
elif [ -x "$(go env GOPATH 2>/dev/null)/bin/task" ]; then
  TASK_BIN="$(go env GOPATH)/bin/task"
else
  echo "pre-commit: 'task' (go-task) is required but not installed." >&2
  echo "            Install: go install github.com/go-task/task/v3/cmd/task@latest" >&2
  exit 1
fi

echo "pre-commit: running 'task ci:fast' (vet + gofmt + ai-lint + budget)..."
if ! "$TASK_BIN" ci:fast; then
  echo "" >&2
  echo "pre-commit: ✗ fast gate failed. Fix the issue above and re-commit," >&2
  echo "            or 'git commit --no-verify' to bypass (not recommended)." >&2
  exit 1
fi
HOOK_EOF
chmod +x "$HOOKS_DIR/pre-commit"
echo "  ✓ pre-commit → task ci:fast"

# ----- pre-push --------------------------------------------------------------
cat > "$HOOKS_DIR/pre-push" << 'HOOK_EOF'
#!/usr/bin/env bash
# Full gate before the push leaves the machine. Bypass: git push --no-verify.
set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

TASK_BIN=""
if command -v task >/dev/null 2>&1; then
  TASK_BIN="task"
elif [ -x "$(go env GOPATH 2>/dev/null)/bin/task" ]; then
  TASK_BIN="$(go env GOPATH)/bin/task"
else
  echo "pre-push: 'task' (go-task) is required but not installed." >&2
  echo "          Install: go install github.com/go-task/task/v3/cmd/task@latest" >&2
  exit 1
fi

echo "pre-push: running 'task ci' (build + test-race + lint + scan + docs/schema + arch + dupl)..."
if ! "$TASK_BIN" ci; then
  echo "" >&2
  echo "pre-push: ✗ full gate failed. Fix the issue above and re-push," >&2
  echo "          or 'git push --no-verify' to bypass (not recommended)." >&2
  exit 1
fi
HOOK_EOF
chmod +x "$HOOKS_DIR/pre-push"
echo "  ✓ pre-push → task ci"

cat <<EOM

Installed:
  pre-commit  → 'task ci:fast' (~seconds). Catches stub panics, agent
                TODOs, unformatted code, and soft-cap regressions before
                they land in a commit.
  pre-push    → 'task ci'      (~1-2 min). Full build + test-race + lint
                + scan + docs/schema regen + arch + dupl before the
                commits leave the machine.

Bypass either with --no-verify when you really need it.

If 'task' isn't on PATH yet:
  go install github.com/go-task/task/v3/cmd/task@latest
  task install-tools
EOM
