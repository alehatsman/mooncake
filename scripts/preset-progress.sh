#!/bin/bash
# Preset Documentation Progress Tracker
# Tracks progress on enhancing preset README files

set -euo pipefail

PRESET_DIR="presets"
MINIMAL_THRESHOLD=50
GOOD_THRESHOLD=150

# Colors
RED='\033[0;31m'
YELLOW='\033[0;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "================================================"
echo "  Preset Documentation Progress Tracker"
echo "================================================"
echo ""

# Count presets by status
minimal=0
needs_work=0
good=0
total=0

minimal_list=()
needs_work_list=()

while IFS= read -r readme; do
    total=$((total + 1))
    lines=$(wc -l < "$readme")
    preset=$(dirname "$readme" | sed 's|presets/||')

    if [ "$lines" -lt "$MINIMAL_THRESHOLD" ]; then
        minimal=$((minimal + 1))
        minimal_list+=("$lines $preset")
    elif [ "$lines" -lt "$GOOD_THRESHOLD" ]; then
        needs_work=$((needs_work + 1))
        needs_work_list+=("$lines $preset")
    else
        good=$((good + 1))
    fi
done < <(find "$PRESET_DIR" -name "README.md" | sort)

# Calculate percentages
pct_minimal=$(awk "BEGIN {printf \"%.1f\", ($minimal/$total)*100}")
pct_needs_work=$(awk "BEGIN {printf \"%.1f\", ($needs_work/$total)*100}")
pct_good=$(awk "BEGIN {printf \"%.1f\", ($good/$total)*100}")

# Summary
echo "📊 Summary:"
echo "  Total presets: $total"
echo ""
echo -e "  ${GREEN}✅ Good docs (≥150 lines):${NC}     $good ($pct_good%)"
echo -e "  ${YELLOW}⚠️  Needs work (50-149):${NC}      $needs_work ($pct_needs_work%)"
echo -e "  ${RED}❌ Minimal (< 50 lines):${NC}      $minimal ($pct_minimal%)"
echo ""

# Progress bar
bar_width=50
filled=$(awk "BEGIN {printf \"%.0f\", ($good/$total)*$bar_width}")
empty=$((bar_width - filled))
echo -n "  Progress: ["
printf "${GREEN}%${filled}s${NC}" | tr ' ' '█'
printf "${RED}%${empty}s${NC}" | tr ' ' '░'
echo "] $pct_good%"
echo ""

# Show top 20 minimal presets (good candidates for enhancement)
if [ ${#minimal_list[@]} -gt 0 ]; then
    echo "🎯 Top 20 Minimal Presets (Priority Targets):"
    printf '%s\n' "${minimal_list[@]}" | sort -n | head -20 | while read -r line; do
        count=$(echo "$line" | awk '{print $1}')
        name=$(echo "$line" | awk '{print $2}')
        printf "  ${RED}%3d${NC} lines - ${BLUE}%s${NC}\n" "$count" "$name"
    done
    echo ""
fi

# Show recently enhanced (≥150 lines)
echo "🌟 Recently Enhanced Presets:"
find "$PRESET_DIR" -name "README.md" -exec sh -c '
    lines=$(wc -l < "$1")
    if [ $lines -ge 150 ]; then
        preset=$(dirname "$1" | sed "s|presets/||")
        echo "$lines $preset"
    fi
' _ {} \; | sort -rn | head -20 | while read -r line; do
    count=$(echo "$line" | awk '{print $1}')
    name=$(echo "$line" | awk '{print $2}')
    printf "  ${GREEN}%3d${NC} lines - ${BLUE}%s${NC}\n" "$count" "$name"
done
echo ""

# Suggest next batch
echo "💡 Suggested Next Batch (3-4 related presets):"
echo ""

# Group by common categories
categories=(
    "csv|data|json|yaml|xml"
    "docker|podman|containerd|container"
    "kubernetes|k8s|helm|kubectl"
    "security|vault|age|gpg"
    "monitor|observability|prometheus"
    "ci|cd|jenkins|gitlab"
    "database|postgres|mysql|redis"
)

for category in "${categories[@]}"; do
    pattern=$(echo "$category" | cut -d'|' -f1)
    matches=()

    for item in "${minimal_list[@]}"; do
        name=$(echo "$item" | awk '{print $2}')
        if echo "$name" | grep -qiE "$category"; then
            matches+=("$item")
        fi
    done

    if [ ${#matches[@]} -ge 2 ]; then
        echo "  Category: $(echo $category | tr '|' ', ')"
        printf '%s\n' "${matches[@]}" | sort -n | head -4 | while read -r line; do
            count=$(echo "$line" | awk '{print $1}')
            name=$(echo "$line" | awk '{print $2}')
            printf "    ${YELLOW}%3d${NC} lines - ${BLUE}%s${NC}\n" "$count" "$name"
        done
        echo ""
        break  # Show only first category with matches
    fi
done

echo "================================================"
echo "💾 To update a preset:"
echo "   1. Edit presets/<name>/README.md"
echo "   2. Follow pattern in ~/.claude/.../PRESET_DOCS_GUIDE.md"
echo "   3. Aim for 150-400 lines"
echo "   4. git commit -m 'enhance <presets> for <category>'"
echo "================================================"
