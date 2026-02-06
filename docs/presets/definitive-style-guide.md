# The Definitive Mooncake Preset Style Guide

**Version**: 1.0.0
**Last Updated**: 2026-02-06
**Purpose**: Production-ready standards for creating high-quality, consistent Mooncake presets

This guide defines the gold standard for creating Mooncake presets. Following these patterns ensures presets are discoverable, maintainable, and provide excellent user experience for both humans and AI agents.

---

## Table of Contents

1. [Philosophy & Principles](#philosophy--principles)
2. [Preset Structure](#preset-structure)
3. [Documentation Standards](#documentation-standards)
4. [Parameter Design](#parameter-design)
5. [Task Organization](#task-organization)
6. [Platform Handling](#platform-handling)
7. [Idempotency & Change Detection](#idempotency--change-detection)
8. [Error Handling](#error-handling)
9. [Testing & Validation](#testing--validation)
10. [Examples & Templates](#examples--templates)
11. [Checklist](#checklist)

---

## Philosophy & Principles

### Core Values

**1. Simplicity First**
- Presets should make complex operations simple, not simple operations complex
- Minimize required parameters, maximize sensible defaults
- A basic installation should be ONE command: `preset: tool-name`

**2. Copy-Paste Ready**
- Every example must work without modification
- No placeholders without clear substitution instructions
- Provide complete, working configurations

**3. Production Grade**
- Assume presets run on real infrastructure
- Include error handling, validation, and safety checks
- Document security implications and best practices

**4. AI-Agent Friendly**
- Structure documentation for both human and LLM consumption
- Include "Agent Use" sections describing automation use cases
- Provide machine-readable success criteria

**5. Discoverability**
- Users should understand what a preset does in 10 seconds
- Quick Start section must come first
- Common operations clearly documented

---

## Preset Structure

### Directory Layout

For complex presets (with templates, multiple task files):

```
presets/
└── tool-name/
    ├── preset.yml              # Main preset definition (orchestration)
    ├── README.md               # User-facing documentation
    ├── tasks/                  # Modular task files
    │   ├── install.yml        # Installation logic
    │   ├── configure.yml      # Service/config setup
    │   ├── verify.yml         # Health checks (optional)
    │   └── uninstall.yml      # Cleanup tasks
    ├── templates/              # Configuration templates
    │   ├── service.conf.j2    # Service configs
    │   └── config.yml.j2      # App configs
    └── files/                  # Static files (optional)
        └── defaults.conf
```

For simple presets (single action, minimal logic):

```
presets/
└── tool-name.yml              # Flat format - all in one file
```

**When to use directory format:**
- Tool requires service configuration (systemd/launchd)
- Multiple installation methods (package manager, script, source)
- Platform-specific logic (Linux vs macOS vs Windows)
- Template files needed for configuration
- More than 50 lines of preset logic

**When to use flat format:**
- Simple package installation (single command)
- No service configuration
- Minimal platform differences
- No templates or additional files

### preset.yml Structure

```yaml
name: tool-name
description: One-line description of what this preset does
version: 1.0.0

parameters:
  state:
    type: string
    required: false
    default: present
    enum: [present, absent]
    description: Whether tool should be installed or removed

  # Additional parameters...

steps:
  # Use include for complex presets
  - name: Install tool
    include: tasks/install.yml
    when: parameters.state == "present"

  - name: Configure service
    include: tasks/configure.yml
    when: parameters.state == "present" and parameters.service

  - name: Uninstall tool
    include: tasks/uninstall.yml
    when: parameters.state == "absent"
```

**Naming Conventions:**
- **Preset name**: Use tool's official name (lowercase, hyphens for multi-word)
  - ✅ `kubectl`, `helm`, `modern-unix`
  - ❌ `kube-ctl`, `Helm`, `modern_unix`
- **Task files**: Action-oriented, lowercase
  - `install.yml`, `configure.yml`, `uninstall.yml`, `verify.yml`
- **Templates**: Descriptive with `.j2` extension
  - `systemd-service.conf.j2`, `config.yml.j2`

---

## Documentation Standards

### README.md Structure

Every preset MUST have a README.md with these sections in this order:

```markdown
# Tool Name - Brief Description

One-sentence description of what this tool does.

## Quick Start
```yaml
- preset: tool-name
```

## Features
- Feature 1
- Feature 2
- Feature 3

## Basic Usage
```bash
# Most common commands with actual examples
tool-name --version
tool-name command arg
```

## Advanced Configuration
```yaml
- preset: tool-name
  with:
    param1: value1
    param2: value2
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |

## Platform Support
- ✅ Linux (apt, dnf, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `/path/to/config`
- **Data directory**: `/path/to/data`
- **Port**: 8080

## Real-World Examples
Practical use cases showing tool in context

## Agent Use
How AI agents can leverage this tool:
- Use case 1
- Use case 2

## Uninstall
```yaml
- preset: tool-name
  with:
    state: absent
```

## Resources
- Official docs: https://...
- Search: "tool-name tutorial"
```

### Section Guidelines

#### 1. Quick Start (REQUIRED)
- **First code block users see**
- **Must work without modification**
- **Shows simplest possible usage**

```markdown
## Quick Start
```yaml
- preset: kubectl
```
```

#### 2. Features (REQUIRED)
- **Bullet list of key capabilities**
- **Focus on value, not implementation**
- **3-8 items maximum**

```markdown
## Features
- **Cross-platform**: Linux, macOS, BSD
- **Fast**: Written in Rust, minimal overhead
- **Beautiful**: Color-coded output with icons
- **Smart**: Respects .gitignore by default
```

#### 3. Basic Usage (REQUIRED)
- **Real commands users will run**
- **Common operations first**
- **Include expected output when helpful**

```markdown
## Basic Usage
```bash
# Check version
kubectl version --client

# List pods
kubectl get pods

# Create deployment
kubectl create deployment nginx --image=nginx
```
```

#### 4. Advanced Configuration (REQUIRED)
- **Show all parameter options**
- **Real working examples, not just parameter lists**
- **Group related parameters**

```markdown
## Advanced Configuration
```yaml
- preset: kubectl
  with:
    version: "1.29.0"              # Specific version
    configure_completion: true     # Shell completion
    install_krew: true             # Plugin manager
    krew_plugins:
      - ctx                        # Context switcher
      - ns                         # Namespace switcher
```
```

#### 5. Real-World Examples (HIGHLY RECOMMENDED)
- **Show tool in realistic scenarios**
- **Complete workflows, not isolated commands**
- **Include context (why you'd do this)**

```markdown
## Real-World Examples

### CI/CD Pipeline
```bash
# Check disk space before build
if duf --json / | jq '.[0].usage' | awk '$1 > 90 {exit 1}'; then
  echo "ERROR: Disk usage above 90%"
  exit 1
fi
```

### Development Workflow
```bash
# Extract API response field
curl https://api.example.com/data | jq '.users[].email'
```
```

#### 6. Agent Use (REQUIRED)
- **How AI agents can use this tool**
- **Automation-friendly use cases**
- **Decision criteria for agents**

```markdown
## Agent Use
- Parse and extract data from JSON APIs
- Transform configuration files in deployment pipelines
- Filter and aggregate log data
- Validate API responses in test suites
- Generate reports from structured data
```

#### 7. Configuration (RECOMMENDED)
- **File locations (absolute paths)**
- **Default ports/addresses**
- **Directory permissions**
- **Environment variables**

```markdown
## Configuration
- **Config file**: `~/.config/tool/config.yml` (Linux), `~/Library/Preferences/tool/config.yml` (macOS)
- **Data directory**: `~/.local/share/tool/` (Linux), `~/Library/Application Support/tool/` (macOS)
- **Cache**: `~/.cache/tool/` (Linux), `~/Library/Caches/tool/` (macOS)
- **Default port**: 8080
```

#### 8. Platform Support (REQUIRED)
- **Clear matrix of what works where**
- **Use ✅ ❌ symbols for clarity**
- **Note installation methods**

```markdown
## Platform Support
- ✅ Linux (systemd, apt, dnf, yum, pacman, zypper)
- ✅ macOS (launchd, Homebrew)
- ❌ Windows (not yet supported)
```

#### 9. Troubleshooting (RECOMMENDED)
- **Common issues and solutions**
- **How to check logs**
- **Debug mode instructions**

```markdown
## Troubleshooting

### Service won't start
Check logs:
```bash
journalctl -u service-name -f  # Linux
tail -f ~/Library/Logs/service.log  # macOS
```

### Permission errors
Most operations require `become: true` (sudo).
```

#### 10. Resources (REQUIRED)
- **Official documentation link**
- **Search suggestions (for AI agents)**
- **Community resources**

```markdown
## Resources
- Official docs: https://tool.example.com/docs/
- GitHub: https://github.com/org/tool
- Search: "tool-name tutorial", "tool-name best practices"
```

### Writing Style

**DO:**
- ✅ Use active voice ("Install Docker" not "Docker installation")
- ✅ Write concise descriptions (one sentence per bullet)
- ✅ Include concrete examples, not abstract descriptions
- ✅ Use consistent terminology throughout
- ✅ Add context to code blocks (what it does, when to use)

**DON'T:**
- ❌ Write marketing copy ("the best tool", "amazing")
- ❌ Use vague placeholders (`<your-value>` without guidance)
- ❌ Assume prior knowledge (explain domain-specific terms)
- ❌ Include incomplete examples
- ❌ Copy-paste from tool's docs without adaptation

### Code Block Standards

**Always include:**
- Language identifier (yaml, bash, python, etc.)
- Context comment (what this does, when to use)
- Complete working example

```markdown
## Example
```yaml
# Production deployment with custom settings
- preset: myapp
  with:
    environment: production
    replicas: 3
    enable_monitoring: true
  become: true
```
```

**DON'T:**
```markdown
## Example
```
preset: myapp
  with:
    environment: <your-env>  # ❌ Placeholder without guidance
```
```

---

## Parameter Design

### Standard Parameters

**Every preset SHOULD support:**

```yaml
parameters:
  state:
    type: string
    required: false
    default: present
    enum: [present, absent]
    description: Whether tool should be installed or removed
```

### Parameter Naming Conventions

**Standard names** (use these for consistency):
- `state`: Installation state (present/absent)
- `version`: Specific version to install
- `service`: Enable as system service (bool)
- `configure`: Run configuration steps (bool)
- `force`: Force reinstall/reconfigure (bool)
- `method`: Installation method (auto/package/script)
- `port`: Network port number
- `host`: Bind address
- `data_dir`: Data storage location
- `config_file`: Configuration file path

**Naming rules:**
- Use snake_case (not camelCase)
- Be specific: `database_url` not `url`
- Avoid abbreviations: `configuration` not `cfg`
- Use singular for single values: `port` not `ports`
- Use plural for arrays: `models` not `model`

### Parameter Types

```yaml
parameters:
  # String - text values
  environment:
    type: string
    enum: [development, staging, production]
    default: development
    description: Deployment environment

  # Boolean - yes/no flags
  enable_monitoring:
    type: bool
    default: false
    description: Enable Prometheus metrics endpoint

  # Array - lists
  features:
    type: array
    default: []
    description: List of feature flags to enable

  # Object - structured data
  config:
    type: object
    required: false
    description: Additional configuration options
```

### Default Values Strategy

**Principle**: Defaults should work for 80% of users.

```yaml
# ✅ Good - sensible production defaults
parameters:
  port:
    type: number
    default: 8080
    description: Application port

  workers:
    type: number
    default: 4
    description: Number of worker processes

  log_level:
    type: string
    default: info
    enum: [debug, info, warn, error]
    description: Logging verbosity

# ❌ Bad - forces users to specify everything
parameters:
  port:
    type: number
    required: true  # Why? 8080 is fine for most
    description: Application port
```

### Parameter Validation

**Use enum for limited choices:**
```yaml
parameters:
  state:
    type: string
    enum: [present, absent]  # Only these values allowed
    description: Installation state
```

**Document valid ranges:**
```yaml
parameters:
  port:
    type: number
    default: 8080
    description: Application port (1024-65535)
```

**Describe format requirements:**
```yaml
parameters:
  version:
    type: string
    default: latest
    description: Version to install (e.g., '1.2.3', 'latest')
```

### Required vs Optional

**Make required ONLY when:**
- No sensible default exists
- Value is user-specific (API keys, hostnames)
- Incorrect guess would be dangerous

**Examples:**

```yaml
# ✅ Optional with default - most users want service
parameters:
  service:
    type: bool
    default: true
    description: Enable and start system service

# ✅ Required - no safe default
parameters:
  database_password:
    type: string
    required: true
    description: Database password for application

# ❌ Bad - has obvious default
parameters:
  install:
    type: bool
    required: true  # Just default to true!
    description: Whether to install
```

---

## Task Organization

### Task File Structure

**Principle**: One file per logical phase

```yaml
# tasks/install.yml - Installation logic only
- name: Check if tool exists
  shell: command -v tool-name
  register: check
  failed_when: false

- name: Install via package manager
  shell: apt-get install -y tool-name
  when: apt_available and check.rc != 0
  become: true

- name: Install via Homebrew
  shell: brew install tool-name
  when: brew_available and check.rc != 0

- name: Install via script
  shell: curl -fsSL https://get.tool.sh | sh
  when: check.rc != 0 and not (apt_available or brew_available)
  become: true
```

### Task File Guidelines

**install.yml:**
- Platform detection
- Multiple installation methods with fallback
- Idempotency (check if already installed)
- Exit early if already present

**configure.yml:**
- Create configuration files
- Set up service files (systemd/launchd)
- Apply configuration changes
- Restart services if needed

**verify.yml** (optional):
- Health checks
- Connectivity tests
- Version verification
- Configuration validation

**uninstall.yml:**
- Stop services
- Remove binaries
- Clean up configuration (optional)
- Remove data directories (only with force: true)

### Step Naming

**Template:**
```
[Action verb] [object] [context]
```

**Examples:**
```yaml
# ✅ Good - clear action and object
- name: Install kubectl binary
- name: Configure systemd service
- name: Pull Docker image
- name: Create config directory
- name: Stop running service

# ❌ Bad - vague or passive
- name: Installation  # What's being installed?
- name: Setup  # Too vague
- name: The service is configured  # Passive voice
```

### Conditional Logic Patterns

**Platform detection:**
```yaml
# ✅ Use system facts
- name: Install via apt
  shell: apt-get install -y tool
  when: apt_available
  become: true

- name: Install via Homebrew
  shell: brew install tool
  when: brew_available

# ❌ Don't hardcode OS checks
- name: Install on Linux
  shell: apt-get install -y tool
  when: os == "linux"  # Not all Linux has apt!
```

**Parameter-based:**
```yaml
- name: Configure service
  include: tasks/service.yml
  when: parameters.service == true

- name: Pull models
  include: tasks/models.yml
  when: parameters.models | length > 0
```

**State-based:**
```yaml
- name: Install workflow
  include: tasks/install.yml
  when: parameters.state == "present"

- name: Uninstall workflow
  include: tasks/uninstall.yml
  when: parameters.state == "absent"
```

---

## Platform Handling

### Detection Strategy

**Use system facts, not OS checks:**

```yaml
# ✅ Good - specific capability detection
- name: Install via apt
  shell: apt-get install -y {{ tool }}
  when: apt_available
  become: true

- name: Install via dnf
  shell: dnf install -y {{ tool }}
  when: dnf_available
  become: true

- name: Install via Homebrew
  shell: brew install {{ tool }}
  when: brew_available

# ❌ Bad - broad OS checks
- name: Install on Linux
  shell: apt-get install -y {{ tool }}  # Assumes apt!
  when: os == "linux"
  become: true
```

### Available Facts

**Package managers:**
- `apt_available` (Debian, Ubuntu)
- `dnf_available` (Fedora, RHEL 8+)
- `yum_available` (CentOS, RHEL 7)
- `pacman_available` (Arch)
- `zypper_available` (openSUSE)
- `apk_available` (Alpine)
- `brew_available` (macOS, Linux)
- `port_available` (macOS)

**Operating systems:**
- `os` ("linux", "darwin", "windows")
- `arch` ("amd64", "arm64")
- `hostname`

**System info:**
- `cpu_cores`
- `memory_total_mb`
- `distribution` (Linux only: "ubuntu", "fedora", etc.)

### Service Configuration

**systemd (Linux):**
```yaml
- name: Configure systemd service
  service:
    name: myapp
    state: started
    enabled: true
    unit:
      content: |
        [Unit]
        Description={{ parameters.description }}
        After=network.target

        [Service]
        Type=simple
        ExecStart=/usr/local/bin/myapp
        Restart=always
        Environment="PORT={{ parameters.port }}"

        [Install]
        WantedBy=multi-user.target
  when: os == "linux"
  become: true
```

**launchd (macOS):**
```yaml
- name: Configure launchd service
  service:
    name: com.example.myapp
    state: started
    enabled: true
    unit:
      content: |
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
          "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
          <key>Label</key>
          <string>com.example.myapp</string>
          <key>ProgramArguments</key>
          <array>
            <string>/usr/local/bin/myapp</string>
          </array>
          <key>RunAtLoad</key>
          <true/>
          <key>KeepAlive</key>
          <true/>
          <key>EnvironmentVariables</key>
          <dict>
            <key>PORT</key>
            <string>{{ parameters.port }}</string>
          </dict>
        </dict>
        </plist>
  when: os == "darwin"
```

### Installation Method Hierarchy

**Preferred order:**
1. Package manager (most reliable, gets updates)
2. Official installation script (maintained by tool vendor)
3. Binary download (with checksum verification)
4. Source compilation (last resort)

```yaml
- name: Try package manager install
  shell: "{{ package_manager }} install -y {{ tool }}"
  when: package_manager_available
  register: pkg_install
  failed_when: false
  become: true

- name: Fall back to official script
  shell: curl -fsSL https://get.tool.sh | sh
  when: pkg_install.rc != 0 or not package_manager_available
  become: true
```

---

## Idempotency & Change Detection

### Principle

**Every step should be safe to run multiple times.**

First run: Makes changes, reports `changed: true`
Second run: No changes needed, reports `changed: false`

### Check Before Action

```yaml
# ✅ Good - idempotent
- name: Check if tool installed
  shell: command -v tool-name
  register: check
  failed_when: false

- name: Install tool
  shell: curl -fsSL https://get.tool.sh | sh
  when: check.rc != 0
  become: true

# ❌ Bad - always runs, always reports changed
- name: Install tool
  shell: curl -fsSL https://get.tool.sh | sh
  become: true
```

### Use Built-in Idempotency

```yaml
# file action is idempotent
- name: Create directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"

# download with checksum is idempotent
- name: Download binary
  download:
    url: https://example.com/tool-v1.2.3
    dest: /usr/local/bin/tool
    checksum: "sha256:abc123..."
    mode: "0755"

# service action is idempotent
- name: Start service
  service:
    name: myapp
    state: started
    enabled: true
```

### Marker Files

```yaml
# Use 'creates' for one-time operations
- name: Initialize database
  shell: pg_restore backup.sql
  creates: /var/lib/postgresql/.initialized
  become: true

# Use marker files for complex operations
- name: Run one-time setup
  shell: |
    # Complex multi-step setup
    ./setup.sh
    touch /opt/myapp/.setup-complete
  creates: /opt/myapp/.setup-complete
```

### Changed Detection

```yaml
# Override changed status based on output
- name: Pull Docker image
  shell: docker pull nginx:latest
  register: pull
  changed_when: "'Downloaded' in pull.stdout"

# Check if update is needed
- name: Update git repository
  shell: git pull
  cwd: /opt/repo
  register: git_pull
  changed_when: "'Already up to date' not in git_pull.stdout"
```

---

## Error Handling

### Validation

**Validate early:**
```yaml
# Validate required files exist
- name: Verify config file exists
  assert:
    file:
      path: "{{ parameters.config_file }}"
      exists: true
  when: parameters.config_file is defined

# Validate connectivity
- name: Check database connection
  shell: pg_isready -h {{ db_host }}
  when: parameters.verify_connection

# Validate version format
- name: Check version format
  shell: echo "{{ parameters.version }}" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$'
  when: parameters.version != "latest"
```

### Graceful Degradation

```yaml
# Try preferred method, fall back gracefully
- name: Try package manager install
  shell: apt-get install -y tool
  when: apt_available
  register: apt_result
  failed_when: false
  become: true

- name: Fall back to script install
  shell: curl -fsSL https://get.tool.sh | sh
  when: apt_result.rc != 0 or not apt_available
  become: true
```

### Clear Error Messages

```yaml
# ✅ Good - helpful error message
- name: Check prerequisites
  assert:
    command:
      cmd: docker --version
      exit_code: 0
  register: docker_check
  failed_when: docker_check.rc != 0
  # User sees: "assertion failed (command): expected exit code 0, got exit code 127"

# ❌ Bad - cryptic failure
- name: Setup
  shell: ./setup.sh  # Fails with no context
```

### Cleanup on Failure

```yaml
# Use register + conditional cleanup
- name: Download archive
  download:
    url: https://example.com/tool.tar.gz
    dest: /tmp/tool.tar.gz
  register: download_result

- name: Extract archive
  unarchive:
    src: /tmp/tool.tar.gz
    dest: /opt/tool
    strip_components: 1

- name: Cleanup download
  file:
    path: /tmp/tool.tar.gz
    state: absent
  when: download_result.changed
```

---

## Testing & Validation

### Dry Run Testing

**Every preset MUST work with `--dry-run`:**

```bash
# Test preset without making changes
mooncake run -c test.yml --dry-run

# Verify output shows intended actions
```

### Multi-Platform Testing

**Test matrix:**
- [ ] Ubuntu 22.04 (apt)
- [ ] Fedora 39 (dnf)
- [ ] macOS 14 (Homebrew)
- [ ] Arch Linux (pacman) - if claiming support

### Idempotency Testing

```bash
# Run preset twice - second run should report no changes
mooncake run -c test.yml
mooncake run -c test.yml  # Should show changed: false
```

### Verification Steps

**Include verification in preset:**
```yaml
- name: Verify installation
  assert:
    command:
      cmd: tool-name --version
      exit_code: 0

- name: Verify service running
  assert:
    command:
      cmd: systemctl is-active tool-service
      exit_code: 0
  when: parameters.service and os == "linux"

- name: Verify API responding
  assert:
    http:
      url: "http://localhost:{{ parameters.port }}/health"
      status: 200
  when: parameters.service
```

### Test Playbook Template

```yaml
# test-preset.yml
- name: Test basic installation
  preset: my-tool
  become: true

- name: Verify installed
  shell: command -v my-tool
  register: check

- name: Test with all options
  preset: my-tool
  with:
    version: "1.2.3"
    service: true
    configure: true
  become: true

- name: Verify service running
  shell: systemctl is-active my-tool
  when: os == "linux"

- name: Test uninstall
  preset: my-tool
  with:
    state: absent
  become: true

- name: Verify removed
  shell: command -v my-tool
  register: removed
  failed_when: removed.rc == 0
```

---

## Examples & Templates

### Simple Tool Preset Template

```yaml
# presets/simple-tool.yml
name: simple-tool
description: Install simple-tool CLI utility
version: 1.0.0

parameters:
  state:
    type: string
    default: present
    enum: [present, absent]
    description: Install or remove tool

steps:
  # Check if installed
  - name: Check if tool exists
    shell: command -v simple-tool
    register: check
    failed_when: false

  # Installation
  - name: Install via apt
    shell: apt-get install -y simple-tool
    when: parameters.state == "present" and apt_available and check.rc != 0
    become: true

  - name: Install via brew
    shell: brew install simple-tool
    when: parameters.state == "present" and brew_available and check.rc != 0

  # Uninstallation
  - name: Uninstall via apt
    shell: apt-get remove -y simple-tool
    when: parameters.state == "absent" and apt_available
    become: true

  - name: Uninstall via brew
    shell: brew uninstall simple-tool
    when: parameters.state == "absent" and brew_available
```

### Complex Preset Template

```yaml
# presets/complex-tool/preset.yml
name: complex-tool
description: Install and configure complex-tool with service management
version: 1.0.0

parameters:
  state:
    type: string
    default: present
    enum: [present, absent]
    description: Install or remove tool

  version:
    type: string
    default: latest
    description: Version to install

  service:
    type: bool
    default: true
    description: Configure as system service

  port:
    type: number
    default: 8080
    description: Service port (1024-65535)

  data_dir:
    type: string
    required: false
    description: Custom data directory

steps:
  - name: Install complex-tool
    include: tasks/install.yml
    when: parameters.state == "present"

  - name: Configure service
    include: tasks/configure.yml
    when: parameters.state == "present" and parameters.service

  - name: Verify installation
    include: tasks/verify.yml
    when: parameters.state == "present"

  - name: Uninstall complex-tool
    include: tasks/uninstall.yml
    when: parameters.state == "absent"
```

### README Template

```markdown
# Tool Name - One-Line Description

Brief paragraph describing what this tool does and why it's useful.

## Quick Start
```yaml
- preset: tool-name
```

## Features
- **Feature 1**: Description
- **Feature 2**: Description
- **Feature 3**: Description
- **Cross-platform**: Linux, macOS

## Basic Usage
```bash
# Most common operation
tool-name command

# Second most common
tool-name other-command arg
```

## Advanced Configuration
```yaml
- preset: tool-name
  with:
    version: "1.2.3"
    option1: value1
    option2: true
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |
| version | string | latest | Version to install |
| option1 | string | - | Description |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `/etc/tool/config.yml`
- **Data directory**: `/var/lib/tool/`
- **Port**: 8080

## Real-World Examples

### Use Case 1
```bash
# Context and explanation
commands here
```

### Use Case 2
```yaml
# Mooncake example
- preset: tool-name
  with:
    specific: configuration
```

## Agent Use
- Use case for AI agents
- Another automation scenario
- Integration pattern

## Troubleshooting

### Problem 1
Solution:
```bash
command to fix
```

### Problem 2
Solution and explanation.

## Uninstall
```yaml
- preset: tool-name
  with:
    state: absent
```

## Resources
- Official docs: https://tool.example.com/docs
- GitHub: https://github.com/org/tool
- Search: "tool-name tutorial", "tool-name examples"
```

---

## Checklist

### Before Submitting Preset

**Structure:**
- [ ] Directory structure follows conventions
- [ ] All files in correct locations
- [ ] No unnecessary files included

**preset.yml:**
- [ ] Name matches directory/filename
- [ ] Description is clear and concise (one line)
- [ ] Version follows semver (1.0.0)
- [ ] All parameters have descriptions
- [ ] Parameters use consistent naming (snake_case)
- [ ] Defaults are sensible for common use
- [ ] Steps are well-organized (install/configure/uninstall)
- [ ] Conditionals use system facts (not hardcoded OS checks)

**README.md:**
- [ ] Quick Start section comes first
- [ ] Quick Start example works without modification
- [ ] Features section lists 3-8 key capabilities
- [ ] Basic Usage shows real commands
- [ ] Advanced Configuration has working examples
- [ ] Parameters table is complete and accurate
- [ ] Platform Support clearly lists OS/package managers
- [ ] Configuration section lists file locations
- [ ] Agent Use section describes automation scenarios
- [ ] Uninstall instructions provided
- [ ] Resources include official docs and search terms

**Code Quality:**
- [ ] All steps have descriptive names
- [ ] No hardcoded values (use parameters)
- [ ] Idempotency: runs safely multiple times
- [ ] Error handling: graceful failures
- [ ] Platform detection: uses facts not OS checks
- [ ] Templates use .j2 extension
- [ ] Task files use action-oriented names

**Testing:**
- [ ] Tested with `--dry-run`
- [ ] Tested on at least one Linux distro
- [ ] Tested on macOS (if claiming support)
- [ ] Run twice - second run reports no changes
- [ ] Uninstall tested and verified
- [ ] All examples in README tested

**Documentation:**
- [ ] No typos or grammatical errors
- [ ] Code blocks have language identifiers
- [ ] All examples are complete and working
- [ ] No dead links
- [ ] Search terms provided for AI agents

---

## Version History

**1.0.0** (2026-02-06)
- Initial comprehensive style guide
- Consolidated patterns from 16 production presets
- Added templates and checklists
- Defined documentation standards

---

## Contributing

This guide is a living document. When you create a great preset that establishes a new pattern, update this guide with that pattern.

**To propose changes:**
1. Create example preset demonstrating the pattern
2. Document the pattern with rationale
3. Update relevant sections of this guide
4. Submit for review

**Principles for guide updates:**
- Patterns must be proven in production presets
- Keep the guide concise - quality over quantity
- Examples must be complete and tested
- Optimize for both human and AI comprehension
