# Quick Reference

A one-page cheat sheet for common Mooncake operations.

---

## Installation & Setup

```bash
# Install
go install github.com/alehatsman/mooncake@latest

# Verify
mooncake --version

# Get help
mooncake --help
mooncake run --help
```

---

## Basic Commands

```bash
# Run configuration
mooncake run --config config.yml

# Preview changes (dry-run)
mooncake run --config config.yml --dry-run

# Show system facts
mooncake facts
mooncake facts --format json

# Generate execution plan
mooncake plan --config config.yml
mooncake plan --config config.yml --format json --output plan.json

# Execute from plan
mooncake run --from-plan plan.json

# Filter by tags
mooncake run --config config.yml --tags dev,test

# With sudo password
mooncake run --config config.yml --ask-become-pass
mooncake run --config config.yml -K  # shorthand

# Disable TUI (for CI/CD)
mooncake run --config config.yml --raw

# JSON output
mooncake run --config config.yml --raw --output-format json

# Debug mode
mooncake run --config config.yml --log-level debug
```

---

## Presets

```bash
# List all available presets
mooncake presets list

# Install preset interactively
mooncake presets -K

# Install specific preset
mooncake presets install docker
mooncake presets install -K postgres  # with sudo

# Show preset status
mooncake presets status
mooncake presets status docker

# Uninstall preset
mooncake presets uninstall docker
```

---

## Configuration Structure

```yaml
# Basic step
- name: Step description
  action_name:
    parameter: value

# With variables
- vars:
    my_var: value

# With conditionals
- name: Only on Linux
  shell: echo "Linux!"
  when: os == "linux"

# With loops
- name: Install packages
  shell: apt install {{item}}
  with_items: [git, vim, curl]
  become: true

# With tags
- name: Dev setup
  shell: install-dev.sh
  tags: [dev, setup]
```

---

## Common Actions

### Shell Command
```yaml
- name: Run command
  shell: echo "Hello {{os}}"

- name: Multi-line script
  shell: |
    apt update
    apt install -y neovim
  become: true
  timeout: 5m
```

### File Operations
```yaml
# Create file
- name: Create config file
  file:
    path: ~/.config/app.conf
    state: file
    content: "key=value"
    mode: "0644"

# Create directory
- name: Create directory
  file:
    path: ~/.local/bin
    state: directory
    mode: "0755"

# Create symlink
- name: Create link
  file:
    path: ~/bin/myapp
    state: link
    target: /usr/local/bin/myapp

# Remove file
- name: Remove file
  file:
    path: /tmp/old-file
    state: absent
```

### Template Rendering
```yaml
- name: Render nginx config
  template:
    src: ./nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    vars:
      port: 8080
      workers: 4
```

### Copy Files
```yaml
- name: Copy with backup
  copy:
    src: ./app.conf
    dest: /etc/app.conf
    mode: "0644"
    backup: true
```

### Download Files
```yaml
- name: Download file
  download:
    url: https://example.com/file.tar.gz
    dest: /tmp/file.tar.gz
    checksum: sha256:abc123...
    timeout: 10m
    retries: 3
```

### Extract Archives
```yaml
- name: Extract tarball
  unarchive:
    src: /tmp/archive.tar.gz
    dest: /opt/app
    strip_components: 1
```

### Service Management
```yaml
- name: Start and enable service
  service:
    name: nginx
    state: started
    enabled: true
  become: true
```

### Assertions
```yaml
# Verify command
- name: Check Docker installed
  assert:
    command:
      cmd: docker --version
      exit_code: 0

# Verify file
- name: Check file exists
  assert:
    file:
      path: /etc/nginx/nginx.conf
      exists: true
      mode: "0644"

# Verify HTTP
- name: Check API health
  assert:
    http:
      url: https://api.example.com/health
      status: 200
```

### Presets
```yaml
- name: Install Ollama
  preset: ollama
  with:
    state: present
    service: true
    pull: [llama3.1:8b]
  become: true
```

---

## Variables & Facts

### Define Variables
```yaml
- vars:
    app_name: MyApp
    version: "1.0.0"
    packages:
      - git
      - vim
      - curl

- name: Use variable
  shell: echo "Installing {{app_name}} v{{version}}"
```

### Auto-Detected Facts
```yaml
# Available system facts
{{os}}                    # darwin, linux, windows
{{arch}}                  # amd64, arm64
{{distribution}}          # ubuntu, fedora, arch, macos
{{package_manager}}       # apt, dnf, yum, brew, choco
{{cpu_cores}}             # Number of CPU cores
{{memory_total_mb}}       # Total RAM in MB
{{hostname}}              # System hostname
{{kernel_version}}        # Kernel version
{{python_version}}        # Python version (if installed)
{{docker_version}}        # Docker version (if installed)
{{git_version}}           # Git version (if installed)
```

---

## Control Flow

### Conditionals
```yaml
# Simple condition
- name: Linux only
  shell: apt update
  when: os == "linux"

# Multiple conditions (AND)
- name: Ubuntu with apt
  shell: apt install vim
  when: os == "linux" && package_manager == "apt"

# OR condition
- name: macOS or Linux
  shell: echo "Unix system"
  when: os == "darwin" || os == "linux"

# Negation
- name: Not Windows
  shell: echo "Not Windows"
  when: os != "windows"

# Check variable
- name: If defined
  shell: echo "{{my_var}}"
  when: my_var is defined
```

### Operators
- `==` Equal
- `!=` Not equal
- `>` Greater than
- `<` Less than
- `>=` Greater than or equal
- `<=` Less than or equal
- `&&` AND
- `||` OR
- `!` NOT
- `in` Contains
- `is defined` / `is not defined`

### Loops
```yaml
# Loop over list
- name: Install package
  shell: brew install {{item}}
  with_items:
    - neovim
    - ripgrep
    - fzf

# Loop over files
- name: Deploy dotfile
  copy:
    src: "{{item.src}}"
    dest: "~/{{item.name}}"
  with_filetree: ./dotfiles
  when: item.is_dir == false
```

### Tags
```yaml
# Tag steps
- name: Dev setup
  shell: setup-dev.sh
  tags: [dev, setup]

- name: Production deploy
  shell: deploy-prod.sh
  tags: [prod, deploy]
```

Run with tags:
```bash
mooncake run --config config.yml --tags dev
mooncake run --config config.yml --tags dev,test  # OR logic
```

---

## Execution Control

### Timeout & Retry
```yaml
- name: Download with retry
  shell: curl -O https://example.com/file.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s
```

### Changed/Failed Conditions
```yaml
- name: Custom changed detection
  shell: make install
  register: result
  changed_when: "'installed' in result.stdout"

- name: Custom failure detection
  shell: curl https://example.com
  register: result
  failed_when: result.rc != 0 and result.rc != 18
```

### Result Registration
```yaml
- name: Check status
  shell: systemctl is-active nginx
  register: nginx_status
  ignore_errors: true

- name: Restart if not running
  service:
    name: nginx
    state: restarted
  when: nginx_status.rc != 0
  become: true
```

---

## Sudo Operations

```yaml
# Inline sudo
- name: Install package
  shell: apt install neovim
  become: true

# Prompt for password
$ mooncake run --config config.yml --ask-become-pass
$ mooncake run --config config.yml -K  # shorthand

# Password from file
$ mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass

# Environment variable
$ export SUDO_ASKPASS=/usr/bin/ssh-askpass
$ mooncake run --config config.yml
```

---

## Template Syntax

```yaml
# Variables
{{ variable_name }}

# Filters
{{ "/tmp/file" | expanduser }}       # ~/file
{{ "hello" | upper }}                # HELLO
{{ "/tmp/file.tar.gz" | basename }}  # file.tar.gz

# Conditionals
{% if os == "darwin" %}
macOS specific
{% elif os == "linux" %}
Linux specific
{% else %}
Other OS
{% endif %}

# Loops
{% for item in packages %}
- {{ item }}
{% endfor %}
```

---

## Common Patterns

### Multi-OS Configuration
```yaml
- name: Install on macOS
  shell: brew install neovim
  when: os == "darwin"

- name: Install on Linux (apt)
  shell: apt install -y neovim
  become: true
  when: os == "linux" && package_manager == "apt"

- name: Install on Linux (dnf)
  shell: dnf install -y neovim
  become: true
  when: os == "linux" && package_manager == "dnf"
```

### Idempotent File Creation
```yaml
- name: Create config directory
  file:
    path: ~/.config/myapp
    state: directory

- name: Create config file
  file:
    path: ~/.config/myapp/config.yml
    state: file
    content: |
      setting: value
    creates: ~/.config/myapp/config.yml  # Only if doesn't exist
```

### Backup Before Modify
```yaml
- name: Update config
  copy:
    src: ./new-config.yml
    dest: ~/.config/app/config.yml
    backup: true  # Creates timestamped backup
```

### Download, Extract, Install
```yaml
- name: Download tarball
  download:
    url: https://example.com/app.tar.gz
    dest: /tmp/app.tar.gz
    checksum: sha256:abc123...

- name: Extract
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/app
    creates: /opt/app/bin/app

- name: Create symlink
  file:
    path: /usr/local/bin/app
    state: link
    target: /opt/app/bin/app
  become: true
```

---

## Debugging

```bash
# Dry-run (preview changes)
mooncake run --config config.yml --dry-run

# Debug logging
mooncake run --config config.yml --log-level debug

# Show facts
mooncake facts

# Validate without running
mooncake plan --config config.yml

# Check specific step
mooncake run --config config.yml --tags mystep --dry-run
```

---

## Exit Codes

- `0` - Success
- `1` - General error
- `2` - Configuration error
- `3` - Validation error
- `4` - Execution error

---

## See Also

- [Full Documentation](https://mooncake.alehatsman.com)
- [Actions Reference](config/actions.md)
- [API Reference](../api/actions.md)
- [Examples](../examples/index.md)
- [Troubleshooting](troubleshooting.md)
- [FAQ](faq.md)
