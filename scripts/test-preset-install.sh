#!/bin/bash
# Test Preset Installation
# Usage: ./scripts/test-preset-install.sh <preset-name>

set -e

PRESET=$1

if [ -z "$PRESET" ]; then
  echo "Usage: $0 <preset-name>"
  exit 1
fi

TEST_FILE="/tmp/mooncake-test-$PRESET.yml"

# Create test playbook
cat > "$TEST_FILE" << EOF
---
- name: Test $PRESET installation
  preset: $PRESET
  register: install_result

- name: Show result
  print:
    msg: "Installation result: {{ install_result }}"
EOF

echo "🧪 Testing preset: $PRESET"
echo "📝 Test file: $TEST_FILE"
echo ""

# Run installation
echo "▶️  Running installation..."
mooncake run "$TEST_FILE"

echo ""
echo "✅ Installation completed"
echo ""

# Create removal test
cat > "$TEST_FILE" << EOF
---
- name: Test $PRESET removal
  preset: $PRESET
  with:
    state: absent
  register: remove_result

- name: Show result
  print:
    msg: "Removal result: {{ remove_result }}"
EOF

read -p "Press Enter to test removal, or Ctrl+C to skip..."

echo "▶️  Running removal..."
mooncake run "$TEST_FILE"

echo ""
echo "✅ Removal completed"
echo ""
echo "🎉 Preset $PRESET verified successfully!"

# Cleanup
rm "$TEST_FILE"
