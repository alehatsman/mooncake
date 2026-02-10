# Core Concepts

Mooncake configurations are YAML files containing an array of **steps**. Each step performs one **action**.

## Two-Phase Architecture

Mooncake uses a two-phase architecture for configuration execution:

1. **Planning Phase** - Expands configuration into a deterministic plan
   - Resolves all includes recursively
   - Expands all loops (`with_items`, `with_filetree`) into individual steps
   - Tracks origin (file:line:col) for every step
   - Filters steps by tags (marked as skipped)
   - Produces a deterministic, inspectable plan

2. **Execution Phase** - Executes the plan
   - Evaluates `when` conditions at runtime
   - Executes actions (shell, file, template, etc.)
   - Captures results and updates variables
   - Logs progress and status

**Benefits:**

- **Deterministic** - Same config always produces the same plan
- **Inspectable** - Use `mooncake plan` to see what will execute
- **Traceable** - Every step tracks its origin with include chain
- **Debuggable** - Understand loop expansions and includes before execution

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

- **shell** / **command** - Execute shell commands or direct commands
- **file** - Create files, directories, links, and manage permissions
- **copy** - Copy files with checksum verification
- **download** - Download files from URLs with checksums and retry
- **unarchive** - Extract tar.gz, zip archives with security protections
- **template** - Render configuration templates
- **package** - Install, remove, and update system packages
- **service** - Manage system services (systemd on Linux, launchd on macOS)
- **assert** - Verify state (command results, file properties, HTTP responses)
- **preset** - Invoke reusable, parameterized workflows (e.g., ollama preset)
- **print** - Display messages to the user
- **include** - Load other configuration files
- **include_vars** - Load variables from files
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

See all facts: `mooncake facts`

## Next

Continue to [Commands](commands.md) to learn about CLI usage.
