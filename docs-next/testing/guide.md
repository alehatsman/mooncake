# Mooncake Multi-Platform Testing Guide

> 📚 **Other Docs**: [Index](README.md) | [Quick Reference](quick-reference.md) | [Architecture](architecture.md) | [Implementation](implementation-summary.md)

This document describes the comprehensive testing setup for Mooncake across Linux, macOS, and Windows platforms.

## Overview

Mooncake uses a hybrid testing approach that balances fast local development with thorough CI validation:

- **Linux**: Docker containers for multiple distributions (local + CI)
- **macOS**: Native testing locally + GitHub Actions for CI
- **Windows**: GitHub Actions only (no local Windows testing required)

## Quick Start

### Local Testing

```bash
# Run unit tests on current platform
make test

# Quick smoke test (Linux via Docker, ~2 minutes)
make test-quick

# Test on specific Linux distro
make test-docker-ubuntu    # Ubuntu 22.04
make test-docker-alpine    # Alpine 3.19
make test-docker-debian    # Debian 12
make test-docker-fedora    # Fedora 39

# Run smoke tests on all Linux distros (~10 minutes)
make test-smoke

# Run integration tests locally
make test-integration

# Run ALL Docker tests (smoke + integration on all distros, ~15 minutes)
make test-docker-all

# Run complete local test suite (native + Docker)
make test-all-platforms
```

### CI Testing

Push to any branch and GitHub Actions will automatically run:

1. **Unit Tests** - Go tests on Ubuntu, macOS (Intel + Apple Silicon), Windows
2. **Docker Tests** - Smoke + integration tests on 5 Linux distros
3. **Integration Tests** - Full feature tests on Ubuntu, macOS, Windows

All CI jobs run in parallel for fast feedback (~5-10 minutes total).

## Test Structure

```
testing/
├── docker/                          # Docker configurations
│   ├── ubuntu-22.04.Dockerfile     # Ubuntu 22.04 LTS
│   ├── ubuntu-20.04.Dockerfile     # Ubuntu 20.04 LTS
│   ├── alpine-3.19.Dockerfile      # Alpine Linux (minimal)
│   ├── debian-12.Dockerfile        # Debian Bookworm
│   └── fedora-39.Dockerfile        # Fedora 39 (RPM-based)
├── common/
│   └── test-runner.sh              # Common test orchestration
├── fixtures/
│   ├── configs/
│   │   ├── smoke/                  # Fast validation tests (<1 min)
│   │   │   ├── 001-version-check.yml
│   │   │   ├── 002-simple-file.yml
│   │   │   ├── 003-simple-shell.yml
│   │   │   └── 004-simple-vars.yml
│   │   └── integration/            # Full feature tests (5-10 min)
│   │       ├── 010-file-operations.yml
│   │       ├── 020-loops.yml
│   │       ├── 030-conditionals.yml
│   │       └── 040-shell-commands.yml
│   └── templates/
│       └── test-template.j2        # Test template file
├── results/                         # Test results (gitignored)
└── README.md                        # This file
```

## Test Types

### Smoke Tests

Fast validation tests that verify basic functionality:

- Binary execution and version check
- Simple file operations
- Basic shell commands
- Variable substitution

**Runtime**: ~30 seconds per distro
**Purpose**: Catch obvious breakage quickly

### Integration Tests

Comprehensive tests that validate full features:

- Complete file management operations
- Loop iteration
- Conditional execution
- Complex shell commands
- Template rendering

**Runtime**: ~2-5 minutes per distro
**Purpose**: Ensure features work correctly across platforms

## Adding New Tests

### 1. Create a Test Config

Create a YAML file in the appropriate directory:

```yaml
# testing/fixtures/configs/smoke/005-my-test.yml
- name: My test step
  shell: echo "test"
  register: result

- name: Verify result
  shell: test "{{ result.stdout }}" = "test"
```

### 2. Test Locally

```bash
# Test with mooncake directly
./out/mooncake run -c testing/fixtures/configs/smoke/005-my-test.yml

# Test in Docker
make test-docker-ubuntu
```

### 3. Verify in CI

Push your changes and verify all platforms pass:

```bash
git add testing/fixtures/configs/smoke/005-my-test.yml
git commit -m "test: add my new test"
git push
```

Check GitHub Actions for results.

## Platform-Specific Notes

### Linux (Docker)

**Supported Distributions**:

- Ubuntu 22.04 LTS (Jammy)
- Ubuntu 20.04 LTS (Focal)
- Alpine 3.19 (minimal, musl libc)
- Debian 12 (Bookworm)
- Fedora 39 (RPM-based)

**Requirements**:

- Docker or Podman installed
- ~2GB disk space for images

**Tips**:

- Use `test-quick` for rapid iteration
- Images are cached after first build
- Add new distros by creating a new Dockerfile in `testing/docker/`

### macOS (Native)

**Testing Approach**:

- Local: Run tests natively on your Mac
- CI: Tests on both Intel (macos-13) and Apple Silicon (macos-latest)

**Requirements**:

- Go 1.25+
- No additional dependencies

**Tips**:

- Use `make test` for quick validation
- CI covers both architectures automatically

### Windows (CI Only)

**Testing Approach**:

- No local testing required (use Docker/WSL if needed)
- Automated testing via GitHub Actions on Windows Server

**Requirements**:

- None for local development
- Push to GitHub to test Windows

**Tips**:

- Use WSL2 with Docker for Linux testing on Windows
- Integration tests use `bash` shell (available via Git Bash on Windows)

## Troubleshooting

### Docker Build Fails

**Problem**: Docker build fails with "binary not found"

**Solution**:
```bash
# Ensure binary is built first
env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake-linux-amd64 ./cmd

# Or use the script which builds automatically
./scripts/test-docker.sh ubuntu-22.04
```

### Docker Not Running

**Problem**: `Cannot connect to the Docker daemon`

**Solution**:
```bash
# Check Docker is running
docker ps

# Start Docker Desktop (macOS/Windows)
# Or start docker service (Linux)
sudo systemctl start docker
```

### Tests Fail on Specific Distro

**Problem**: Tests pass on Ubuntu but fail on Alpine

**Solution**:
```bash
# Run container interactively
docker build -f testing/docker/alpine-3.19.Dockerfile -t mooncake-test-alpine .
docker run -it mooncake-test-alpine /bin/sh

# Debug inside container
/test-runner.sh smoke
```

### Test Results Not Visible

**Problem**: Can't see detailed test output

**Solution**:
```bash
# Check results directory
ls -la testing/results/

# View specific test log
cat testing/results/smoke-001-version-check.yml.log
```

### Integration Tests Fail Locally

**Problem**: Integration tests fail with "binary not found"

**Solution**:
```bash
# Build binary for current platform
go build -v -o out/mooncake ./cmd

# Run integration tests
./scripts/run-integration-tests.sh
```

## CI Workflow Details

### GitHub Actions Jobs

1. **unit-tests**: Go tests on 4 platforms (Ubuntu, macOS x2, Windows)
2. **docker-tests**: Smoke + integration on 5 Linux distros
3. **integration-tests**: Full feature tests on 3 platforms

### Viewing Results

1. Go to the [Actions tab](../../actions) in GitHub
2. Click on your workflow run
3. Expand job details to see logs
4. Download artifacts for detailed test results

### Coverage Reports

Code coverage is automatically calculated and uploaded to Codecov:

- Only runs on Ubuntu (to avoid duplicate reports)
- View at: https://codecov.io/gh/alehatsman/mooncake

## Performance

### Local Testing Times

- `make test`: ~10 seconds (Go unit tests)
- `make test-quick`: ~2 minutes (smoke tests on Ubuntu)
- `make test-smoke`: ~10 minutes (smoke on all distros)
- `make test-docker-all`: ~15 minutes (all tests, all distros)
- `make test-all-platforms`: ~15 minutes (native + Docker)

### CI Testing Times

- Unit tests: ~2-3 minutes per platform (parallel)
- Docker tests: ~5-7 minutes total (parallel builds)
- Integration tests: ~3-5 minutes per platform (parallel)
- **Total CI runtime**: ~7-10 minutes

## Best Practices

### For Developers

1. **Run `make test` before committing** - catches obvious issues
2. **Run `make test-quick` for local validation** - tests Linux compatibility
3. **Let CI handle comprehensive testing** - covers all platforms
4. **Add smoke tests for new features** - ensures basic functionality works
5. **Add integration tests for complex features** - validates complete workflows

### For Test Authors

1. **Keep smoke tests fast** - under 1 minute total
2. **Make tests idempotent** - can run multiple times
3. **Clean up test artifacts** - remove temp files/directories
4. **Use descriptive names** - clear what's being tested
5. **Test cross-platform** - avoid platform-specific commands in shared tests

### For CI

1. **Don't make CI required initially** - let it stabilize first
2. **Monitor flaky tests** - fix or remove unreliable tests
3. **Keep CI fast** - parallel execution, cached dependencies
4. **Fail fast** - stop on first critical failure
5. **Provide clear errors** - logs should point to root cause

## Future Enhancements

Potential improvements not in current scope:

- ARM64 Linux testing (currently x86_64 only)
- Performance benchmarking across platforms
- Windows WSL2 local testing support
- Visual regression testing for TUI output
- Test result dashboard/reporting
- Automated test generation from examples

## Contributing

When adding tests:

1. Add smoke test if testing basic functionality
2. Add integration test if testing complex features
3. Update this README if adding new test patterns
4. Ensure tests pass locally before pushing
5. Verify CI passes on all platforms

## Support

For issues or questions:

1. Check this README first
2. Look at existing test examples
3. Try running tests with verbose output
4. Check GitHub Actions logs for CI failures
5. Open an issue with reproduction steps

## License

Same as Mooncake project license.
