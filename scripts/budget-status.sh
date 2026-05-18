#!/usr/bin/env bash
# budget-status — print current state of the three CLAUDE.md soft caps.
#
# Soft caps (see CLAUDE.md §Architecture soft caps):
#   1. Handler LOC > 1500       → split into per-OS sub-pkgs
#   2. gocyclo > 35 (non-test)  → refactor on next touch
#   3. Step universal fields > 40 → flag the "why does every step need this?" question
#
# Each section prints:  ✗ over cap   ⚠ within 20% of cap   ✓ clean
set -euo pipefail

CAP_HANDLER_LOC=1500
CAP_GOCYCLO=35
CAP_STEP_FIELDS=40

cd "$(git rev-parse --show-toplevel)"

if [ -t 1 ]; then
  bold=$(tput bold 2>/dev/null || true)
  red=$(tput setaf 1 2>/dev/null || true)
  yellow=$(tput setaf 3 2>/dev/null || true)
  green=$(tput setaf 2 2>/dev/null || true)
  reset=$(tput sgr0 2>/dev/null || true)
else
  bold= red= yellow= green= reset=
fi

printf '%sCLAUDE.md soft caps — current state%s\n' "$bold" "$reset"
echo

# ---- 1. Handler LOC ----------------------------------------------------------
printf '%s1. Handler LOC vs cap %d%s\n' "$bold" "$CAP_HANDLER_LOC" "$reset"
warn_at=$((CAP_HANDLER_LOC * 80 / 100))
violations=0; warnings=0
for d in internal/actions/*/; do
  [ -d "$d" ] || continue
  loc=$(find "$d" -maxdepth 1 -name '*.go' -not -name '*_test.go' -print0 2>/dev/null \
        | xargs -0 cat 2>/dev/null | wc -l | awk '{print $1}')
  name=${d#internal/actions/}; name=${name%/}
  if [ "$loc" -gt "$CAP_HANDLER_LOC" ]; then
    printf '   %s✗ %-40s %5d LOC (over)%s\n' "$red" "$name" "$loc" "$reset"
    violations=$((violations + 1))
  elif [ "$loc" -gt "$warn_at" ]; then
    printf '   %s⚠ %-40s %5d LOC (within 20%%)%s\n' "$yellow" "$name" "$loc" "$reset"
    warnings=$((warnings + 1))
  fi
done
if [ "$violations" -eq 0 ] && [ "$warnings" -eq 0 ]; then
  printf '   %s✓ all handlers under %d LOC%s\n' "$green" "$warn_at" "$reset"
fi
echo

# ---- 2. gocyclo --------------------------------------------------------------
printf '%s2. Non-test functions vs gocyclo cap %d%s\n' "$bold" "$CAP_GOCYCLO" "$reset"
if command -v gocyclo >/dev/null 2>&1; then
  out=$(gocyclo -over "$CAP_GOCYCLO" . 2>/dev/null | grep -v '_test\.go' || true)
  if [ -n "$out" ]; then
    printf '%s\n' "$out" | awk -v r="$red" -v R="$reset" '{ printf "   %s✗ gocyclo=%-3s %-30s %s%s\n", r, $1, $2"."$3, $4, R }'
  else
    printf '   %s✓ all functions under %d%s\n' "$green" "$CAP_GOCYCLO" "$reset"
  fi
else
  echo "   gocyclo not installed — run 'task install-tools'"
fi
echo

# ---- 3. Step universal-field count -------------------------------------------
# Count fields in `type Step struct` that DO NOT carry an `action:` tag.
# Action-selector fields (FileWrite, Pkg, …) are not "universal"; everything
# else (Name, Tags, When, idempotency guards, register, retry, …) is.
printf '%s3. internal/config.Step universal-field count vs cap %d%s\n' "$bold" "$CAP_STEP_FIELDS" "$reset"
step_fields=$(awk '/^type Step struct/,/^}/' internal/config/config.go \
  | grep -E '^[[:space:]]+[A-Z][a-zA-Z0-9_]+[[:space:]]' \
  | grep -v 'action:"' \
  | wc -l \
  | awk '{print $1}')
warn_at=$((CAP_STEP_FIELDS * 80 / 100))
if [ "$step_fields" -gt "$CAP_STEP_FIELDS" ]; then
  printf '   %s✗ %d universal fields (over)%s\n' "$red" "$step_fields" "$reset"
elif [ "$step_fields" -gt "$warn_at" ]; then
  printf '   %s⚠ %d universal fields (within 20%%)%s\n' "$yellow" "$step_fields" "$reset"
else
  printf '   %s✓ %d universal fields (cap: %d)%s\n' "$green" "$step_fields" "$CAP_STEP_FIELDS" "$reset"
fi
