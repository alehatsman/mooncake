# Core Concepts

Mooncake configurations are YAML files containing an array of **steps**. Each step performs one **action**.

## Steps

Steps are executed sequentially:

```yaml
- name: First step
  shell: echo "hello"

- name: Second step
  file:
    path: /tmp/test
    state: directory
```

## Actions

Available actions:
- **shell** - Execute shell commands
- **file** - Create files and directories
- **template** - Render configuration templates
- **include** - Load other configuration files
- **vars** - Define variables

## Variables

Use `{{variable}}` syntax for dynamic values:

```yaml
- vars:
    app_name: MyApp

- shell: echo "Installing {{app_name}}"
```

## System Facts

Automatically available variables:
- `os` - Operating system (linux, darwin, windows)
- `arch` - Architecture (amd64, arm64)
- `hostname` - System hostname
- `distribution` - Linux/macOS distribution

See all facts: `mooncake explain`

## Next

Continue to [Commands](commands.md) to learn about CLI usage.
