# Commands

## mooncake plan

Generate and inspect a deterministic execution plan from your configuration.

### Usage

```bash
mooncake plan --config <file> [options]
```

### Flags

| Flag | Description |
|------|-------------|
| `--config, -c` | Path to configuration file (required) |
| `--vars, -v` | Path to variables file |
| `--tags, -t` | Filter steps by tags |
| `--format, -f` | Output format: text, json, yaml (default: text) |
| `--show-origins` | Display file:line:col origin for each step |
| `--output, -o` | Save plan to file |

### What is a Plan?

A **plan** is a fully expanded, deterministic representation of your configuration:

- **All loops expanded** - `with_items` and `with_filetree` expanded to individual steps
- **All includes resolved** - Nested includes flattened into a linear sequence
- **Origin tracking** - Every step tracks its source file:line:col and include chain
- **Deterministic** - Same config always produces identical plan
- **Tag filtering** - Steps not matching tags are marked as `skipped`

### Examples

```bash
# View plan as text
mooncake plan --config config.yml

# View plan with origins
mooncake plan --config config.yml --show-origins

# Export plan as JSON
mooncake plan --config config.yml --format json

# Save plan to file
mooncake plan --config config.yml --format json --output plan.json

# Filter by tags
mooncake plan --config config.yml --tags dev

# With variables
mooncake plan --config config.yml --vars prod.yml
```

### Use Cases

- **Inspect expansions** - See exactly how loops and includes expand
- **Debug configurations** - Understand step ordering and variable resolution
- **Verify determinism** - Ensure same config produces same plan
- **CI/CD integration** - Export plans for review before execution
- **Traceability** - Track every step back to source file location

### Plan Output Format

**Text format** (default):
```
[1] Install package (ID: step-0001)
    Action: shell
    Loop: with_items[0] (first=true, last=false)

[2] Install package (ID: step-0002)
    Action: shell
    Loop: with_items[1] (first=false, last=false)
```

**With `--show-origins`:**
```
[1] Install package (ID: step-0001)
    Action: shell
    Origin: /path/to/config.yml:15:3
    Chain: main.yml:10 -> tasks/setup.yml:15

[2] Install package (ID: step-0002)
    Action: shell
    Origin: /path/to/config.yml:15:3
```

**JSON format** includes full step details:
```json
{
  "version": "1.0",
  "generated_at": "2026-02-04T10:30:00Z",
  "root_file": "/path/to/config.yml",
  "steps": [
    {
      "id": "step-0001",
      "name": "Install package",
      "origin": {
        "file": "/path/to/config.yml",
        "line": 15,
        "column": 3,
        "include_chain": ["main.yml:10", "tasks/setup.yml:15"]
      },
      "loop_context": {
        "type": "with_items",
        "item": "neovim",
        "index": 0,
        "first": true,
        "last": false
      },
      "action": {
        "type": "shell",
        "data": {
          "command": "brew install neovim"
        }
      }
    }
  ]
}
```

## mooncake run

Run a configuration file.

### Usage

```bash
mooncake run --config <file> [options]
```

### Flags

| Flag | Description |
|------|-------------|
| `--config, -c` | Path to configuration file (required, unless using --from-plan) |
| `--from-plan` | Execute from a saved plan file (JSON/YAML) |
| `--vars, -v` | Path to variables file |
| `--tags, -t` | Filter steps by tags |
| `--dry-run` | Preview without executing |
| `--sudo-pass, -s` | Sudo password |
| `--raw, -r` | Disable animated TUI |
| `--log-level, -l` | Log level (debug, info, error) |

### Examples

```bash
# Basic execution
mooncake run --config config.yml

# Preview changes
mooncake run --config config.yml --dry-run

# Filter by tags
mooncake run --config config.yml --tags dev

# With sudo
mooncake run --config config.yml --sudo-pass mypass

# Execute from saved plan
mooncake plan --config config.yml --format json --output plan.json
mooncake run --from-plan plan.json
```

## mooncake explain

Display system information.

### Usage

```bash
mooncake explain
# or
mooncake info
```

Shows:
- OS, distribution, architecture
- CPU cores, memory
- GPUs
- Storage devices
- Network interfaces
- Package manager, Python version

Use this to see what system facts are available as variables.
