#!/usr/bin/env bash
# escalation-lint — spec-72 §G1 enforcement.
#
# Goal: ctx.Privileged() is the only allowed escalation constructor in
# production handler code. Direct construction of
# security.BecomeRunner{...} or security.PrivilegedRunner{...} outside
# the two privileged files is a smell — it bypasses the centralized
# escalation decision and was the root cause of the five sudo bugs
# documented in F051.
#
# Phase 2 (current): warning-only — exits 0 regardless of findings,
# prints a counted list so PR reviewers can see the trend.
# Phase 5 (future): flips to exit 1 once the 12 nil-guard sites and
# the 3 deferred hand-rolled sites have been migrated.
#
# Allowed locations (kernel of the escalation policy):
#   - internal/security/                  — owns BecomeRunner /
#                                           PrivilegedRunner construction
#   - internal/executor/context.go        — ctx.Privileged() factory
#
# All other matches in non-test code are reported. Test files are
# scanned too but reported separately — tests injecting empty
# fakes is fine; the spec only forbids it in production paths.
#
# Usage:
#   bash scripts/escalation-lint.sh        # scan all tracked Go files
#   bash scripts/escalation-lint.sh --fail # exit 1 on production findings
#                                            (Phase 5 will make this the
#                                             default)

set -euo pipefail

fail_on_findings=0
for arg in "$@"; do
  case "$arg" in
    --fail) fail_on_findings=1 ;;
    -h|--help) sed -n '2,28p' "$0"; exit 0 ;;
  esac
done

cd "$(git rev-parse --show-toplevel)"

pattern='security\.(BecomeRunner|PrivilegedRunner)\{'

# Allowed files — relative paths from repo root. These own the
# escalation kernel and are the deliberate constructor sites named
# in spec-72 §"Reuse map" (the canonical paths already consume
# RunServices correctly today).
is_allowed() {
  case "$1" in
    internal/security/*) return 0 ;;
    internal/executor/context.go) return 0 ;;
    # defaultPerformer.runSudo is the kernel's "write-with-fallback-
    # to-sudo" primitive; spec-72 §"Reuse map" lists it as a canonical
    # path that correctly threads SudoPass+PasswordlessSudo from
    # RunServices. Not a violation.
    internal/effects/default.go) return 0 ;;
    scripts/escalation-lint.sh) return 0 ;;
  esac
  return 1
}

# Skip matches inside line comments — keeps spec/docs references in
# code comments from false-positiving (e.g. "migrated from direct
# security.BecomeRunner{} construction").
is_comment_match() {
  local line_content="${1#*:*:}" # strip "path:lineno:" prefix
  [[ "${line_content##[[:space:]]}" == //* ]]
}

prod_findings=0
test_findings=0
prod_lines=()
test_lines=()

while IFS= read -r f; do
  # Skip generated / vendored files
  case "$f" in
    vendor/*|*/vendor/*|*.pb.go|*_generated.go|*_gen.go) continue ;;
  esac
  if [ -f "$f" ] && head -5 "$f" 2>/dev/null | grep -q '^// Code generated'; then
    continue
  fi
  if is_allowed "$f"; then continue; fi

  while IFS= read -r match; do
    [ -z "$match" ] && continue
    if is_comment_match "$match"; then continue; fi
    if [[ "$f" == *_test.go ]]; then
      test_lines+=("$match")
      test_findings=$((test_findings + 1))
    else
      prod_lines+=("$match")
      prod_findings=$((prod_findings + 1))
    fi
  done < <(grep -nHE "$pattern" "$f" 2>/dev/null || true)
done < <(git ls-files -- '*.go')

echo "  escalation-lint (spec-72 §G1)"
echo "    Production sites violating ctx.Privileged() rule: $prod_findings"
if [ "$prod_findings" -gt 0 ]; then
  for line in "${prod_lines[@]}"; do
    echo "      $line"
  done
fi
echo "    Test sites (informational, not a violation): $test_findings"

if [ "$fail_on_findings" -eq 1 ] && [ "$prod_findings" -gt 0 ]; then
  echo ""
  echo "  ✗ escalation-lint: $prod_findings production violation(s) (spec-72 §G1)."
  echo "    Migrate each site to ctx.Privileged() — see"
  echo "    docs-working/streams/fleet/specs/spec-72-unified-escalation-policy.md."
  exit 1
fi

exit 0
