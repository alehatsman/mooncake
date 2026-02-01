# Quick Start

Get started with Mooncake in 30 seconds!

## Installation

```bash
go install github.com/alehatsman/mooncake@latest
```

Verify installation:
```bash
mooncake --help
```

## Your First Config

Create `config.yml`:

```yaml
- name: Hello Mooncake
  shell: echo "Chookity! Running on {{os}}/{{arch}}"

- name: Create a file
  file:
    path: /tmp/mooncake-test.txt
    state: file
    content: "Hello from Mooncake!"
```

## Run It

```bash
# Preview what will happen (safe!)
mooncake run --config config.yml --dry-run

# Run it for real
mooncake run --config config.yml
```

You'll see:
```
▶ Hello Mooncake
Chookity! Running on darwin/arm64
✓ Hello Mooncake

▶ Create a file
✓ Create a file
```

## Check the Result

```bash
cat /tmp/mooncake-test.txt
# Output: Hello from Mooncake!
```

## What Just Happened?

1. **First step** - Ran a shell command with system variables
2. **Second step** - Created a file with content

Mooncake automatically detected your OS and architecture!

## Next Steps

### Try More Features

```yaml
# Variables
- vars:
    app_name: MyApp
    version: "1.0"

# Conditionals
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

# Templates
- name: Render config
  template:
    src: ./config.j2
    dest: ~/.config/myapp.conf
```

### Explore Examples

Check out the [examples](../examples/index.md) for:

- **Beginner** (01-04) - Basic features
- **Intermediate** (05-07) - Templates and loops
- **Advanced** (08-10) - Tags and organization

### Read the Guide

Learn about all features in the [User Guide](../guide/core-concepts.md).

## Tips

!!! tip "Always use dry-run first"
    ```bash
    mooncake run --config config.yml --dry-run
    ```
    Preview changes before applying them!

!!! info "See available system facts"
    ```bash
    mooncake explain
    ```
    Shows OS, hardware, network info available as variables

!!! success "You're ready!"
    Continue to [Your First Config](first-config.md) for a deeper tutorial
