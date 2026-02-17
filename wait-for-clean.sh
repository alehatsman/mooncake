#!/bin/bash
# Wait for git working directory to be clean
# Used to coordinate between multiple agents working on the same repo

set -e

echo "⏳ Waiting for git working directory to be clean..."
echo "   (Another agent may be working on commits)"
echo ""

TIMEOUT=300  # 5 minutes timeout
ELAPSED=0
CHECK_INTERVAL=2

while [ $ELAPSED -lt $TIMEOUT ]; do
    # Check if working directory is clean
    if git diff --quiet && git diff --cached --quiet; then
        echo "✅ Working directory is clean!"
        echo ""
        git log --oneline -1
        echo ""
        exit 0
    fi

    # Show what's dirty
    if [ $((ELAPSED % 10)) -eq 0 ]; then
        echo "⏱  ${ELAPSED}s - Still waiting..."
        echo "   Modified files:"
        git status --short | head -3
    fi

    sleep $CHECK_INTERVAL
    ELAPSED=$((ELAPSED + CHECK_INTERVAL))
done

echo "❌ Timeout after ${TIMEOUT}s - working directory still dirty"
echo ""
git status --short
exit 1
