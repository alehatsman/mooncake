# sccache - Shared Compilation Cache

Compiler cache for Rust, C/C++, and CUDA builds. Speeds up compilation by caching previous builds locally or in the cloud (S3, Redis, GCS).

## Quick Start
```yaml
- preset: sccache
```

## Features
- **Multi-language**: Rust, C, C++, CUDA, Clang, GCC
- **Cloud backends**: S3, Redis, GCS, Azure, Memcached
- **Fast**: Dramatically reduces compilation times
- **CI/CD optimized**: Share cache across build machines
- **Drop-in**: Wraps existing compilers transparently
- **Statistics**: Track cache hit rates and savings

## Basic Usage
```bash
# Check version
sccache --version

# Show statistics
sccache --show-stats

# Clear cache statistics
sccache --zero-stats

# Stop server
sccache --stop-server

# Show cache location
sccache --show-cache-location

# Cache info
sccache --show-adv-stats
```

## Rust Integration
```bash
# Use with cargo
export RUSTC_WRAPPER=sccache
cargo build

# Or in .cargo/config.toml
[build]
rustc-wrapper = "sccache"

# Build with stats
cargo build
sccache --show-stats
```

## C/C++ Integration
```bash
# GCC/Clang
export CC="sccache gcc"
export CXX="sccache g++"
make

# CMake
cmake -DCMAKE_C_COMPILER_LAUNCHER=sccache \
      -DCMAKE_CXX_COMPILER_LAUNCHER=sccache .
make
```

## Configuration

### Local Cache (Default)
```bash
# Location
~/.cache/sccache/  # Linux
~/Library/Caches/Mozilla.sccache/  # macOS

# Max size (default 10GB)
export SCCACHE_CACHE_SIZE="20G"
```

### S3 Backend
```bash
# Configuration
export SCCACHE_BUCKET=my-build-cache
export SCCACHE_REGION=us-east-1
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...

# Optional endpoint (MinIO, etc)
export SCCACHE_ENDPOINT=https://s3.example.com

# Build with S3 cache
cargo build
```

### Redis Backend
```bash
# Configuration
export SCCACHE_REDIS=redis://localhost:6379

# With auth
export SCCACHE_REDIS=redis://:password@host:6379

# TTL (seconds, default 604800 = 7 days)
export SCCACHE_REDIS_TTL=86400
```

### GCS Backend
```bash
# Configuration
export SCCACHE_GCS_BUCKET=my-build-cache
export SCCACHE_GCS_CREDENTIALS_URL=file:///path/to/creds.json

# Optional key prefix
export SCCACHE_GCS_KEY_PREFIX=sccache/
```

### Azure Backend
```bash
# Configuration
export SCCACHE_AZURE_CONNECTION_STRING="DefaultEndpointsProtocol=https;..."

# Or use key
export SCCACHE_AZURE_BLOB_ENDPOINT=https://account.blob.core.windows.net
export SCCACHE_AZURE_ACCOUNT_NAME=myaccount
export SCCACHE_AZURE_ACCOUNT_KEY=...
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Install sccache
  run: |
    curl -L https://github.com/mozilla/sccache/releases/latest/download/sccache-v0.7.4-x86_64-unknown-linux-musl.tar.gz | tar xz
    sudo mv sccache-*/sccache /usr/local/bin/

- name: Configure sccache
  run: |
    echo "RUSTC_WRAPPER=sccache" >> $GITHUB_ENV
    echo "SCCACHE_BUCKET=my-github-cache" >> $GITHUB_ENV
    echo "SCCACHE_REGION=us-east-1" >> $GITHUB_ENV

- name: Build
  run: |
    cargo build --release
    sccache --show-stats
```

### GitLab CI
```yaml
variables:
  RUSTC_WRAPPER: sccache
  SCCACHE_BUCKET: my-gitlab-cache
  SCCACHE_REGION: us-east-1

build:
  before_script:
    - export PATH="/usr/local/bin:$PATH"
  script:
    - cargo build --release
    - sccache --show-stats
```

### Docker
```dockerfile
# Install sccache
RUN curl -L https://github.com/mozilla/sccache/releases/latest/download/sccache-v0.7.4-x86_64-unknown-linux-musl.tar.gz | tar xz && \
    mv sccache-*/sccache /usr/local/bin/

# Configure
ENV RUSTC_WRAPPER=sccache
ENV SCCACHE_BUCKET=my-docker-cache

# Build
RUN cargo build --release && sccache --show-stats
```

## Advanced Configuration

### Custom Port
```bash
# Server port (default random)
export SCCACHE_SERVER_PORT=4226
```

### Idle Timeout
```bash
# Shutdown after idle (seconds, default 600)
export SCCACHE_IDLE_TIMEOUT=300
```

### Log Level
```bash
# Logging (off, error, warn, info, debug, trace)
export SCCACHE_LOG=info
export RUST_LOG=sccache=debug
```

### Readonly Mode
```bash
# Only read from cache, never write
export SCCACHE_READONLY=1
```

### Disable for Specific Files
```bash
# Skip caching for specific files
export SCCACHE_IGNORED_FILES=/path/to/generated/
```

## Statistics
```bash
# Show stats
sccache --show-stats

# Example output:
# Compile requests:    1000
# Compile hits:        800 (80%)
# Cache misses:        200
# Cache timeouts:      0
# Cache read errors:   0
# Cache write errors:  0
# Cache location:      Local
# Cache size:          5.2 GB
# Max cache size:      10 GB
```

## Performance Tips
- Use S3 or Redis for distributed builds
- Set appropriate SCCACHE_CACHE_SIZE
- Monitor cache hit rates
- Use separate buckets per project
- Enable compression for S3
- Consider local cache for laptop builds
- Use Redis for high-frequency CI

## Troubleshooting

### Low Cache Hit Rate
```bash
# Check stats
sccache --show-stats

# Possible causes:
# - Different compiler versions
# - Unstable timestamps
# - Generated code in source
# - Cache too small (evictions)
```

### Connection Errors
```bash
# S3 access denied
aws s3 ls s3://my-bucket  # Verify permissions

# Redis connection failed
redis-cli -h host -p 6379 ping

# Enable debug logging
export SCCACHE_LOG=debug
cargo clean && cargo build
```

### Cache Not Working
```bash
# Verify wrapper is active
echo $RUSTC_WRAPPER

# Check server running
sccache --show-stats

# Restart server
sccache --stop-server
cargo build
```

## Comparison with ccache
| Feature | sccache | ccache |
|---------|---------|--------|
| Languages | Rust, C/C++, CUDA | C/C++ only |
| Cloud backends | Yes (S3, Redis, GCS) | No |
| Distributed builds | Yes | Limited |
| Rust support | Native | Via wrapper |
| Active development | Yes | Yes |

## Real-World Examples

### Monorepo Build
```yaml
# Share cache across all projects
- name: Configure sccache
  shell: |
    export RUSTC_WRAPPER=sccache
    export SCCACHE_BUCKET=monorepo-cache
    export SCCACHE_GCS_KEY_PREFIX=project-a/
  register: setup

- name: Build project A
  shell: cargo build --release
  cwd: /workspace/project-a

- name: Build project B
  shell: cargo build --release
  cwd: /workspace/project-b

- name: Show cache efficiency
  shell: sccache --show-stats
```

### Multi-Architecture Builds
```bash
# Linux build
docker run --rm -e SCCACHE_BUCKET=cross-arch \
  rust:latest cargo build --target x86_64-unknown-linux-gnu

# ARM build (shares cache)
docker run --rm -e SCCACHE_BUCKET=cross-arch \
  rust:latest cargo build --target aarch64-unknown-linux-gnu
```

## Advanced Usage

### Mooncake Installation
```yaml
- preset: sccache
  become: true

- name: Configure for CI
  shell: |
    mkdir -p /etc/sccache
    cat > /etc/sccache/config <<EOF
    [cache.s3]
    bucket = "ci-cache"
    region = "us-east-1"
    EOF
```

### Environment Configuration
```yaml
- name: Setup sccache environment
  vars:
    sccache_config:
      bucket: "{{ project_name }}-cache"
      region: "{{ aws_region }}"
  template:
    content: |
      export RUSTC_WRAPPER=sccache
      export SCCACHE_BUCKET={{ sccache_config.bucket }}
      export SCCACHE_REGION={{ sccache_config.region }}
      export SCCACHE_CACHE_SIZE=20G
    dest: /etc/profile.d/sccache.sh
    mode: "0644"
  become: true
```

## Platform Support
- ✅ Linux (glibc, musl)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (MSVC)
- ✅ FreeBSD

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Speed up CI/CD build times
- Share compilation cache across build agents
- Reduce cloud build costs
- Monitor cache effectiveness
- Optimize distributed build systems
- Cache management automation

## Uninstall
```yaml
- preset: sccache
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/mozilla/sccache
- Documentation: https://github.com/mozilla/sccache/blob/main/docs/
- Releases: https://github.com/mozilla/sccache/releases
- Search: "sccache rust", "sccache s3 setup", "sccache ci cd"
