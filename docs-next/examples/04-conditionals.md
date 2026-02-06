# 04 - Conditionals

Learn how to conditionally execute steps based on system properties or variables.

## What You'll Learn

- Using `when` for conditional execution
- OS and architecture detection
- Complex conditions with logical operators
- Combining conditionals with tags

## Quick Start

```bash
cd examples/04-conditionals

# Run all steps (only matching conditions will execute)
mooncake run --config config.yml

# Run only dev-tagged steps
mooncake run --config config.yml --tags dev
```

## What It Does

1. Demonstrates steps that always run
2. Shows OS-specific steps (macOS vs Linux)
3. Shows architecture-specific steps
4. Demonstrates tag filtering

## Key Concepts

### Basic Conditionals

Use `when` to conditionally execute steps:
```yaml
- name: Linux only
  shell: echo "Running on Linux"
  when: os == "linux"
```

### Available System Variables

- `os` - darwin, linux, windows
- `arch` - amd64, arm64, 386, etc.
- `distribution` - ubuntu, debian, centos, macos, etc.
- `distribution_major` - major version number
- `package_manager` - apt, yum, brew, pacman, etc.

### Comparison Operators

- `==` - equals
- `!=` - not equals
- `>`, `<`, `>=`, `<=` - comparisons
- `&&` - logical AND
- `||` - logical OR
- `!` - logical NOT

### Complex Conditions

```yaml
- name: ARM Mac only
  shell: echo "ARM-based macOS"
  when: os == "darwin" && arch == "arm64"

- name: High memory systems
  shell: echo "Lots of RAM!"
  when: memory_total_mb >= 16000

- name: Ubuntu 20+
  shell: apt update
  when: distribution == "ubuntu" && distribution_major >= "20"
```

### Tags vs Conditionals

**Conditionals (`when`):**

- Evaluated at runtime
- Based on system facts or variables
- Step-level decision making

**Tags:**

- User-controlled filtering
- Specified via CLI `--tags` flag
- Workflow-level decision making

## Testing Different Conditions

Try these commands:
```bash
# See which steps run on your system
mooncake run --config config.yml

# Preview without executing
mooncake run --config config.yml --dry-run

# Run only development steps
mooncake run --config config.yml --tags dev
```

## Next Steps

Continue to [05-templates](05-templates.md) to learn about template rendering.
