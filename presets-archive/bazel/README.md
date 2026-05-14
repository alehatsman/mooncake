# Bazel - Fast, Scalable Build System

Google's build tool that supports multi-language projects with incremental builds, remote caching, and distributed execution.

## Quick Start
```yaml
- preset: bazel
```

## Features
- **Multi-language support**: Java, C++, Python, Go, Android, iOS, and more
- **Incremental builds**: Only rebuilds changed code
- **Remote caching**: Share build artifacts across teams
- **Distributed execution**: Scale builds across multiple machines
- **Hermetic builds**: Reproducible and deterministic
- **Large monorepos**: Designed for Google-scale codebases

## Basic Usage
```bash
# Build a target
bazel build //main:hello-world

# Run a binary
bazel run //main:hello-world

# Test targets
bazel test //tests:all

# Query dependencies
bazel query 'deps(//main:app)'

# Clean build cache
bazel clean

# Build with optimization
bazel build -c opt //main:app

# Verbose build output
bazel build //main:app --verbose_failures
```

## Advanced Configuration

```yaml
# Install Bazel
- preset: bazel
  register: bazel_result

# Verify installation
- name: Check Bazel version
  shell: bazel version
  register: version

- name: Display version
  print: "Bazel version {{ version.stdout }}"

# Build project
- name: Build application
  shell: bazel build //...
  cwd: /path/to/project
  register: build_result

- name: Run tests
  shell: bazel test //tests:all
  cwd: /path/to/project
```

## Project Structure

### WORKSPACE File
```python
# WORKSPACE - Defines external dependencies
workspace(name = "my_project")

# Load rules
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Download dependencies
http_archive(
    name = "rules_go",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/v0.39.0/rules_go-v0.39.0.zip"],
)
```

### BUILD File
```python
# BUILD - Defines build targets
load("@rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "hello_lib",
    srcs = ["hello.go"],
    visibility = ["//visibility:public"],
)

go_binary(
    name = "hello",
    srcs = ["main.go"],
    deps = [":hello_lib"],
)
```

## Common Build Targets

```bash
# Build everything
bazel build //...

# Build specific package
bazel build //main/...

# Build with debugging symbols
bazel build --compilation_mode=dbg //main:app

# Build for production
bazel build -c opt //main:app

# Build Android APK
bazel build //android:app

# Build iOS app
bazel build //ios:app
```

## Testing

```bash
# Run all tests
bazel test //...

# Run specific test
bazel test //tests:unit_test

# Run tests matching pattern
bazel test //tests:all --test_filter=TestLogin*

# Run with test output
bazel test //tests:all --test_output=all

# Run tests in parallel
bazel test //tests:all --jobs=8

# Coverage report
bazel coverage //tests:all
```

## Query and Analysis

```bash
# Show dependencies
bazel query 'deps(//main:app)'

# Find reverse dependencies
bazel query 'rdeps(//..., //lib:utils)'

# Show build graph
bazel query --output=graph 'deps(//main:app)' > graph.dot

# List all targets
bazel query '//...'

# Find test targets
bazel query 'kind(.*_test, //...)'

# Analyze build
bazel aquery 'deps(//main:app)'
```

## Caching

### Local Cache
```bash
# Use disk cache
bazel build //main:app --disk_cache=/tmp/bazel-cache

# Set cache size
bazel build //main:app --experimental_disk_cache_gc_idle_delay=1h
```

### Remote Cache
```bash
# Use remote cache
bazel build //main:app \
  --remote_cache=grpc://cache.example.com:9092

# Build with remote execution
bazel build //main:app \
  --remote_executor=grpc://executor.example.com:8980 \
  --remote_cache=grpc://cache.example.com:9092
```

## Configuration

### .bazelrc
```bash
# .bazelrc - Bazel configuration file
# Build settings
build --jobs=8
build --compilation_mode=opt

# Test settings
test --test_output=errors
test --test_verbose_timeout_warnings

# Remote cache
build --remote_cache=grpc://cache.example.com:9092
build --remote_timeout=60s

# Common flags
common --experimental_ui_deduplicate
common --experimental_allow_tags_propagation
```

### Platform-specific Config
```bash
# .bazelrc
build:linux --cxxopt=-std=c++17
build:macos --cxxopt=-std=c++17
build:windows --cxxopt=/std:c++17

# Use with
bazel build --config=linux //main:app
```

## Real-World Examples

### CI/CD Pipeline Build
```yaml
# Build and test in CI
- preset: bazel
  become: true

- name: Configure Bazel
  template:
    dest: /workspace/.bazelrc
    content: |
      build --remote_cache=grpc://{{ cache_server }}:9092
      build --jobs={{ cpu_cores }}
      test --test_output=errors

- name: Build all targets
  shell: bazel build //...
  cwd: /workspace
  register: build

- name: Run tests
  shell: bazel test //tests:all
  cwd: /workspace
  register: tests

- name: Check test results
  assert:
    command:
      cmd: echo {{ tests.rc }}
      exit_code: 0
```

### Monorepo Builds
```bash
# Build specific services
bazel build //services/api/...
bazel build //services/web/...
bazel build //services/worker/...

# Test only changed code (with bazelisk)
bazel test --test_tag_filters=-integration $(bazel query 'tests(...)')

# Build all containers
bazel build //docker:all
```

### Multi-platform Builds
```bash
# Build for different platforms
bazel build --platforms=//platforms:linux_x86_64 //main:app
bazel build --platforms=//platforms:linux_arm64 //main:app
bazel build --platforms=//platforms:darwin_x86_64 //main:app

# Cross-compilation
bazel build --cpu=k8 //main:app              # Linux x86_64
bazel build --cpu=darwin_arm64 //main:app   # macOS ARM64
```

### Container Image Builds
```python
# BUILD file
load("@rules_docker//container:container.bzl", "container_image")

container_image(
    name = "app_image",
    base = "@distroless_base//image",
    files = [":app"],
    cmd = ["/app"],
)
```

```bash
# Build and push image
bazel run //main:app_image -- --norun
docker tag bazel/main:app_image myregistry.com/app:latest
docker push myregistry.com/app:latest
```

## Performance Optimization

```bash
# Use local resources efficiently
bazel build //... --local_cpu_resources=8 --local_ram_resources=16384

# Limit network threads
bazel build //... --experimental_http_download_scheduler=8

# Use sandbox
bazel build //... --spawn_strategy=sandboxed

# Profile build
bazel build //main:app --profile=/tmp/profile.json
bazel analyze-profile /tmp/profile.json
```

## Troubleshooting

### Build Failures
```bash
# Verbose output
bazel build //main:app --verbose_failures

# Show full command lines
bazel build //main:app -s

# Debug sandbox
bazel build //main:app --sandbox_debug

# Clean and rebuild
bazel clean --expunge
bazel build //main:app
```

### Cache Issues
```bash
# Disable cache
bazel build //main:app --noremote_cache

# Clear local cache
bazel clean

# Verify cache connectivity
curl -v grpc://cache.example.com:9092
```

### Dependency Problems
```bash
# Show why target depends on another
bazel query 'somepath(//main:app, //lib:old_version)'

# Find unused dependencies
bazel query 'deps(//main:app)' --output graph
```

### Permission Errors
```bash
# Fix permissions on cache
chmod -R u+rw ~/.cache/bazel

# Or use different cache location
bazel build //main:app --disk_cache=/tmp/bazel-cache
```

## Platform Support
- ✅ Linux (apt, dnf, Homebrew, binary)
- ✅ macOS (Homebrew, binary)
- ✅ Windows (binary, Chocolatey)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Build large monorepos in CI/CD pipelines
- Implement incremental builds for faster feedback
- Share build artifacts across team via remote cache
- Cross-compile applications for multiple platforms
- Manage multi-language project dependencies
- Generate reproducible builds for compliance
- Scale builds with distributed execution

## Uninstall
```yaml
- preset: bazel
  with:
    state: absent
```

## Resources
- Official docs: https://bazel.build/docs
- Rules: https://bazel.build/rules
- GitHub: https://github.com/bazelbuild/bazel
- Community: https://bazel.build/community
- Search: "bazel tutorial", "bazel build rules", "bazel monorepo"
