#!/bin/bash
# Preset Verification Script
# Usage: ./scripts/verify-preset.sh <preset-name>

set -e

PRESET=$1

if [ -z "$PRESET" ]; then
  echo "Usage: $0 <preset-name>"
  exit 1
fi

PRESET_DIR="presets/$PRESET"

if [ ! -d "$PRESET_DIR" ]; then
  echo "❌ Preset directory not found: $PRESET_DIR"
  exit 1
fi

echo "🔍 Verifying preset: $PRESET"
echo ""

# Check 1: preset.yml exists
if [ -f "$PRESET_DIR/preset.yml" ]; then
  echo "✅ preset.yml exists"
else
  echo "❌ preset.yml missing"
  exit 1
fi

# Check 2: Has install tasks
if grep -q "tasks/install" "$PRESET_DIR/preset.yml"; then
  echo "✅ Has install tasks"
else
  echo "❌ No install tasks found"
fi

# Check 3: Has remove/uninstall support
if grep -q "state.*absent" "$PRESET_DIR/preset.yml" || \
   grep -q "tasks/uninstall" "$PRESET_DIR/preset.yml" || \
   grep -q "tasks/remove" "$PRESET_DIR/preset.yml"; then
  echo "✅ Has removal support"
else
  echo "⚠️  No explicit removal support found"
fi

# Check 4: README exists and has content
if [ -f "$PRESET_DIR/README.md" ]; then
  LINE_COUNT=$(wc -l < "$PRESET_DIR/README.md" | tr -d ' ')
  if [ "$LINE_COUNT" -ge 150 ]; then
    echo "✅ README.md exists ($LINE_COUNT lines - comprehensive)"
  elif [ "$LINE_COUNT" -ge 50 ]; then
    echo "⚠️  README.md exists ($LINE_COUNT lines - needs enhancement)"
  else
    echo "❌ README.md exists ($LINE_COUNT lines - minimal)"
  fi
else
  echo "❌ README.md missing"
fi

# Check 5: List available task files
echo ""
echo "📁 Available task files:"
ls -1 "$PRESET_DIR/tasks/" 2>/dev/null | while read task; do
  echo "   - $task"
done

# Check 6: Show parameters (if any)
if grep -q "^parameters:" "$PRESET_DIR/preset.yml"; then
  echo ""
  echo "⚙️  Parameters defined:"
  grep -A 20 "^parameters:" "$PRESET_DIR/preset.yml" | grep "^  " | head -10
fi

echo ""
echo "📝 To test installation:"
echo "   mooncake run test.yml"
echo "   # Where test.yml contains:"
echo "   # - preset: $PRESET"
