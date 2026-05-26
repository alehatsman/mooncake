#!/usr/bin/env bash
# dupl-report — run dupl on production (non-test) Go files and print a
# ranked list of clone pairs. Threshold via $T (default 100). The
# python post-processor dedupes pairs and orders by clone size — dupl
# itself emits one line per direction (A → B and B → A as separate
# entries).
#
# When dupl isn't installed, exits 0 with a "skipping" notice so this
# script can run as part of `mooncake task ci` without forcing the
# tool to be a hard dependency. Run `mooncake task install-tools` to
# install dupl.
set -euo pipefail

T="${T:-100}"

if ! command -v dupl >/dev/null 2>&1 && ! [ -x "$(go env GOPATH)/bin/dupl" ]; then
  echo "  dupl not installed — run 'mooncake task install-tools'. Skipping."
  exit 0
fi

dupl -threshold "$T" -plumbing . 2>&1 | grep -v "_test.go" | T="$T" python3 -c '
import os, sys
threshold = os.environ.get("T", "100")
pairs = set()
for line in sys.stdin:
    parts = line.strip().split(": duplicate of ")
    if len(parts) == 2:
        pairs.add(tuple(sorted(parts)))
def lines(rng):
    lo, hi = rng.split(":")[-1].split("-")
    return int(hi) - int(lo) + 1
ranked = sorted(pairs, key=lambda p: -lines(p[0]))
if not ranked:
    print(f"  no production duplication at threshold {threshold}.")
else:
    print(f"  {len(ranked)} production clone pair(s) at threshold {threshold} (informational):")
    for a, b in ranked:
        print(f"    {lines(a):>3}L  {a}  <-->  {b}")
'
