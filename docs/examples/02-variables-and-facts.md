# 02 - Variables and System Facts

Learn how to define custom variables and use Mooncake's comprehensive system facts.

## What You'll Learn

- Defining custom variables with `vars`
- Using all available system facts
- Combining custom variables with system facts
- Using variables in file operations

## Quick Start

```bash
cd examples/02-variables-and-facts
mooncake run --config config.yml
```

## What It Does

1. Defines custom application variables
2. Displays all system facts (OS, hardware, network, software)
3. Creates files using both custom variables and system facts

## Key Concepts

### Custom Variables

Define your own variables:
```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    environment: development
```

Use them in commands and paths:
```yaml
- shell: echo "Running {{app_name}} v{{version}}"
```

### System Facts

Mooncake automatically collects system information:

**Basic:**
- `os` - Operating system (linux, darwin, windows)
- `arch` - Architecture (amd64, arm64)
- `hostname` - System hostname
- `user_home` - User's home directory

**Hardware:**
- `cpu_cores` - Number of CPU cores
- `memory_total_mb` - Total RAM in megabytes

**Distribution:**
- `distribution` - Distribution name (ubuntu, debian, macos, etc.)
- `distribution_version` - Full version (e.g., "22.04")
- `distribution_major` - Major version number

**Software:**
- `package_manager` - Detected package manager (apt, yum, brew, etc.)
- `python_version` - Installed Python version

**Network:**
- `ip_addresses` - Array of IP addresses
- `ip_addresses_string` - Comma-separated IP addresses

### Variable Substitution

Variables work everywhere:
```yaml
- file:
    path: "/tmp/{{app_name}}-{{version}}-{{os}}"
    state: directory
```

## Seeing All Facts

Run `mooncake explain` to see all facts for your system:
```bash
mooncake explain
```

## Next Steps

Continue to [03-files-and-directories](03-files-and-directories.md) to learn about file operations.
