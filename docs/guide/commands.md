# Commands

## mooncake run

Run a configuration file.

### Usage

```bash
mooncake run --config <file> [options]
```

### Flags

| Flag | Description |
|------|-------------|
| `--config, -c` | Path to configuration file (required) |
| `--vars, -v` | Path to variables file |
| `--tags, -t` | Filter steps by tags |
| `--dry-run` | Preview without executing |
| `--sudo-pass, -s` | Sudo password |
| `--raw, -r` | Disable animated TUI |
| `--log-level, -l` | Log level (debug, info, error) |

### Examples

```bash
# Basic
mooncake run --config config.yml

# With dry-run
mooncake run --config config.yml --dry-run

# Filter by tags
mooncake run --config config.yml --tags dev

# With sudo
mooncake run --config config.yml --sudo-pass mypass
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
