#!/usr/bin/env bash
# install-tools — go install every static-analysis tool the project
# depends on. Idempotent: re-running picks up newer versions.
#
# Verify after with: mooncake task check-tools
set -euo pipefail

tools=(
  "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
  "golang.org/x/vuln/cmd/govulncheck@latest"
  "github.com/securego/gosec/v2/cmd/gosec@latest"
  "github.com/fzipp/gocyclo/cmd/gocyclo@latest"
  "github.com/loov/goda@latest"
  "github.com/mibk/dupl@latest"
)

for spec in "${tools[@]}"; do
  echo "→ go install $spec"
  go install "$spec"
done

echo
echo "✓ Tools installed. Verify with 'mooncake task check-tools'."
