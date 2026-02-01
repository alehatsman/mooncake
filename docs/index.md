# Mooncake

Space fighters provisioning tool, **Chookity!**

[![CI](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/ci.yml)
[![Security](https://github.com/alehatsman/mooncake/actions/workflows/security.yml/badge.svg?branch=master)](https://github.com/alehatsman/mooncake/actions/workflows/security.yml)
[![codecov](https://codecov.io/gh/alehatsman/mooncake/branch/master/graph/badge.svg)](https://codecov.io/gh/alehatsman/mooncake)

Mooncake is a simple, powerful provisioning tool for system configuration. Write YAML configs to manage dotfiles, configure systems, and automate your development environment setup.

<div class="grid cards" markdown>

-   :rocket:{ .lg .middle } **Fast & Lightweight**

    ---

    Single Go binary with no dependencies - install and run in seconds

-   :material-shield-check:{ .lg .middle } **Safe by Default**

    ---

    Dry-run mode previews all changes before applying them

-   :material-devices:{ .lg .middle } **Cross-Platform**

    ---

    Works seamlessly on Linux, macOS, and Windows

-   :material-file-code:{ .lg .middle } **Simple YAML**

    ---

    No complex DSL to learn - just write YAML configurations

-   :material-puzzle:{ .lg .middle } **Powerful Features**

    ---

    Variables, conditionals, loops, templates, and system facts

-   :material-draw:{ .lg .middle } **Beautiful TUI**

    ---

    Animated progress tracking with real-time status updates

</div>

---

## What is Mooncake?

Mooncake is a **lightweight configuration management tool** designed for:

- **Personal Use:** Manage dotfiles and development environments
- **System Setup:** Automate new machine configuration
- **Cross-Platform:** Write once, run on Linux, macOS, or Windows
- **Simplicity:** YAML configs without complex abstractions

**Perfect for:** Developers managing personal configs, dotfiles enthusiasts, and anyone wanting simple system automation without the complexity of enterprise tools.

---

## Installation

```bash
go install github.com/alehatsman/mooncake@latest
```

Verify installation:
```bash
mooncake --help
```

---

## Quick Start

Get started in 30 seconds:

```bash
# Create a simple configuration
cat > config.yml <<EOF
- name: Hello Mooncake
  shell: echo "Chookity! Running on {{os}}/{{arch}}"

- name: Create a file
  file:
    path: /tmp/mooncake-test.txt
    state: file
    content: "Hello from Mooncake!"
EOF

# Run it
mooncake run --config config.yml

# Preview without executing
mooncake run --config config.yml --dry-run
```

**Next:** Explore [Examples](examples/index.md) for real-world configurations or continue reading below.

---

## Core Concepts

Mooncake configurations are YAML files containing an array of **steps**. Each step performs one **action**:

```yaml
- name: Step description
  shell: echo "This is a shell action"

- name: Another step
  file:
    path: /tmp/example
    state: directory
```

**Key concepts:**

- **Steps** - Sequential actions to execute
- **Actions** - What to do: `shell`, `file`, `template`, `include`, `vars`
- **Variables** - Dynamic values using `{{variable}}` syntax
- **Conditionals** - Execute steps based on conditions with `when`
- **System Facts** - Automatic OS, hardware, and network detection
- **Tags** - Filter execution by workflow with `--tags`

---

## Commands

### mooncake plan

Generate and inspect a deterministic execution plan.

**Syntax:**
```bash
mooncake plan --config <file> [options]
```

**What it does:**
- Expands all loops and includes into individual steps
- Shows exactly what will be executed before running
- Tracks origin (file:line:col) for every step
- Exports plans as JSON/YAML for CI/CD integration

**Common options:**

| Flag | Description |
|------|-------------|
| `--config, -c` | Path to configuration file (required) |
| `--format, -f` | Output format: text, json, yaml (default: text) |
| `--show-origins` | Display file:line:col for each step |
| `--output, -o` | Save plan to file |
| `--tags, -t` | Filter steps by tags |

**Examples:**

```bash
# View plan as text
mooncake plan --config config.yml

# View with origins
mooncake plan --config config.yml --show-origins

# Export as JSON
mooncake plan --config config.yml --format json --output plan.json

# Filter by tags
mooncake plan --config config.yml --tags dev
```

**Use cases:**
- 🔍 **Debugging** - See how loops and includes expand
- ✅ **Verification** - Review changes before execution
- 📊 **CI/CD** - Export plans for approval workflows
- 🔬 **Analysis** - Understand configuration behavior

### mooncake run

Run a configuration file.

**Syntax:**
```bash
mooncake run --config <file> [options]
```

**Common options:**

| Flag | Description |
|------|-------------|
| `--config, -c` | Path to configuration file (required, unless using --from-plan) |
| `--from-plan` | Execute from a saved plan file (JSON/YAML) |
| `--vars, -v` | Path to variables file |
| `--tags, -t` | Filter steps by tags (comma-separated) |
| `--dry-run` | Preview without executing |
| `--sudo-pass, -s` | Sudo password for `become: true` steps |
| `--raw, -r` | Disable animated TUI |
| `--log-level, -l` | Log level: debug, info, error (default: info) |

**Examples:**

```bash
# Basic execution
mooncake run --config config.yml

# With variables file
mooncake run --config config.yml --vars prod.yml

# Preview changes (safe!)
mooncake run --config config.yml --dry-run

# Filter by tags
mooncake run --config config.yml --tags dev,test

# With sudo for system operations
mooncake run --config config.yml --sudo-pass <password>

# Debug mode
mooncake run --config config.yml --log-level debug

# Execute from saved plan
mooncake plan --config config.yml --format json --output plan.json
mooncake run --from-plan plan.json
```

**Features:**

- 🎨 **Animated TUI** - Real-time progress with animated character (use `--raw` to disable)
- 🔍 **Dry-run mode** - Preview all changes before applying
- 🏷️ **Tag filtering** - Run specific workflows
- 🔐 **Sudo support** - Execute privileged operations
- ✅ **Validation** - YAML syntax and step structure checking

### mooncake explain

Display detailed system information.

**Syntax:**
```bash
mooncake explain
# or
mooncake info
```

**What it shows:**

- **System**: OS, distribution, version, architecture, hostname
- **Hardware**: CPU cores, memory, GPUs (vendor, model, memory, driver)
- **Storage**: Disks with mount points, filesystem types, size, used, available
- **Network**: Active interfaces with MAC addresses and IP addresses
- **Software**: Package manager, Python version

**Use cases:**

- See what system facts are available as variables
- Troubleshoot hardware detection
- Check system compatibility before running configurations

---

## Configuration Basics

### Shell Commands

Execute shell commands with variable templating.

```yaml
- name: Simple command
  shell: echo "Hello"

- name: Multi-line commands
  shell: |
    echo "Starting setup"
    mkdir -p ~/.local/bin
    echo "Setup complete"

- name: With variables
  shell: echo "Running on {{os}}/{{arch}}"
```

**Features:**

- Template variables in commands
- Multi-line scripts
- Exit code capture with `register`

[→ See example](examples/01-hello-world.md)

### File Operations

Create files and directories with permissions.

```yaml
# Create directory
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory
    mode: "0755"

# Create file with content
- name: Create config file
  file:
    path: ~/.config/myapp/config.txt
    state: file
    content: |
      app_name: myapp
      version: 1.0
    mode: "0644"

# Create executable script
- name: Create script
  file:
    path: ~/.local/bin/deploy.sh
    state: file
    content: "#!/bin/bash\necho 'Deploying...'"
    mode: "0755"
```

**States:**

- `directory` - Create directory
- `file` - Create file (optionally with content)

**Permissions:**

- `"0755"` - rwxr-xr-x (directories, executables)
- `"0644"` - rw-r--r-- (regular files)
- `"0600"` - rw------- (private files)

[→ See example](examples/03-files-and-directories.md)

### Template Rendering

Render configuration files from templates using pongo2 syntax.

```yaml
- name: Render nginx config
  template:
    src: ./templates/nginx.conf.j2
    dest: /etc/nginx/sites-available/myapp
    mode: "0644"
    vars:
      server_name: example.com
      port: 8080
```

**Template syntax (pongo2):**

```jinja
# Variables
server_name {{ server_name }};
listen {{ port }};

# Conditionals
{% if enable_ssl %}
ssl_certificate {{ ssl_cert }};
{% endif %}

# Loops
{% for upstream in upstreams %}
upstream {{ upstream.name }} {
    server {{ upstream.host }}:{{ upstream.port }};
}
{% endfor %}

# Filters
{{ path | expanduser }}  # Expands ~ to home directory
{{ text | upper }}       # Uppercase
```

[→ See example](examples/05-templates.md)

### Include Files

Load and execute steps from other configuration files.

```yaml
- name: Include common setup
  include: ./tasks/common.yml

- name: Load OS-specific config
  include: ./tasks/{{os}}.yml
```

**Path resolution:**

- Paths are relative to the **current file**, not working directory
- Supports variables in paths
- Can be nested (includes can include other files)

**Include variables:**

```yaml
- name: Load environment variables
  include_vars: ./vars/{{environment}}.yml
```

[→ See example](examples/10-multi-file-configs.md)

---

## Conditionals

Execute steps based on conditions using `when`.

```yaml
# OS-specific steps
- name: Install on Linux
  shell: apt install neovim
  become: true
  when: os == "linux"

- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

# Complex conditions
- name: High memory systems
  shell: echo "Configuring for high memory"
  when: memory_total_mb >= 16000

- name: Ubuntu 20+
  shell: apt update
  become: true
  when: distribution == "ubuntu" and distribution_major >= "20"

# ARM Macs
- name: ARM Mac specific
  shell: echo "ARM architecture detected"
  when: os == "darwin" && arch == "arm64"
```

**Operators:**

- Comparison: `==`, `!=`, `>`, `<`, `>=`, `<=`
- Logical: `&&` (and), `||` (or), `!` (not)
- String: `in`, `contains`
- Functions: `len()`, see [expr-lang](https://github.com/expr-lang/expr)

[→ See example](examples/04-conditionals.md)

---

## Tags

Filter execution by tags for workflow management.

```yaml
- name: Install dev tools
  shell: brew install neovim ripgrep
  tags:
    - dev
    - tools

- name: Production deployment
  shell: ./deploy-prod.sh
  tags:
    - prod
    - deploy

- name: Run tests
  shell: npm test
  tags:
    - test
    - dev
```

**Usage:**

```bash
# Run only dev-tagged steps
mooncake run --config config.yml --tags dev

# Run dev OR prod steps
mooncake run --config config.yml --tags dev,prod

# No tags = run all steps
mooncake run --config config.yml
```

**Behavior:**

- **No filter**: All steps run (including untagged)
- **With filter**: Only matching tags run; untagged steps skipped
- **Multiple tags**: Step runs if it has ANY specified tag (OR logic)

[→ See example](examples/08-tags.md)

---

## Loops

Iterate over lists or files to avoid repetition.

**List iteration (with_items):**

```yaml
- vars:
    packages:
      - neovim
      - ripgrep
      - fzf
      - tmux

- name: Install package
  shell: brew install {{ item }}
  with_items: "{{ packages }}"
```

**File tree iteration (with_filetree):**

```yaml
- name: Deploy dotfiles
  shell: cp "{{ item.src }}" "~/{{ item.name }}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**Available in loops:**

- **with_items**: `{{ item }}` - Current list item
- **with_filetree**:
  - `{{ item.src }}` - Source file path
  - `{{ item.name }}` - File name
  - `{{ item.is_dir }}` - Boolean, true if directory

[→ See example](examples/06-loops.md)

---

## Variables

### Custom Variables

Define and use your own variables.

```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    config_dir: ~/.config/{{app_name}}

- name: Create app directory
  file:
    path: "{{config_dir}}"
    state: directory

- name: Display info
  shell: echo "Installing {{app_name}} v{{version}}"
```

**Variable syntax:**

- Define: `vars:` at step level
- Use: `{{variable_name}}` anywhere
- Nest: `{{outer}}/{{inner}}`

[→ See example](examples/02-variables-and-facts.md)

### System Facts

Mooncake automatically collects system information available as variables.

| Category | Variable | Description | Example |
|----------|----------|-------------|---------|
| **Basic** | `os` | Operating system | `linux`, `darwin`, `windows` |
| | `arch` | Architecture | `amd64`, `arm64` |
| | `hostname` | System hostname | `my-laptop` |
| | `user_home` | User home directory | `/home/user` |
| **Hardware** | `cpu_cores` | Number of CPU cores | `8` |
| | `memory_total_mb` | Total RAM in MB | `16384` |
| **Distribution** | `distribution` | Distribution name | `ubuntu`, `debian`, `macos` |
| | `distribution_version` | Full version | `22.04`, `15.7.3` |
| | `distribution_major` | Major version | `22`, `15` |
| **Software** | `package_manager` | Package manager | `apt`, `yum`, `brew`, `pacman` |
| | `python_version` | Python version | `3.11.4` |
| **Network** | `ip_addresses` | Array of IPs | `["192.168.1.10"]` |
| | `ip_addresses_string` | Comma-separated IPs | `"192.168.1.10, 10.0.0.5"` |

**Check your system:**
```bash
mooncake explain
```

**Example usage:**

```yaml
- name: Install with detected package manager
  shell: "{{package_manager}} install neovim"
  become: true
  when: os == "linux"

- name: High memory config
  shell: echo "Using high memory settings"
  when: memory_total_mb >= 16000
```

[→ See example](examples/02-variables-and-facts.md)

### Register (Capture Output)

Capture command output and use it in subsequent steps.

```yaml
# Capture command output
- name: Check if git exists
  shell: which git
  register: git_check

# Use in conditional
- name: Show git location
  shell: echo "Git is at {{git_check.stdout}}"
  when: git_check.rc == 0

# Capture and use in paths
- name: Get username
  shell: whoami
  register: current_user

- name: Create user-specific file
  file:
    path: "/tmp/{{current_user.stdout}}_config.txt"
    state: file
    content: "Config for {{current_user.stdout}}"
```

**Available fields:**

- `{name}.stdout` - Standard output
- `{name}.stderr` - Standard error
- `{name}.rc` - Return code (0 = success)
- `{name}.failed` - Boolean, true if failed
- `{name}.changed` - Boolean, true if changed

**Works with:**

- `shell` - Captures output
- `file` - Detects if created/modified
- `template` - Detects if rendered output changed

[→ See example](examples/07-register.md)

---

## Advanced Features

### Sudo / Privilege Escalation

Execute commands with elevated privileges.

```yaml
- name: Install system package
  shell: apt install neovim
  become: true

- name: Create system directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"
  become: true
```

**Provide sudo password:**
```bash
mooncake run --config config.yml --sudo-pass <password>
```

**Note:** Only steps with `become: true` use sudo.

[→ See example](examples/09-sudo.md)

### Path Resolution

Mooncake uses Node.js-style relative path resolution.

**Key principle:** Paths are relative to the **current file**, not the working directory.

```
project/
├── main.yml
└── configs/
    ├── app.yml
    └── templates/
        └── config.j2
```

**main.yml:**
```yaml
- include: ./configs/app.yml  # Relative to main.yml
```

**configs/app.yml:**
```yaml
- template:
    src: ./templates/config.j2  # Relative to app.yml
    dest: ~/app/config
```

**Path types:**

- Relative: `./file.yml`, `../templates/app.j2`
- Absolute: `/etc/config`, `/opt/app`
- Home: `~/.config`, `~/bin/script`

[→ See example](examples/10-multi-file-configs.md)

### Dry-Run Mode

Preview changes without executing them.

```bash
mooncake run --config config.yml --dry-run
```

**What it does:**

- ✅ Validates YAML syntax and structure
- ✅ Checks required files exist
- ✅ Verifies paths and variables resolve
- ✅ Shows what would be executed
- ✅ Processes includes recursively
- ❌ Does NOT make any changes

**Example output:**
```
▶ Create application directory
  [DRY-RUN] Would create directory: /tmp/myapp (mode: 0755)
✓ Create application directory
▶ Render neovim configuration
  [DRY-RUN] Would template: ./init.lua.j2 -> /tmp/myapp/config/init.lua (mode: 0644)
✓ Render neovim configuration
▶ Install neovim on Linux
  [DRY-RUN] Would execute: apt install neovim
  [DRY-RUN] With sudo privileges
✓ Install neovim on Linux
```

**Use cases:**
- Test configurations before applying
- Preview changes in production
- Debug complex conditionals and includes
- Verify variable substitution

---

## Best Practices

### 1. Always Use Dry-Run First
```bash
mooncake run --config config.yml --dry-run
```
Preview changes before applying, especially in production.

### 2. Organize by Purpose
```
project/
├── main.yml           # Entry point
├── tasks/
│   ├── common.yml     # Shared setup
│   ├── dev.yml        # Development
│   └── prod.yml       # Production
└── vars/
    ├── dev.yml
    └── prod.yml
```

### 3. Use Variables for Reusability
```yaml
- vars:
    app_name: myapp
    version: "1.0.0"

- name: Create versioned directory
  file:
    path: "/opt/{{app_name}}-{{version}}"
    state: directory
```

### 4. Tag Your Workflows
```yaml
- name: Install dev tools
  shell: brew install neovim
  tags: [dev, tools]

- name: Deploy to production
  shell: ./deploy.sh
  tags: [prod, deploy]
```

Run selectively:
```bash
mooncake run --config config.yml --tags dev
```

### 5. Document Conditions
```yaml
# Install on Ubuntu 20+ only (older versions have incompatible package)
- name: Install modern package
  shell: apt install package-name
  become: true
  when: distribution == "ubuntu" and distribution_major >= "20"
```

### 6. Use System Facts
```yaml
# Automatic OS detection
- shell: "{{package_manager}} install neovim"
  become: true
  when: os == "linux"
```

### 7. Test Incrementally
Build configurations step by step:
1. Start with simple steps
2. Test with `--dry-run`
3. Add complexity gradually
4. Use `--log-level debug` to troubleshoot

### 8. Handle Errors with Register
```yaml
- name: Check if command exists
  shell: which docker
  register: docker_check

- name: Install if missing
  shell: curl -fsSL https://get.docker.com | sh
  when: docker_check.rc != 0
```

---

## Example Configurations

**Complete Neovim Setup:**

```yaml
- vars:
    config_dir: ~/.config
    nvim_dir: "{{config_dir}}/nvim"

- name: Create neovim directory
  file:
    path: "{{nvim_dir}}"
    state: directory
    mode: "0755"

- name: Render neovim config
  template:
    src: ./init.lua.j2
    dest: "{{nvim_dir}}/init.lua"
    mode: "0644"

- name: Install neovim on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install neovim on Linux
  shell: apt install neovim
  become: true
  when: os == "linux"

- name: Copy plugin configs
  template:
    src: "{{item.src}}"
    dest: "{{nvim_dir}}/lua/{{item.name}}"
  with_filetree: ./nvim/lua
  when: item.is_dir == false
```

**Multi-OS Package Installation:**

```yaml
- vars:
    packages:
      - neovim
      - ripgrep
      - fzf
      - tmux

# macOS
- name: Install package (macOS)
  shell: brew install {{item}}
  with_items: "{{packages}}"
  when: os == "darwin"

# Ubuntu/Debian
- name: Install package (Ubuntu)
  shell: apt install -y {{item}}
  become: true
  with_items: "{{packages}}"
  when: os == "linux" and package_manager == "apt"

# Arch Linux
- name: Install package (Arch)
  shell: pacman -S --noconfirm {{item}}
  become: true
  with_items: "{{packages}}"
  when: os == "linux" and package_manager == "pacman"
```

**Dotfiles Deployment with Backup:**

```yaml
- vars:
    backup_dir: ~/.dotfiles-backup

- name: Create backup directory
  file:
    path: "{{backup_dir}}"
    state: directory

- name: Backup existing dotfiles
  shell: |
    for file in .bashrc .vimrc .gitconfig; do
      [ -f ~/$file ] && cp ~/$file {{backup_dir}}/$file.$(date +%Y%m%d)
    done

- name: Deploy dotfiles
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

**See [Examples](examples/index.md) for complete working examples.**

---

## Next Steps

- 📚 **[Examples](examples/index.md)** - Step-by-step learning path from beginner to advanced
- 📖 **[Reference](guide/config/actions.md)** - Detailed configuration documentation
- 🤝 **[Contributing](development/contributing.md)** - Help make Mooncake better
- 📝 **[Changelog](about/changelog.md)** - See what's new

---

## Community

- [:fontawesome-brands-github: GitHub Issues](https://github.com/alehatsman/mooncake/issues) - Report bugs and request features
- [:material-star: Star the project](https://github.com/alehatsman/mooncake) if you find it useful!

## License

MIT License - Copyright (c) 2024-2026 Aleh Atsman
