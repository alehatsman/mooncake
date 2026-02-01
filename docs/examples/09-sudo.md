# 09 - Sudo / Privilege Escalation

Learn how to execute commands and operations with elevated privileges.

## What You'll Learn

- Using `become: true` for sudo operations
- Providing sudo password via CLI
- System-level operations
- OS-specific privileged operations

## Quick Start

```bash
cd examples/09-sudo

# Requires sudo password
mooncake run --config config.yml --sudo-pass <your-password>

# Preview what would run with sudo
mooncake run --config config.yml --sudo-pass <password> --dry-run
```

⚠️ **Warning:** This example contains commands that require root privileges. Review the config before running!

## What It Does

1. Runs regular command (no sudo)
2. Runs privileged command with sudo
3. Updates package list (Linux)
4. Installs system packages
5. Creates system directories and files

## Key Concepts

### Basic Sudo

Add `become: true` to run with sudo:
```yaml
- name: System operation
  shell: apt update
  become: true
```

### Providing Password

Three ways to provide sudo password:

**1. Command line (recommended):**
```bash
mooncake run --config config.yml --sudo-pass mypassword
```

**2. Environment variable:**
```bash
export MOONCAKE_SUDO_PASS=mypassword
mooncake run --config config.yml
```

**3. Interactive prompt:**
Some systems may prompt automatically (if configured)

### Which Operations Need Sudo?

**Typically require sudo:**
- Package management (`apt`, `yum`, `dnf`)
- System file operations (`/etc`, `/opt`, `/usr/local`)
- Service management (`systemctl`)
- User/group management
- Mounting filesystems
- Network configuration

**Don't require sudo:**
- User-space operations
- Home directory files
- `/tmp` directory
- Homebrew on macOS (usually)

### File Operations with Sudo

Create system directories:
```yaml
- name: Create system directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"
  become: true
```

Create system files:
```yaml
- name: Create system config
  file:
    path: /etc/myapp/config.yml
    state: file
    content: "config: value"
  become: true
```

### OS-Specific Sudo

```yaml
# Linux package management
- name: Install package (Linux)
  shell: apt install -y curl
  become: true
  when: os == "linux" and package_manager == "apt"

# macOS typically doesn't need sudo for homebrew
- name: Install package (macOS)
  shell: brew install curl
  when: os == "darwin"
```

## Security Considerations

1. **Review before running** - Check what commands will execute with sudo
2. **Use dry-run** - Preview with `--dry-run` first
3. **Minimize sudo usage** - Only use on steps that require it
4. **Specific commands** - Don't use `become: true` on untrusted commands
5. **Password handling** - Be careful with password in shell history

## Common Use Cases

### Package Installation

```yaml
- name: Install system packages
  shell: |
    apt update
    apt install -y nginx postgresql
  become: true
  when: os == "linux"
```

### System Service Setup

```yaml
- name: Create systemd service
  template:
    src: ./myapp.service.j2
    dest: /etc/systemd/system/myapp.service
    mode: "0644"
  become: true

- name: Enable service
  shell: systemctl enable myapp
  become: true
```

### System Directory Setup

```yaml
- name: Create application directories
  file:
    path: "{{ item }}"
    state: directory
    mode: "0755"
  become: true
  with_items:
    - /opt/myapp
    - /etc/myapp
    - /var/log/myapp
```

## Testing

```bash
# Preview what will run with sudo
mooncake run --config config.yml --sudo-pass test --dry-run

# Run with sudo
mooncake run --config config.yml --sudo-pass <password>

# Check created system files
ls -la /opt/myapp/
```

## Troubleshooting

**"sudo: no tty present"**
- Make sure to provide `--sudo-pass` flag

**Permission denied without sudo**
- Add `become: true` to the step

**Command not found**
- Check if command exists: `which <command>`
- Some commands need full paths with sudo

## Next Steps

Continue to [10-multi-file-configs](10-multi-file-configs.md) to learn about organizing large configurations.
