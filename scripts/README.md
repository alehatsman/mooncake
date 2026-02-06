# Scripts Directory

Essential scripts for building, testing, and releasing mooncake.

## Active Scripts

### Build & Release

#### `release.sh`
Creates a new release tag and triggers the release workflow.

```bash
./scripts/release.sh
# Or use: make release
```

### Documentation

#### `generate-preset-docs.py`
Generates the preset documentation page for MkDocs by scanning the `presets/` directory.

```bash
python3 ./scripts/generate-preset-docs.py
# Or use: make docs-gen
```

**What it does:**
- Scans `presets/` directory for preset definitions (both `name/preset.yml` and `name.yml` formats)
- Extracts metadata (name, description, version, parameters)
- Generates `docs/presets/available.md` with styled cards
- Cards use Material for MkDocs components and custom CSS (`docs/stylesheets/presets.css`)

**When to regenerate:**
- After adding a new preset
- After updating preset metadata
- Before building documentation (`make docs-build`)

**Related commands:**
```bash
make docs-gen      # Generate preset docs
make docs-serve    # Generate and serve docs locally at http://127.0.0.1:8000
make docs-build    # Build docs for deployment
```

### Docker Testing

These scripts test mooncake in Linux Docker environments:

#### `test-ubuntu-quick.sh`
Quick unit tests in Ubuntu Docker (matches CI environment exactly).

```bash
./scripts/test-ubuntu-quick.sh
# Or use: make test-docker
```

#### `test-ubuntu-docker.sh`
Full test suite in Ubuntu Docker (build + unit tests + coverage + race detector).

```bash
./scripts/test-ubuntu-docker.sh
# Or use: make test-docker-full
```

#### `test-docker.sh`
Build Linux binary and run tests on specific distros.

```bash
./scripts/test-docker.sh ubuntu-22.04 smoke
# Or use: make test-linux
```

Supported distros:
- ubuntu-22.04
- ubuntu-20.04
- alpine-3.19
- debian-12
- fedora-39

Test suites: smoke, integration, all

#### `run-integration-tests.sh`
Run integration tests (end-to-end testing).

```bash
./scripts/run-integration-tests.sh
# Or use: make test-integration
```

### Preset Testing

#### `test-preset-local.sh`
Test a specific preset locally with custom parameters.

```bash
./scripts/test-preset-local.sh docker
./scripts/test-preset-local.sh postgres version=14
# Or use: make test-preset PRESET=docker
```

#### `test-presets-docker.sh`
Test all presets in Docker Ubuntu environment (comprehensive testing).

```bash
./scripts/test-presets-docker.sh
# Or use: make test-presets
```

#### `test-presets-advanced.sh`
Advanced preset testing with various configurations and options.

```bash
./scripts/test-presets-advanced.sh
./scripts/test-presets-advanced.sh --quick    # Skip slow presets
# Or use: make test-presets-advanced
# Or use: make test-presets-quick
```

## Utility Scripts

These are kept for reference:

- `build_cli_binary.sh` - Old build script (use `make build` instead)
- `test-docker-all.sh` - Run tests on all distros (for comprehensive testing)
- `test-all-platforms.sh` - Multi-platform testing matrix
- `setup-docs.sh` - Documentation setup
- `verify-testing-setup.sh` - Verify testing infrastructure

## Usage Recommendations

**For local development:**
```bash
make test              # Quick local tests
make test-race         # With race detector
```

**For Linux compatibility testing:**
```bash
make test-docker       # Test in Linux environment (macOS/Windows users)
make test-linux        # Build Linux binary and smoke test
```

**For comprehensive testing:**
```bash
make ci                # Full CI suite (lint + race + scan)
make test-docker-full  # Full Docker test suite
make test-integration  # Integration tests
```
