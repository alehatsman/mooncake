# Mooncake Testing Documentation

Complete testing documentation for the Mooncake multi-platform testing infrastructure.

## 📚 Documentation Guide

### For Getting Started

**[Quick Reference](quick-reference.md)** - Start here for common commands

- One-line commands for daily use
- Quick examples and usage patterns
- Troubleshooting quick fixes
- Perfect for daily development

### For Understanding the System

**[Testing Guide](guide.md)** - Complete testing guide

- Overview of the testing setup
- Detailed instructions for local and CI testing
- Test structure and organization
- Platform-specific notes (Linux, macOS, Windows)
- Comprehensive troubleshooting section
- Best practices

**[Architecture](architecture.md)** - System architecture and design

- Visual diagrams of the testing infrastructure
- How components work together
- Data flow and execution paths
- Platform coverage matrix
- Design decisions explained

### For Implementation Details

**[Implementation Summary](implementation-summary.md)** - What was built

- Complete list of files created and modified
- Phase-by-phase implementation breakdown
- Success criteria checklist
- Known limitations and trade-offs

## Quick Start

```bash
# Fast smoke test (2 minutes)
make test-quick

# Test all Linux distros (10 minutes)
make test-smoke

# Complete local test suite (15 minutes)
make test-all-platforms
```

## 📖 Documentation Map

```
Testing Documentation Structure:
├── README.md (this file)           # Documentation index
├── quick-reference.md              # Quick commands and examples
├── guide.md                        # Complete testing guide
├── architecture.md                 # System architecture
└── implementation-summary.md       # Implementation details
```

## Find What You Need

| I want to... | Read this... |
|--------------|--------------|
| Run tests quickly | [Quick Reference](quick-reference.md) |
| Understand the full setup | [Testing Guide](guide.md) |
| See how it works | [Architecture](architecture.md) |
| Know what was implemented | [Implementation Summary](implementation-summary.md) |
| Add new tests | [Testing Guide - Adding Tests](guide.md#adding-new-tests) |
| Troubleshoot issues | [Testing Guide - Troubleshooting](guide.md#troubleshooting) |
| Understand design decisions | [Architecture - Design Decisions](architecture.md#key-design-decisions) |

## 🌍 Platform Coverage

- **Linux**: Ubuntu 22.04/20.04, Alpine 3.19, Debian 12, Fedora 39
- **macOS**: Intel (macos-13) + Apple Silicon (macos-latest)
- **Windows**: Windows Server (GitHub Actions)

## Key Commands

```bash
# Quick validation
make test              # Unit tests (10 sec)
make test-quick        # Smoke test on Ubuntu (2 min)

# Linux testing
make test-docker-ubuntu    # Test on Ubuntu
make test-docker-alpine    # Test on Alpine
make test-smoke            # Smoke tests all distros (10 min)

# Complete testing
make test-all-platforms    # Native + Docker (15 min)

# Verification
./scripts/verify-testing-setup.sh    # Verify setup
```

## 🔗 External Links

- [Main README](../../README.md)
- [GitHub Actions Workflow](../../.github/workflows/ci.yml)
- [Test Fixtures](../../testing/fixtures/)
- [Test Scripts](../../scripts/)

## Next Steps

1. **New to testing?** Start with [Quick Reference](quick-reference.md)
2. **Setting up?** Read [Testing Guide](guide.md)
3. **Want to understand?** Check [Architecture](architecture.md)
4. **Need implementation details?** See [Implementation Summary](implementation-summary.md)

---

**Last Updated**: 2026-02-05
**Status**:  Fully Implemented and Tested
