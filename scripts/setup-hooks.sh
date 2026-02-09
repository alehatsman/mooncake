#!/usr/bin/env bash
# Setup git hooks for mooncake development

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$REPO_ROOT/.git/hooks"

echo "Setting up mooncake development hooks..."

# Create pre-commit hook
cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/usr/bin/env bash
# Pre-commit hook for mooncake
# Regenerates documentation if code changes

set -e

echo "🔍 Pre-commit: Checking for code changes..."

# Check if any Go files, preset files, or config files changed
if git diff --cached --name-only | grep -qE '\.(go|yml|yaml)$'; then
    echo "📝 Code changes detected, regenerating documentation..."

    # Check if docs are already staged
    DOCS_STAGED=$(git diff --cached --name-only docs-next/generated/ || echo "")

    # Build and generate docs
    make build > /dev/null 2>&1
    make docs-generate > /dev/null 2>&1

    # Check if docs changed
    if ! git diff --quiet docs-next/generated/; then
        echo "📚 Documentation updated, staging changes..."
        git add docs-next/generated/
        echo "✅ Documentation regenerated and staged"
    else
        echo "✅ Documentation is already up to date"
    fi
else
    echo "✅ No code changes, skipping documentation generation"
fi
EOF

chmod +x "$HOOKS_DIR/pre-commit"

echo "✅ Pre-commit hook installed at $HOOKS_DIR/pre-commit"
echo ""
echo "The hook will automatically:"
echo "  - Detect Go/YAML file changes"
echo "  - Regenerate documentation"
echo "  - Stage updated docs in the commit"
echo ""
echo "To bypass the hook temporarily, use: git commit --no-verify"
