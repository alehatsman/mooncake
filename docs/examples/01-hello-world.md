# 01 - Hello World

**Start here!** This is the simplest possible Mooncake configuration.

## What You'll Learn

- Running basic shell commands
- Using global system variables
- Multi-line shell commands

## Quick Start

```bash
cd examples/01-hello-world
mooncake run --config config.yml
```

## What It Does

1. Prints a hello message
2. Runs system commands to show OS info
3. Uses Mooncake's global variables to display OS and architecture

## Key Concepts

### Shell Commands

Execute commands with the `shell` action:
```yaml
- name: Print message
  shell: echo "Hello!"
```

### Multi-line Commands

Use `|` for multiple commands:
```yaml
- name: Multiple commands
  shell: |
    echo "First command"
    echo "Second command"
```

### Global Variables

Mooncake automatically provides system information:
- `{{os}}` - Operating system (linux, darwin, windows)
- `{{arch}}` - Architecture (amd64, arm64, etc.)

## Output Example

```
▶ Print hello message
Hello from Mooncake!
✓ Print hello message

▶ Print system info
OS: Darwin
Arch: arm64
✓ Print system info

▶ Show global variables
Running on darwin/arm64
✓ Show global variables
```

## Next Steps

Continue to [02-variables-and-facts](02-variables-and-facts.md) to learn about custom variables and all available system facts.
