#!/bin/bash
set -e
echo "Setting up docs..."
mkdir -p docs/development
cp CONTRIBUTING.md docs/development/contributing.md 2>/dev/null || echo "Skip CONTRIBUTING"
cp ROADMAP.md docs/development/roadmap.md 2>/dev/null || echo "Skip ROADMAP"
cp docs/DEVELOPMENT.md docs/development/development.md 2>/dev/null || echo "Skip DEVELOPMENT"
cp docs/proposals/README.md docs/development/proposals.md 2>/dev/null || echo "Skip proposals"
echo "Done"
