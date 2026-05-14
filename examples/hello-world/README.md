# 01 - Hello World

**Start here!** This is the simplest possible Mooncake configuration.

## What You'll Learn

- Running basic shell commands
- Using global system variables
- Multi-line shell commands

## Quick Start

```bash
mooncake apply --config config.yml           # run it
mooncake apply --config config.yml --dry-run # preview first
```

> In your own projects, name the file `mooncake.yml` and you can drop `-c`
> entirely — `mooncake apply` auto-discovers it.

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

→ See [`../README.md`](../README.md) for the curated learning path
  through all examples.
→ Next stop: [`../variables-and-facts/`](../variables-and-facts/) for
  custom variables and the full system-facts reference.
→ Run `mooncake init` in a new directory to scaffold your own project.
