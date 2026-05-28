#!/usr/bin/env bash
# seed-action-readmes — drop a stub README.md into each
# internal/actions/<dir>/ that contains a handler.go and has no
# README yet. Each stub records the action name + one-line
# description from the handler's metadata literal, plus a pointer
# to the generated reference page. Idempotent — re-running leaves
# existing READMEs alone (organic content is preserved).
#
# This is a one-shot seed; future per-action prose is added by
# editing the README in the same PR that changes the action.
set -eu
# Intentionally no pipefail: `grep -oE ... | head -1` legitimately
# exits non-zero when there's no match (handler without Description
# literal), and we want the script to keep iterating. The downstream
# empty-string check catches the no-match case explicitly.

ROOT="$(git rev-parse --show-toplevel)"
ACTIONS_DIR="$ROOT/internal/actions"

if [ ! -d "$ACTIONS_DIR" ]; then
  echo "error: $ACTIONS_DIR not found — run from a mooncake checkout" >&2
  exit 1
fi

seeded=0
skipped=0
unmatched=0

for handler in "$ACTIONS_DIR"/*/handler.go; do
  dir="$(dirname "$handler")"
  readme="$dir/README.md"

  if [ -f "$readme" ]; then
    skipped=$((skipped + 1))
    continue
  fi

  # Extract the action name. Most handlers use a literal:
  #   Name: "file.copy"
  # A minority (e.g. windows_firewall_rule) reference a const:
  #   Name: actionName
  #   ...
  #   const actionName = "windows.firewall_rule"
  # We try the literal form first; if empty, we extract the identifier
  # and look up its const value in the same file.
  name="$(grep -oE 'Name:[[:space:]]+"[^"]+"' "$handler" | head -1 \
          | sed -E 's/.*"([^"]+)".*/\1/')"

  if [ -z "$name" ]; then
    ident="$(grep -oE 'Name:[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*' "$handler" | head -1 \
             | sed -E 's/Name:[[:space:]]+//')"
    if [ -n "$ident" ]; then
      name="$(grep -oE "${ident}[[:space:]]*=[[:space:]]*\"[^\"]+\"" "$handler" | head -1 \
              | sed -E 's/.*"([^"]+)".*/\1/')"
    fi
  fi

  if [ -z "$name" ]; then
    echo "  ? skipped (no Name resolved): $handler" >&2
    unmatched=$((unmatched + 1))
    continue
  fi

  # Description is single-line in most handlers. Multi-line strings
  # exist but are rare; we capture the first line which is enough
  # for a stub.
  desc="$(grep -oE 'Description:[[:space:]]+"[^"]+"' "$handler" | head -1 \
          | sed -E 's/.*"([^"]+)".*/\1/')"

  if [ -z "$desc" ]; then
    desc="(add description here)"
  fi

  slug="$(echo "$name" | tr '.' '_')"
  cat > "$readme" <<EOF
---
action: $name
---
# $name

$desc

See generated reference: \`dist/docs/actions/$slug.md\`

> Stub README. Add caveats, common pitfalls, or background context
> here. The generated action card on the docs site picks up this
> file's body when present.
EOF

  echo "  + seeded $readme"
  seeded=$((seeded + 1))
done

echo
echo "seed-action-readmes: seeded=$seeded skipped=$skipped unmatched=$unmatched"
