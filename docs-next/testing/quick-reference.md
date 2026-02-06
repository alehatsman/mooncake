# Mooncake Testing - Quick Reference

> 📚 **Other Docs**: [Index](README.md) | [Testing Guide](guide.md) | [Architecture](architecture.md) | [Implementation](implementation-summary.md)

## One-Line Commands

```bash
# Quick smoke test (2 min)
make test-quick

# Test all distros (10 min)
make test-smoke

# Full local test suite (15 min)
make test-all-platforms

# Just unit tests (10 sec)
make test

# Just integration tests
make test-integration
```

## Test by Platform

```bash
# Linux - Specific distro
make test-docker-ubuntu     # Ubuntu 22.04
make test-docker-alpine     # Alpine 3.19
make test-docker-debian     # Debian 12
make test-docker-fedora     # Fedora 39

# Linux - All distros
make test-docker-all

# macOS - Native
make test

# Windows - Push to GitHub
git push  # Check GitHub Actions
```

## Direct Script Usage

```bash
# Test single distro with specific suite
./scripts/test-docker.sh ubuntu-22.04 smoke
./scripts/test-docker.sh alpine-3.19 integration

# Test all distros with specific suite
./scripts/test-docker-all.sh smoke
./scripts/test-docker-all.sh integration
./scripts/test-docker-all.sh all

# Run integration tests
./scripts/run-integration-tests.sh

# Complete local test
./scripts/test-all-platforms.sh
```

## Test a Single Config Manually

```bash
# Build binary
go build -v -o out/mooncake ./cmd

# Run a specific test
./out/mooncake run -c testing/fixtures/configs/smoke/001-version-check.yml
./out/mooncake run -c testing/fixtures/configs/integration/010-file-operations.yml
```

## Debug Docker Tests

```bash
# Build image
docker build -f testing/docker/ubuntu-22.04.Dockerfile -t mooncake-test-ubuntu .

# Run interactively
docker run -it mooncake-test-ubuntu /bin/sh

# Inside container:
mooncake --version
/test-runner.sh smoke
```

## View Test Results

```bash
# List results
ls -la testing/results/

# View specific log
cat testing/results/smoke-001-version-check.yml.log

# View all smoke test logs
cat testing/results/smoke-*.log
```

## CI Status

```bash
# Check CI status
gh run list --limit 5

# View specific run
gh run view <run-id>

# Watch current run
gh run watch
```

## Common Workflows

### Before Committing
```bash
make test                    # Quick unit tests
make test-quick              # Smoke test on Ubuntu
```

### Before Pushing
```bash
make test-all-platforms      # Full local suite
```

### After Pushing
Check GitHub Actions:

- https://github.com/alehatsman/mooncake/actions

### Adding New Test
```bash
# 1. Create test file
vim testing/fixtures/configs/smoke/005-my-test.yml

# 2. Test directly
./out/mooncake run -c testing/fixtures/configs/smoke/005-my-test.yml

# 3. Test in Docker
make test-quick

# 4. Commit and push
git add testing/fixtures/configs/smoke/005-my-test.yml
git commit -m "test: add my new test"
git push
```

## File Locations

```
testing/fixtures/configs/smoke/          # Smoke tests (<1 min)
testing/fixtures/configs/integration/    # Integration tests (5-10 min)
testing/docker/                          # Dockerfiles for each distro
scripts/test-*.sh                        # Test orchestration scripts
testing/results/                         # Test output logs (gitignored)
```

## Troubleshooting Quick Fixes

```bash
# Binary not found
env GOOS=linux GOARCH=amd64 go build -v -o out/mooncake-linux-amd64 ./cmd

# Docker not running
docker ps  # If fails, start Docker Desktop

# Scripts not executable
chmod +x scripts/*.sh testing/common/*.sh

# Clean Docker cache
docker system prune -f

# Clean test results
rm -rf testing/results/*
```

## Expected Times

| Command | Time | Purpose |
|---------|------|---------|
| `make test` | 10s | Quick Go unit tests |
| `make test-quick` | 2 min | Smoke test on Ubuntu |
| `make test-smoke` | 10 min | Smoke on all distros |
| `make test-docker-all` | 15 min | All tests, all distros |
| `make test-all-platforms` | 15 min | Native + Docker |
| CI complete | 7-10 min | All jobs in parallel |

## Need More Details?

- Full guide: `guide.md`
- Implementation: `implementation-summary.md`
- CI config: `.github/workflows/ci.yml`
- Makefile: `Makefile` (lines 70+)
