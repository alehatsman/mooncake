#!/usr/bin/env bash
# deadcode-check — run golang.org/x/tools/cmd/deadcode over the whole
# module (including tests) and fail on any unreachable function, with
# one documented exception: internal/dscl is darwin-only (its two
# consumers, internal/actions/{os_user,os_group}/platform_darwin.go,
# carry a `//go:build darwin` tag), so a run on a linux/windows host
# always reports its three exported functions as unreachable — a
# cross-platform false positive, not real dead code. Verified with
# `GOOS=darwin GOARCH=arm64 go build ./...` (clean) and
# `GOOS=darwin GOARCH=arm64 deadcode -test ./...` (dscl absent from
# that platform's own unreachable list) — moongit #208.
#
# Pinned to the x/tools version already vendored in go.sum so `go run`
# doesn't fetch a newer, potentially differently-behaved analyzer.
set -euo pipefail

DEADCODE_VERSION="v0.49.0"
ALLOWLIST='^internal/dscl/(nextid\.go:(26|50):[0-9]+|run\.go:22:[0-9]+): unreachable func: (NextAvailableID|captureDscl|Run)$'

out="$(go run "golang.org/x/tools/cmd/deadcode@${DEADCODE_VERSION}" -test ./... 2>&1)" || {
  echo "✗ deadcode failed to run:"
  echo "$out"
  exit 1
}

# A cold module cache (first run in a fresh container) makes `go run`
# print "go: downloading ..." progress to stderr, captured above by
# 2>&1 alongside deadcode's own findings on stdout. Strip that
# toolchain noise before checking against the allowlist — it isn't a
# deadcode finding, and its exact lines vary with what's already cached.
findings="$(echo "$out" | grep -v '^go: ')"

unexpected="$(echo "$findings" | grep -vE "$ALLOWLIST" || true)"

if [ -n "$unexpected" ]; then
  echo "✗ deadcode found unreachable code outside the documented dscl exception:"
  echo
  echo "$unexpected"
  echo
  echo "Delete it, or if it's a genuine cross-platform false positive," \
       "add it to ALLOWLIST in scripts/deadcode-check.sh with a comment" \
       "explaining why (see the internal/dscl precedent above)."
  exit 1
fi

echo "✓ No unreachable code (internal/dscl's 3 darwin-only false positives excepted)"
