# Mooncake

**The Standard Runtime for AI System Configuration**

Mooncake is to AI agents what Docker is to containers - a safe, validated execution layer for system configuration. **Chookity!**

<div class="grid cards" markdown>

-   :material-shield-check:{ .lg .middle } **Safe by Default**

    ---

    Dry-run validation, idempotency guarantees, rollback support for AI-driven configuration

-   :material-chart-line:{ .lg .middle } **Full Observability**

    ---

    Structured events, audit trails, execution logs for AI agent compliance

-   :material-check-circle:{ .lg .middle } **Validated Operations**

    ---

    Schema validation, type checking, state verification before execution

-   :material-robot:{ .lg .middle } **AI-Friendly Format**

    ---

    Simple YAML that any AI can generate and understand - no complex DSL

-   :rocket:{ .lg .middle } **Zero Dependencies**

    ---

    Single Go binary with no Python, no modules, no setup required

-   :material-devices:{ .lg .middle } **Cross-Platform**

    ---

    Unified interface for Linux, macOS, and Windows

</div>

---

## What is Mooncake?

Mooncake provides a safe, validated execution environment for AI agents to configure systems. Built for the AI-driven infrastructure era.

**Target Audiences:**

- **🤖 AI Agent Developers** - Build agents that configure systems safely with validated execution, observability, and compliance
- **🏗️ Platform Engineers** - Manage AI-driven infrastructure with audit trails and safety guardrails
- **👨‍💻 Developers with AI Assistants** - Let AI manage your dotfiles and dev setup with built-in safety and undo
- **⚙️ DevOps Teams** - Simpler alternative to Ansible for personal/team configs with AI workflow integration

**Why AI Agents Choose Mooncake:**

- Industry-standard YAML format that any AI can target
- Guarantees idempotency and reproducibility
- Enables system configuration without risk
- Provides observability and compliance out of the box

---

## Installation

```bash
go install github.com/alehatsman/mooncake@latest
```

Verify installation:
```bash
mooncake --help
```

→ **[Detailed Installation Guide](getting-started/installation.md)** for other platforms and methods

---

## 30 Second Quick Start

```bash
# Create config.yml
cat > config.yml <<'EOF'
- name: Hello Mooncake
  shell: echo "Chookity! Running on {{os}}/{{arch}}"

- name: Create a file
  file:
    path: /tmp/mooncake-test.txt
    state: file
    content: "Hello from Mooncake!"
EOF

# Preview changes (safe!)
mooncake run --config config.yml --dry-run

# Run it for real
mooncake run --config config.yml
```

**What just happened?**

1. Mooncake detected your OS automatically (`{{os}}`, `{{arch}}`)
2. Ran a shell command using those variables
3. Created a file with specific content

Check the result:
```bash
cat /tmp/mooncake-test.txt
# Output: Hello from Mooncake!
```

→ **[Try More Examples](examples/)** - Step-by-step learning path from beginner to advanced

---

## What You Can Do

Quick reference of available actions with examples:

### 🐚 Run Commands

Execute shell commands with variables and conditionals.

```yaml
- name: OS-specific package install
  shell: "{{package_manager}} install neovim"
  become: true
  when: os == "linux"
```

**Features**: Multi-line scripts, timeouts, retries, environment variables, working directory

[Learn more: Shell Action →](guide/config/actions.md#shell)

---

### 📁 Manage Files & Directories

Create files, directories, links with permissions and ownership.

```yaml
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory
    mode: "0755"

- name: Create config file
  file:
    path: ~/.config/myapp/settings.yml
    state: file
    content: |
      app_name: myapp
      version: 1.0
    mode: "0644"
```

**Features**: File/directory creation, symlinks, hard links, permissions, ownership, removal

[Learn more: File Action →](guide/config/actions.md#file)

---

### 📝 Render Templates

Render configuration files from templates with variables and logic.

```yaml
- name: Render nginx config
  template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 8080
      ssl_enabled: true
```

**Template syntax**: Variables `{{ var }}`, conditionals `{% if %}`, loops `{% for %}`, filters `{{ path | expanduser }}`

[Learn more: Template Action →](guide/config/actions.md#template)

---

### 📦 Copy Files

Copy files with checksum verification and backup support.

```yaml
- name: Deploy application config
  copy:
    src: ./configs/app.yml
    dest: /etc/app/config.yml
    mode: "0644"
    owner: app
    group: app
    backup: true
```

**Features**: Checksum verification, automatic backups, ownership management

[Learn more: Copy Action →](guide/config/actions.md#copy)

---

### ⬇️ Download Files

Download files from URLs with checksums and retry logic.

```yaml
- name: Download Go tarball
  download:
    url: "https://go.dev/dl/go1.21.5.linux-amd64.tar.gz"
    dest: "/tmp/go.tar.gz"
    checksum: "e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e"
    timeout: "10m"
    retries: 3
```

**Features**: Checksum verification, retry logic, custom headers, idempotent downloads

[Learn more: Download Action →](guide/config/actions.md#download)

---

### 📂 Extract Archives

Extract tar, tar.gz, and zip archives with security protections.

```yaml
- name: Extract Node.js
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    strip_components: 1
    creates: /opt/node/bin/node
```

**Features**: Automatic format detection, path stripping, security validation, idempotency

[Learn more: Unarchive Action →](guide/config/actions.md#unarchive)

---

### ⚙️ Manage Services

Manage system services (systemd on Linux, launchd on macOS).

```yaml
- name: Configure and start nginx
  service:
    name: nginx
    state: started
    enabled: true
  become: true
```

**Features**: Start/stop/restart services, enable on boot, create unit files, drop-in configs

[Learn more: Service Action →](guide/config/actions.md#service)

---

### ✓ Verify State

Assert command results, file properties, and HTTP responses.

```yaml
- name: Verify Docker is installed
  assert:
    command:
      cmd: docker --version
      exit_code: 0

- name: Verify API is healthy
  assert:
    http:
      url: https://api.example.com/health
      status: 200
```

**Features**: Command assertions, file property checks, HTTP response validation, fail-fast behavior

[Learn more: Assert Action →](guide/config/actions.md#assert)

---

### 🎯 Reusable Workflows

Use presets for complex, parameterized workflows.

```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
    service: true
    pull:
      - llama3.1:8b
      - mistral:latest
  become: true
```

**Features**: Parameter validation, type safety, idempotency, platform detection

[Learn more: Presets →](guide/presets.md)

---

**→ [See All Actions in Reference](guide/config/actions.md)** - Complete action documentation with examples

---

## Control Your Execution

### Variables & System Facts

Define custom variables and use auto-detected system information.

```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"

- name: Install application
  shell: echo "Installing {{app_name}} v{{version}} on {{os}}"
```

**Auto-detected facts**: `os`, `arch`, `cpu_cores`, `memory_total_mb`, `distribution`, `package_manager`, `hostname`, and more

```bash
mooncake facts  # See all available system facts
```

[Learn more: Variables Guide →](guide/config/variables.md)

---

### Conditionals

Execute steps based on conditions.

```yaml
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Linux
  shell: apt install neovim
  become: true
  when: os == "linux" && package_manager == "apt"
```

**Operators**: `==`, `!=`, `>`, `<`, `>=`, `<=`, `&&`, `||`, `!`, `in`

[Learn more: Control Flow →](guide/config/control-flow.md)

---

### Loops

Iterate over lists or files to avoid repetition.

```yaml
# Iterate over lists
- vars:
    packages: [neovim, ripgrep, fzf, tmux]

- name: Install package
  shell: brew install {{item}}
  with_items: "{{packages}}"

# Iterate over files
- name: Deploy dotfile
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

[Learn more: Loops →](guide/config/control-flow.md#loops)

---

### Tags

Filter execution by workflow.

```yaml
- name: Development setup
  shell: install-dev-tools.sh
  tags: [dev, setup]

- name: Production deployment
  shell: deploy-prod.sh
  tags: [prod, deploy]
```

**Usage**:
```bash
# Run only dev-tagged steps
mooncake run --config config.yml --tags dev

# Multiple tags (OR logic)
mooncake run --config config.yml --tags dev,test
```

[Learn more: Tags →](guide/config/control-flow.md#tags)

---

## Key Features

### 🔍 Dry-Run Mode

Preview all changes before applying with `--dry-run`:

```bash
mooncake run --config config.yml --dry-run
```

**What it shows**: Validates syntax, checks paths, shows what would execute - without making any changes.

---

### 📊 System Facts Collection

Mooncake automatically detects system information:

- **OS**: `os`, `arch`, `distribution`, `distribution_version`, `kernel_version`
- **Hardware**: `cpu_cores`, `cpu_model`, `memory_total_mb`, `memory_free_mb`
- **Network**: `ip_addresses`, `default_gateway`, `dns_servers`, `network_interfaces`
- **Software**: `package_manager`, `python_version`, `docker_version`, `git_version`
- **Storage**: `disks` (mounts, filesystem, size, usage)
- **GPU**: `gpus` (vendor, model, memory, driver, CUDA version)

```bash
mooncake facts              # Text output
mooncake facts --format json  # JSON output
```

[See all facts →](guide/config/reference.md#system-facts-reference)

---

### 📋 Execution Planning

Generate deterministic execution plans before running:

```bash
# View plan as text
mooncake plan --config config.yml

# Export as JSON for CI/CD
mooncake plan --config config.yml --format json --output plan.json

# Execute from saved plan
mooncake run --from-plan plan.json
```

**Use cases**: Debugging, verification, CI/CD integration, configuration analysis

[Learn more: Commands →](guide/commands.md)

---

### ⚡ Robust Execution

Control command execution with timeouts, retries, and custom conditions:

```yaml
- name: Download with retry
  shell: curl -O https://example.com/file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
  failed_when: "result.rc != 0 and result.rc != 18"  # 18 = partial transfer
```

[Learn more: Execution Control →](examples/11-execution-control.md)

---

### 🔐 Sudo Support

Execute privileged operations securely:

```yaml
- name: Install system package
  shell: apt update && apt install neovim
  become: true
```

**Password methods**:
- Interactive: `mooncake run --config config.yml --ask-become-pass` (or `-K`)
- File-based: `--sudo-pass-file ~/.mooncake/sudo_pass`
- Environment variable: `export SUDO_ASKPASS=/usr/bin/ssh-askpass`

[Learn more: Sudo →](examples/09-sudo.md)

---

## Quick Commands Reference

```bash
# Run configuration
mooncake run --config config.yml

# Preview changes (safe!)
mooncake run --config config.yml --dry-run

# Show system facts
mooncake facts
mooncake facts --format json

# Generate execution plan
mooncake plan --config config.yml
mooncake plan --config config.yml --format json --output plan.json

# Filter by tags
mooncake run --config config.yml --tags dev,test

# With sudo
mooncake run --config config.yml --ask-become-pass

# Execute from plan
mooncake run --from-plan plan.json

# Debug mode
mooncake run --config config.yml --log-level debug

# Disable TUI (for CI/CD)
mooncake run --config config.yml --raw

# JSON output
mooncake run --config config.yml --raw --output-format json
```

[See all commands →](guide/commands.md)

---

## Common Use Cases

### Dotfiles Management

Deploy and manage dotfiles across machines:

```yaml
- name: Create backup directory
  file:
    path: ~/.dotfiles-backup
    state: directory

- name: Deploy dotfiles
  shell: cp "{{item.src}}" "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

[See complete example →](examples/real-world-dotfiles.md)

---

### Development Environment Setup

Automate dev tool installation:

```yaml
- vars:
    dev_tools:
      - neovim
      - ripgrep
      - fzf
      - tmux
      - docker

- name: Install dev tools (macOS)
  shell: brew install {{item}}
  with_items: "{{dev_tools}}"
  when: os == "darwin"

- name: Install dev tools (Linux)
  shell: apt install -y {{item}}
  become: true
  with_items: "{{dev_tools}}"
  when: os == "linux" && package_manager == "apt"
```

---

### Multi-OS Configuration

Write once, run anywhere:

```yaml
- name: Install on Linux
  shell: apt install neovim
  become: true
  when: os == "linux"

- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Windows
  shell: choco install neovim
  when: os == "windows"
```

---

### System Provisioning

Set up new machines automatically:

```yaml
- name: Install system packages
  shell: "{{package_manager}} install {{item}}"
  become: true
  with_items:
    - git
    - curl
    - vim
    - htop
  when: os == "linux"

- name: Create user directories
  file:
    path: "{{item}}"
    state: directory
  with_items:
    - ~/.local/bin
    - ~/.config
    - ~/projects
    - ~/backup

- name: Deploy SSH config
  template:
    src: ./ssh_config.j2
    dest: ~/.ssh/config
    mode: "0600"
```

---

## Comparison

| Feature | Mooncake | Ansible | Shell Scripts |
|---------|----------|---------|---------------|
| **Setup** | Single binary | Python + modules | Text editor |
| **Dependencies** | None | Python, modules | System tools |
| **AI Agent Friendly** | ✅ Native support | ⚠️ Complex | ❌ Unsafe |
| **Dry-run** | ✅ Native | ✅ Check mode | ❌ Manual |
| **Idempotency** | ✅ Guaranteed | ✅ Yes | ❌ Manual |
| **Cross-platform** | ✅ Built-in | ⚠️ Limited | ❌ OS-specific |
| **System Facts** | ✅ Auto-detected | ✅ Gathered | ❌ Manual |
| **Best For** | AI agents, dotfiles | Enterprise automation | Quick tasks |

**Mooncake is the execution layer for AI-driven system configuration** - providing safety, validation, and observability that AI agents need.

---

## Next Steps

1. **[Quick Start →](getting-started/quick-start.md)** - Get running in 30 seconds
2. **[Examples →](examples/)** - Learn by doing
3. **[Actions Guide →](guide/config/actions.md)** - See what you can do
4. **[Complete Reference →](guide/config/reference.md)** - All properties

---

## Community & Support

- [:fontawesome-brands-github: GitHub Issues](https://github.com/alehatsman/mooncake/issues) - Report bugs and request features
- [:material-star: Star the project](https://github.com/alehatsman/mooncake) if you find it useful!
- [:material-book-open: Contributing Guide](development/contributing.md) - Help make Mooncake better
- [:material-map: Roadmap](development/roadmap.md) - Planned features
- [:material-history: Changelog](about/changelog.md) - What's new

---

## License

MIT License - Copyright (c) 2024-2026 Aleh Atsman

See [LICENSE](https://github.com/alehatsman/mooncake/blob/master/LICENSE) for details.
