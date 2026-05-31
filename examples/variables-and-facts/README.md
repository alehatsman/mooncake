# 02 - Variables and System Facts

Learn how to define custom variables and use Mooncake's comprehensive system facts.

## What You'll Learn

- Defining custom variables with `vars`
- Using all available system facts
- Combining custom variables with system facts
- Using variables in file operations

## Quick Start

```bash
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

### Built-in Directory Variables

Two directory variables are always in scope, mainly useful when authoring
reusable components/modules:

- `invocation_dir` - the working directory mooncake was launched from (the
  project under management). This is also where `shell:`/`cmd:` steps run,
  so it's the dir a shared gate operates on. Constant for the whole run.
- `component_dir` - the directory of the file (or `use:`d component) that
  *declares* the current step. For a module fetched from a registry this is
  its cache dir. Lets a component reference its OWN bundled assets and run
  them against the consumer's code, e.g.:

  ```yaml
  - shell:
      cmd: "bash {{ component_dir }}/scripts/lint.sh"
  ```

  The script path resolves to the component, but the command still runs in
  `invocation_dir`.

### Variable Substitution

Variables work everywhere:
```yaml
- file:
    path: "/tmp/{{app_name}}-{{version}}-{{os}}"
    state: directory
```

## Seeing All Facts

Run `mooncake facts` to see all facts for your system:
```bash
mooncake facts
```

## Next Steps

→ Continue to [03-files-and-directories](../03-files-and-directories/) to learn about file operations.
