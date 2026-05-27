# Mooncake Multi-Architecture Testing Framework

Docker-based testing framework for Mooncake presets with full multi-architecture support (linux/amd64 and linux/arm64).

## Quick Start

```bash
cd testing-next

# Setup (first time only)
task setup-buildx

# Test on your native architecture
task test-ubuntu

# Test specific preset
task test-preset PRESET=docker

# Clean up
task clean-all
```

## Architecture

This framework supports **multi-architecture testing**:
- **linux/amd64** (x86_64) - Intel/AMD processors
- **linux/arm64** (aarch64) - ARM processors (M1/M2 Macs, Graviton, etc.)

## Structure

```
testing-next/
├── images/               # Dockerfiles for each distro
│   ├── Dockerfile.ubuntu
│   ├── Dockerfile.alpine
│   └── Dockerfile.fedora
├── lib/
│   └── common.sh        # Shared test functions
├── core-tests/
│   └── run.sh           # Core mooncake functionality tests
├── test-presets.sh      # Main preset test runner
├── Makefile             # Build and test orchestration
└── README.md
```

## Usage

### Initial Setup

```bash
# Setup Docker buildx for multi-platform builds (one-time)
task setup-buildx
```

### Building Binaries

```bash
# Build for specific architecture
task binary-amd64        # Linux x86_64
task binary-arm64        # Linux ARM64

# Build both
task binaries
```

### Building Images

```bash
# Build for specific architecture
task build-ubuntu-amd64
task build-ubuntu-arm64

# Build both
task build-ubuntu
```

### Running Tests

```bash
# Test on native architecture (auto-detects)
task test-ubuntu

# Test on specific architecture
task test-ubuntu-amd64
task test-ubuntu-arm64

# Test specific preset
task test-preset PRESET=nginx

# Test specific preset on specific arch
task test-preset-amd64 PRESET=docker
task test-preset-arm64 PRESET=kubernetes
```

### Results

Results are saved to `../artifacts/<distro>-<arch>/`:
- **`results.json`** - Machine-readable test results (JSON array)
- **`summary.md`** - Human-readable summary with pass/fail counts
- **`<preset>.log`** - Individual preset output logs

Example:
```bash
# View results
cat ../artifacts/ubuntu-amd64/summary.md
cat ../artifacts/ubuntu-amd64/results.json

# View specific preset log
cat ../artifacts/ubuntu-amd64/docker.log
```

### Cleanup

```bash
# Remove compiled binaries only
task clean-binaries

# Remove Docker images only
task clean-docker

# Remove everything (artifacts, binaries, images)
task clean-all
```

### Debugging

```bash
# Start interactive shell in container
task shell-ubuntu-amd64
task shell-ubuntu-arm64

# Inside container:
mooncake actions list
mooncake facts
ls /usr/share/mooncake/presets/ 2>/dev/null  # or wherever the container vendors them
```

## Multi-Architecture Details

### How It Works

1. **Build Stage**:
   - Compiles mooncake binaries for each target architecture using Go cross-compilation
   - Creates architecture-specific binaries: `mooncake-linux-amd64`, `mooncake-linux-arm64`

2. **Image Stage**:
   - Uses Docker buildx to create platform-specific images
   - Each image contains only the binary for its architecture
   - Verifies binary executes correctly during build

3. **Test Stage**:
   - Runs containers with explicit `--platform` flag
   - Tests execute natively on matching architecture
   - Cross-architecture testing uses emulation (QEMU)

### Architecture Detection

The Makefile auto-detects your native architecture:
- `task test-ubuntu` automatically runs the right version
- `uname -m` values: `x86_64` → amd64, `arm64`/`aarch64` → arm64

### Cross-Architecture Testing

```bash
# Test amd64 on ARM Mac (uses emulation)
task test-ubuntu-amd64

# Test arm64 on x86 machine (uses emulation)
task test-ubuntu-arm64
```

**Note**: Cross-architecture testing is slower due to QEMU emulation.

## Supported Distributions

Currently implemented:
- **Ubuntu 22.04** - Debian-based, apt package manager
- **Alpine 3.19** - Minimal musl-based, apk package manager
- **Fedora 39** - RHEL-based, dnf package manager

Adding more distros is straightforward - just create a new Dockerfile in `images/`.

## Performance

**Build times** (M1 Mac):
- Binary compilation: ~5s per architecture
- Image build (native): ~10-15s
- Image build (emulated): ~30-60s

**Test execution**:
- Single preset: ~2-10s (varies by complexity)
- All presets (~400): ~20-40 minutes (sequential)

## Troubleshooting

### "ERROR: mooncake binary failed to execute"

The Dockerfile includes a verification step. If this fails:
1. Check binary was built for correct architecture: `file mooncake-linux-amd64`
2. Verify platform matches: `docker image inspect mooncake-test:ubuntu-amd64`
3. Try rebuilding: `task clean-all && task build-ubuntu-amd64`

### "buildx: command not found"

Docker buildx isn't installed. Update Docker Desktop or install buildx manually:
```bash
docker buildx install
task setup-buildx
```

### Platform mismatch warnings

```
WARNING: The requested image's platform (linux/amd64) does not match
the detected host platform (linux/arm64/v8)
```

This is expected when testing cross-architecture. Tests will run via emulation.

### Slow cross-architecture tests

Cross-architecture tests use QEMU emulation which is slower. For faster tests:
- Use native architecture: `task test-ubuntu` (auto-detects)
- Or test on matching hardware (amd64 on x86, arm64 on ARM)

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Test Presets
on: [push, pull_request]

jobs:
  test:
    strategy:
      matrix:
        arch: [amd64, arm64]
        distro: [ubuntu]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2
      - name: Run tests
        run: |
          cd testing-next
          task test-${{ matrix.distro }}-${{ matrix.arch }}
      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: results-${{ matrix.distro }}-${{ matrix.arch }}
          path: artifacts/
```

## Design Decisions

### Why Multi-Arch?

- **Cloud agnostic**: Test on AWS Graviton (ARM), GCP (x86), etc.
- **Developer machines**: Support M1/M2 Macs and x86 laptops
- **Production parity**: Test on same architecture as deployment

### Why Pre-built Binaries?

- **Fast builds**: No Go installation in containers (~1GB savings)
- **Minimal images**: Runtime-only dependencies (~50MB vs ~500MB)
- **Reproducible**: Same binary tests everywhere

### Why Sequential Testing?

- **Simpler**: No race conditions or resource conflicts
- **Predictable**: Deterministic ordering
- **Sufficient**: ~400 presets complete in ~30 minutes

Parallel execution can be added later if needed.

## Contributing

When adding new presets, they're automatically discovered and tested. No changes needed unless:
- New distro support (add Dockerfile in `images/`)
- New architecture (add build target in Makefile)
- New test dimensions (parameters, platforms, etc.)

## License

Same as main Mooncake project.
