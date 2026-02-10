#!/usr/bin/env bash
# merge-properties.sh - Merge auto-generated properties into manual documentation
#
# This script takes auto-generated properties tables from schema.json and
# merges them with manually maintained documentation, preserving examples
# and use cases while ensuring properties are always in sync with the schema.
#
# Usage:
#   ./scripts/merge-properties.sh
#
# Architecture:
# - mooncake generates properties from schema.json
# - This script merges generated content into docs-next/guide/config/actions.md
# - Manual content (examples, use cases) is preserved
# - Properties tables are replaced with auto-generated versions

set -euo pipefail

# Configuration
GENERATED_PROPERTIES="${GENERATED_PROPERTIES:-docs-next/generated/properties.md}"
MANUAL_DOCS="${MANUAL_DOCS:-docs-next/guide/config/actions.md}"
BACKUP_DIR=".tmp/docs-backup"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}✓${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $*"
}

log_error() {
    echo -e "${RED}✗${NC} $*"
}

# Check if files exist
if [ ! -f "$GENERATED_PROPERTIES" ]; then
    log_error "Generated properties file not found: $GENERATED_PROPERTIES"
    log_info "Run: ./out/mooncake docs generate --section action-properties --output $GENERATED_PROPERTIES"
    exit 1
fi

if [ ! -f "$MANUAL_DOCS" ]; then
    log_error "Manual documentation file not found: $MANUAL_DOCS"
    exit 1
fi

# Create backup
mkdir -p "$BACKUP_DIR"
cp "$MANUAL_DOCS" "$BACKUP_DIR/actions.md.$(date +%Y%m%d-%H%M%S)"
log_info "Created backup in $BACKUP_DIR"

# For now, just report what would be merged
log_info "Generated properties available at: $GENERATED_PROPERTIES"
log_info "Manual documentation at: $MANUAL_DOCS"
log_warn "Merge functionality not yet implemented"
log_info "Properties can be manually reviewed and integrated"

# Future implementation would:
# 1. Parse generated properties markdown
# 2. Find corresponding action sections in manual docs
# 3. Locate properties table markers (or create them)
# 4. Replace properties table content
# 5. Preserve all surrounding manual content

exit 0
