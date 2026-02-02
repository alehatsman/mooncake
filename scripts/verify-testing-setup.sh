#!/bin/bash
# Verification script for multi-platform testing setup

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║        Mooncake Multi-Platform Testing Setup Verification        ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo ""

ERRORS=0

check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✓${NC} $1"
    else
        echo -e "${RED}✗${NC} $1 - MISSING"
        ((ERRORS++))
    fi
}

check_dir() {
    if [ -d "$1" ]; then
        echo -e "${GREEN}✓${NC} $1/"
    else
        echo -e "${RED}✗${NC} $1/ - MISSING"
        ((ERRORS++))
    fi
}

check_executable() {
    if [ -x "$1" ]; then
        echo -e "${GREEN}✓${NC} $1 (executable)"
    else
        echo -e "${YELLOW}⚠${NC} $1 (not executable - run: chmod +x $1)"
        ((ERRORS++))
    fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking Docker Infrastructure..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_file "testing/docker/ubuntu-22.04.Dockerfile"
check_file "testing/docker/ubuntu-20.04.Dockerfile"
check_file "testing/docker/alpine-3.19.Dockerfile"
check_file "testing/docker/debian-12.Dockerfile"
check_file "testing/docker/fedora-39.Dockerfile"
check_executable "testing/common/test-runner.sh"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking Test Fixtures..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_dir "testing/fixtures/configs/smoke"
check_file "testing/fixtures/configs/smoke/001-version-check.yml"
check_file "testing/fixtures/configs/smoke/002-simple-file.yml"
check_file "testing/fixtures/configs/smoke/003-simple-shell.yml"
check_file "testing/fixtures/configs/smoke/004-simple-vars.yml"

check_dir "testing/fixtures/configs/integration"
check_file "testing/fixtures/configs/integration/010-file-operations.yml"
check_file "testing/fixtures/configs/integration/020-loops.yml"
check_file "testing/fixtures/configs/integration/030-conditionals.yml"
check_file "testing/fixtures/configs/integration/040-shell-commands.yml"

check_file "testing/fixtures/templates/test-template.j2"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking Test Scripts..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_executable "scripts/test-docker.sh"
check_executable "scripts/test-docker-all.sh"
check_executable "scripts/test-all-platforms.sh"
check_executable "scripts/run-integration-tests.sh"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking Documentation..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_dir "docs/testing"
check_file "docs/testing/guide.md"
check_file "docs/testing/implementation-summary.md"
check_file "docs/testing/quick-reference.md"
check_file "docs/testing/architecture.md"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking Makefile Targets..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
TARGETS=("test-quick" "test-smoke" "test-integration" "test-docker-ubuntu" "test-docker-alpine" "test-docker-debian" "test-docker-fedora" "test-docker-all" "test-all-platforms")

for target in "${TARGETS[@]}"; do
    if grep -q "^${target}:" Makefile; then
        echo -e "${GREEN}✓${NC} make $target"
    else
        echo -e "${RED}✗${NC} make $target - NOT FOUND"
        ((ERRORS++))
    fi
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking CI Configuration..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_file ".github/workflows/ci.yml"

if grep -q "unit-tests:" .github/workflows/ci.yml; then
    echo -e "${GREEN}✓${NC} unit-tests job"
else
    echo -e "${RED}✗${NC} unit-tests job - NOT FOUND"
    ((ERRORS++))
fi

if grep -q "docker-tests:" .github/workflows/ci.yml; then
    echo -e "${GREEN}✓${NC} docker-tests job"
else
    echo -e "${RED}✗${NC} docker-tests job - NOT FOUND"
    ((ERRORS++))
fi

if grep -q "integration-tests:" .github/workflows/ci.yml; then
    echo -e "${GREEN}✓${NC} integration-tests job"
else
    echo -e "${RED}✗${NC} integration-tests job - NOT FOUND"
    ((ERRORS++))
fi

if grep -q "windows-latest" .github/workflows/ci.yml; then
    echo -e "${GREEN}✓${NC} Windows testing enabled"
else
    echo -e "${RED}✗${NC} Windows testing - NOT ENABLED"
    ((ERRORS++))
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Checking Dependencies..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if command -v docker &> /dev/null; then
    echo -e "${GREEN}✓${NC} Docker installed"
    docker --version | sed 's/^/  /'
else
    echo -e "${YELLOW}⚠${NC} Docker not found (required for Linux testing)"
fi

if command -v go &> /dev/null; then
    echo -e "${GREEN}✓${NC} Go installed"
    go version | sed 's/^/  /'
else
    echo -e "${RED}✗${NC} Go not found (REQUIRED)"
    ((ERRORS++))
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Directory Structure Check..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
check_dir "testing/docker"
check_dir "testing/common"
check_dir "testing/fixtures"
check_dir "testing/fixtures/configs"
check_dir "testing/fixtures/configs/smoke"
check_dir "testing/fixtures/configs/integration"
check_dir "testing/fixtures/templates"
check_dir "scripts"
check_dir "out"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed!${NC}"
    echo ""
    echo "Setup is complete and ready for use."
    echo ""
    echo "Next steps:"
    echo "  1. Build mooncake: go build -v -o out/mooncake ./cmd"
    echo "  2. Run quick test: make test-quick"
    echo "  3. Read the docs: cat docs/testing/guide.md"
    exit 0
else
    echo -e "${RED}✗ Found $ERRORS issue(s)${NC}"
    echo ""
    echo "Please fix the issues above before proceeding."
    echo ""
    echo "Common fixes:"
    echo "  • Make scripts executable: chmod +x scripts/*.sh testing/common/*.sh"
    echo "  • Create missing directories: mkdir -p out testing/results"
    exit 1
fi
