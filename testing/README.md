# Testing Directory

Test fixtures and Docker testing infrastructure for mooncake.

## Active Testing Infrastructure

### `fixtures/`
Test fixtures used by Go unit tests.

```
fixtures/
├── configs/    # Test configuration files
└── templates/  # Test template files
```

### `docker/`
Multi-distro Docker test images for Linux compatibility testing.

**Available distros:**
- `ubuntu-22.04.Dockerfile` - Ubuntu 22.04 LTS
- `ubuntu-20.04.Dockerfile` - Ubuntu 20.04 LTS
- `alpine-3.19.Dockerfile` - Alpine Linux 3.19
- `debian-12.Dockerfile` - Debian 12 (Bookworm)
- `fedora-39.Dockerfile` - Fedora 39

**Usage:**
```bash
make test-docker       # Quick tests in Ubuntu (matches CI)
make test-docker-full  # Full test suite in Docker
make test-linux        # Build Linux binary + smoke tests
```

**Direct script usage:**
```bash
./scripts/test-docker.sh ubuntu-22.04 smoke
./scripts/test-docker.sh alpine-3.19 integration
```

### `common/`
Shared test runner scripts for Docker-based tests.

Contains `test-runner.sh` which is used by Docker containers to run different test suites (smoke, integration, all).

## Legacy Test Files

Kept for reference:
- `dotfiles.Dockerfile` - Old dotfiles testing
- `essentials/` - Old essentials testing
- `provisioning/` - Old provisioning testing
- `*.yml` files - Example/test YAML files

## Testing Matrix

### CI Pipeline (Automated)
The CI pipeline (`.github/workflows/ci.yml`) runs on every push:
- Unit tests on Ubuntu (native Go)
- Race detection tests
- Linting (golangci-lint)
- Security scans (gosec + govulncheck)
- Coverage reporting (Codecov)

### Local Development
```bash
# Quick development cycle
make test              # Unit tests
make test-race         # With race detector
make fmt               # Format code
make lint              # Lint

# Full CI locally
make ci                # Run complete CI suite
```

### Linux Compatibility Testing
Essential for macOS/Windows developers:

```bash
# Test in Linux environment
make test-docker       # Quick (matches CI exactly)
make test-docker-full  # Comprehensive

# Test Linux binary
make test-linux        # Build + smoke test
```

### Integration Testing
```bash
make test-integration  # Run integration tests
```

### Multi-Platform Testing
For comprehensive cross-platform verification:

```bash
# Test on specific distro
./scripts/test-docker.sh ubuntu-22.04 all
./scripts/test-docker.sh alpine-3.19 smoke

# Test on all distros (takes time!)
./scripts/test-docker-all.sh all
```

## Test Suites

- **smoke** - Quick smoke tests (basic functionality)
- **integration** - Integration tests (end-to-end scenarios)
- **all** - All tests (comprehensive)

## Why Docker Testing?

1. **Linux Compatibility** - Test on Linux without a Linux machine
2. **Distro Verification** - Ensure mooncake works across different Linux distributions
3. **CI Matching** - Local environment matches GitHub Actions
4. **Reproducibility** - Isolated, consistent test environment
5. **Integration Testing** - Test real-world deployment scenarios
