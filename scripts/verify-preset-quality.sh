#!/bin/bash
# Verify preset quality against definitive style guide

PRESET_DIR="$1"

if [ -z "$PRESET_DIR" ]; then
    echo "Usage: $0 <preset-directory>"
    exit 1
fi

PRESET_NAME=$(basename "$PRESET_DIR")
ISSUES=()

# Check if preset exists
if [ ! -d "$PRESET_DIR" ]; then
    echo "ERROR: Preset directory not found: $PRESET_DIR"
    exit 1
fi

echo "Checking preset: $PRESET_NAME"
echo "================================"

# Check preset.yml
if [ ! -f "$PRESET_DIR/preset.yml" ]; then
    ISSUES+=("❌ Missing preset.yml")
else
    # Check description is meaningful
    DESC=$(grep "^description:" "$PRESET_DIR/preset.yml" | sed 's/description: //')
    if echo "$DESC" | grep -qE "(Utility tool|Java/JVM tool|Programming language|CLI tool)"; then
        ISSUES+=("⚠️  Generic description in preset.yml: '$DESC'")
    fi

    # Check if name matches directory
    NAME=$(grep "^name:" "$PRESET_DIR/preset.yml" | sed 's/name: //')
    if [ "$NAME" != "$PRESET_NAME" ]; then
        ISSUES+=("❌ Name mismatch: preset.yml has '$NAME', directory is '$PRESET_NAME'")
    fi
fi

# Check README.md
if [ ! -f "$PRESET_DIR/README.md" ]; then
    ISSUES+=("❌ Missing README.md")
else
    README="$PRESET_DIR/README.md"

    # Check for required sections
    grep -q "^## Quick Start" "$README" || ISSUES+=("❌ Missing '## Quick Start' section")
    grep -q "^## Features" "$README" || ISSUES+=("⚠️  Missing '## Features' section")
    grep -q "^## Basic Usage" "$README" || ISSUES+=("⚠️  Missing '## Basic Usage' section")
    grep -q "^## Advanced Configuration" "$README" || ISSUES+=("⚠️  Missing '## Advanced Configuration' section")
    grep -q "^## Parameters" "$README" || ISSUES+=("⚠️  Missing '## Parameters' section")
    grep -q "^## Platform Support" "$README" || ISSUES+=("⚠️  Missing '## Platform Support' section")
    grep -q "^## Agent Use" "$README" || ISSUES+=("❌ Missing '## Agent Use' section")
    grep -q "^## Resources" "$README" || ISSUES+=("❌ Missing '## Resources' section")

    # Check if README is too minimal (< 50 lines)
    LINE_COUNT=$(wc -l < "$README")
    if [ "$LINE_COUNT" -lt 50 ]; then
        ISSUES+=("⚠️  README is very short ($LINE_COUNT lines) - likely minimal template")
    fi

    # Check title format
    TITLE=$(head -1 "$README")
    if echo "$TITLE" | grep -qE "^# [a-z-]+$"; then
        ISSUES+=("⚠️  Title missing descriptive subtitle (should be '# name - Description')")
    fi
fi

# Check task files
if [ ! -f "$PRESET_DIR/tasks/install.yml" ]; then
    ISSUES+=("❌ Missing tasks/install.yml")
else
    # Check for assertions in install
    if ! grep -q "assert:" "$PRESET_DIR/tasks/install.yml"; then
        ISSUES+=("⚠️  No assertions in install.yml")
    fi

    # Check for platform detection using facts OR package action (which handles platform detection internally)
    # Also accept language package managers and installer scripts (rustup, os in) which work cross-platform
    if ! grep -qE "(apt_available|dnf_available|brew_available|package:|npm install|pip install|pip3 install|gem install|cargo install|conda install|sh.rustup.rs|os in)" "$PRESET_DIR/tasks/install.yml"; then
        ISSUES+=("⚠️  Not using fact-based platform detection in install.yml")
    fi
fi

if [ ! -f "$PRESET_DIR/tasks/uninstall.yml" ]; then
    ISSUES+=("❌ Missing tasks/uninstall.yml")
else
    # Check for assertions in uninstall
    if ! grep -q "assert:" "$PRESET_DIR/tasks/uninstall.yml"; then
        ISSUES+=("⚠️  No assertions in uninstall.yml")
    fi
fi

# Report results
if [ ${#ISSUES[@]} -eq 0 ]; then
    echo "✅ No issues found"
    exit 0
else
    echo ""
    echo "Issues found:"
    for issue in "${ISSUES[@]}"; do
        echo "  $issue"
    done
    echo ""
    echo "Total issues: ${#ISSUES[@]}"
    exit 1
fi
