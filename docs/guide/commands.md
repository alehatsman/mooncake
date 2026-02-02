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
| **Privilege Escalation** ||
| `--ask-become-pass, -K` | Prompt for sudo password interactively (recommended) |
| `--sudo-pass-file` | Read sudo password from file (must have 0600 permissions) |
| `--sudo-pass, -s` | Sudo password (requires --insecure-sudo-pass) |
| `--insecure-sudo-pass` | Allow --sudo-pass flag (password visible in history) |
| **Display Options** ||
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

# With sudo (interactive prompt - recommended)
mooncake run --config config.yml --ask-become-pass
# or
mooncake run --config config.yml -K

# With sudo (file-based)
echo "mypassword" > ~/.mooncake/sudo_pass
chmod 0600 ~/.mooncake/sudo_pass
mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass

# With sudo (insecure CLI - not recommended)
mooncake run --config config.yml --sudo-pass mypass --insecure-sudo-pass

# Execute from saved plan
mooncake plan --config config.yml --format json --output plan.json
mooncake run --from-plan plan.json
```

## mooncake facts

Display system facts that are available as template variables.

### Usage

```bash
mooncake facts [--format text|json]
```

### Flags

| Flag | Description |
|------|-------------|
| `--format, -f` | Output format: text or json (default: text) |

### What Facts Are Shown?

System information collected and available as template variables:

**System:**
- OS, distribution, kernel version, architecture, hostname

**Hardware:**
- CPU model, cores, flags (AVX, SSE, etc.)
- Memory total/free, swap
- GPUs (vendor, model, memory, driver, CUDA version)
- Disks (device, mount point, size, usage)

**Network:**
- Network interfaces (name, MAC, MTU, addresses)
- Default gateway
- DNS servers
- IP addresses

**Software:**
- Package manager (apt, brew, etc.)
- Python version
- Docker, Git, Go versions

### Examples

**Text Output (Human-Readable)**

```bash
mooncake facts
```

Example output:
```
╭─────────────────────────────────────────────────────────────╮
│                    System Information                       │
╰─────────────────────────────────────────────────────────────╯

OS:         ubuntu 22.04
Arch:       amd64
Hostname:   server01
Kernel:     6.5.0-14-generic

CPU:
  Cores:    8
  Model:    Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz
  Flags:    avx avx2 sse4_2 fma aes

Memory:
  Total:    16384 MB (16.0 GB)
  Free:     8192 MB (8.0 GB)
  Swap:     4096 MB total, 2048 MB free

Software:
  Package Manager: apt
  Python:          3.11.5
  Docker:          24.0.7
  Git:             2.43.0
  Go:              1.21.5

GPUs:
  • NVIDIA GeForce RTX 4090, Memory: 24GB, Driver: 535.54.03, CUDA: 12.3

Storage:
  Device        Mount     Type      Size        Used       Avail
  ────────────────────────────────────────────────────────────
  /dev/sda1     /         ext4      500 GB      250 GB     250 GB
  /dev/sdb1     /data     ext4      1000 GB     500 GB     500 GB

Network:
  Gateway:  192.168.1.1
  DNS:      8.8.8.8, 1.1.1.1

Network Interfaces:
  • eth0  |  MAC: 00:11:22:33:44:55  |  192.168.1.100/24
```

**JSON Output (Machine-Readable)**

```bash
mooncake facts --format json
```

Example output:
```json
{
  "OS": "linux",
  "Arch": "amd64",
  "Hostname": "server01",
  "Username": "admin",
  "UserHome": "/home/admin",
  "Distribution": "ubuntu",
  "DistributionVersion": "22.04",
  "DistributionMajor": "22",
  "KernelVersion": "6.5.0-14-generic",
  "CPUCores": 8,
  "CPUModel": "Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz",
  "CPUFlags": ["fpu", "vme", "avx", "avx2", "sse4_2", "fma"],
  "MemoryTotalMB": 16384,
  "MemoryFreeMB": 8192,
  "SwapTotalMB": 4096,
  "SwapFreeMB": 2048,
  "DefaultGateway": "192.168.1.1",
  "DNSServers": ["8.8.8.8", "1.1.1.1"],
  "IPAddresses": ["192.168.1.100"],
  "NetworkInterfaces": [
    {
      "Name": "eth0",
      "MACAddress": "00:11:22:33:44:55",
      "MTU": 1500,
      "Addresses": ["192.168.1.100/24"],
      "Up": true
    }
  ],
  "Disks": [
    {
      "Device": "/dev/sda1",
      "MountPoint": "/",
      "Filesystem": "ext4",
      "SizeGB": 500,
      "UsedGB": 250,
      "AvailGB": 250,
      "UsedPct": 50
    }
  ],
  "GPUs": [
    {
      "Vendor": "nvidia",
      "Model": "GeForce RTX 4090",
      "Memory": "24GB",
      "Driver": "535.54.03",
      "CUDAVersion": "12.3"
    }
  ],
  "PythonVersion": "3.11.5",
  "PackageManager": "apt",
  "DockerVersion": "24.0.7",
  "GitVersion": "2.43.0",
  "GoVersion": "1.21.5"
}
```

### Using Facts in Templates

All facts are available as variables in your configuration templates:

```yaml
steps:
  - name: Show system info
    shell: |
      echo "Running on {{ os }}/{{ arch }}"
      echo "CPU: {{ cpu_model }}"
      echo "Memory: {{ memory_total_mb }}MB"
      echo "Kernel: {{ kernel_version }}"

  - name: Iterate over disks
    shell: |
      {% for disk in disks %}
      echo "Disk: {{ disk.Device }} at {{ disk.MountPoint }} ({{ disk.SizeGB }}GB)"
      {% endfor %}

  - name: Check Docker availability
    shell: echo "Docker {{ docker_version }} is installed"
    when: docker_version != ""
```

See [Variables](config/variables.md) for complete list of available facts.
