#!/bin/bash
# Setup script for Mooncake documentation

set -e

echo "🚀 Setting up Mooncake documentation site..."

# Check if Python is installed
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is required but not installed"
    echo "   Install from: https://www.python.org/downloads/"
    exit 1
fi

# Install pipenv if not already installed
if ! command -v pipenv &> /dev/null; then
    echo "📦 Installing pipenv..."
    pip install --user pipenv
    echo "✓ pipenv installed"
fi

# Install dependencies
echo "📦 Installing documentation dependencies with pipenv..."
pipenv install

# Copy documentation files
echo "🔗 Copying documentation files..."

# Copy CONTRIBUTING.md
mkdir -p docs/development
cp CONTRIBUTING.md docs/development/contributing.md
echo "✓ Copied CONTRIBUTING.md"

# Copy ROADMAP.md
cp ROADMAP.md docs/development/roadmap.md
echo "✓ Copied ROADMAP.md"

# Copy DEVELOPMENT.md
if [ -f "docs/DEVELOPMENT.md" ]; then
    cp docs/DEVELOPMENT.md docs/development/development.md
    echo "✓ Copied DEVELOPMENT.md"
fi

# Copy proposals README
if [ -f "docs/proposals/README.md" ]; then
    cp docs/proposals/README.md docs/development/proposals.md
    echo "✓ Copied proposals README"
fi

# Test the site
echo "🧪 Testing the site..."
pipenv run mkdocs build --strict

echo ""
echo "✅ Documentation setup complete!"
echo ""
echo "📝 Next steps:"
echo "1. Run: pipenv run mkdocs serve"
echo "2. Open: http://127.0.0.1:8000"
echo "3. Edit files in docs/"
echo "4. Push to master to auto-deploy"
echo ""
echo "🌐 After pushing, your site will be at:"
echo "   https://mooncake.alehatsman.com"
