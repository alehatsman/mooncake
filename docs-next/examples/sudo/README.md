# 09 - Sudo / Privilege Escalation

Learn how to execute commands and operations with elevated privileges.

## What You'll Learn

- Using `become: true` for sudo operations
- Providing sudo password via CLI
- System-level operations
- OS-specific privileged operations

## Quick Start

```bash
# Interactive prompt (recommended)
mooncake run --config config.yml --ask-become-pass

# Or using short flag
mooncake run --config config.yml -K

# Preview what would run with sudo
mooncake run --config config.yml -K --dry-run
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

Four ways to provide sudo password (mutually exclusive):

**1. Interactive prompt (recommended):**
```bash
mooncake run --config config.yml --ask-become-pass
# or
mooncake run --config config.yml -K
```
Password is hidden while typing. Most secure option.

**2. File-based (secure for automation):**
```bash
echo "mypassword" > ~/.mooncake/sudo_pass
chmod 0600 ~/.mooncake/sudo_pass
mooncake run --config config.yml --sudo-pass-file ~/.mooncake/sudo_pass
```
⚠️ File must have 0600 permissions and be owned by current user.

**3. SUDO_ASKPASS (password manager integration):**
```bash
export SUDO_ASKPASS=/usr/bin/ssh-askpass
mooncake run --config config.yml
```
Uses external helper program for password input.

**4. Command line (insecure, not recommended):**
```bash
mooncake run --config config.yml --sudo-pass mypassword --insecure-sudo-pass
```
⚠️ **WARNING:** Password visible in shell history and process list. Requires explicit `--insecure-sudo-pass` flag.

**Security Features:**
- Passwords are automatically redacted from all log output
- Only one password method can be used at a time
- File permissions are strictly validated

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
5. **Password input** - Use interactive prompt or file-based methods, avoid CLI flag
6. **Password redaction** - Passwords are automatically redacted from logs (debug, stdout, stderr)
7. **File permissions** - If using `--sudo-pass-file`, ensure 0600 permissions
8. **Platform support** - Only works on Linux and macOS (explicitly fails on Windows)

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
mooncake run --config config.yml -K --dry-run

# Run with sudo
mooncake run --config config.yml -K

# Check created system files
sudo ls -la /opt/myapp/

# Verify password redaction in debug logs
mooncake run --config config.yml -K --log-level debug | grep -i password
# Should show [REDACTED] instead of actual password
```

## Troubleshooting

**"step requires sudo but no password provided"**
- Provide password using `--ask-become-pass`, `--sudo-pass-file`, or `SUDO_ASKPASS`

**"--sudo-pass requires --insecure-sudo-pass flag"**
- CLI password flag requires explicit security acknowledgment
- Use `--ask-become-pass` instead (more secure)

**"password file must have 0600 permissions"**
- Fix permissions: `chmod 0600 /path/to/password/file`
- Verify ownership: `ls -l /path/to/password/file`

**"only one password method can be specified"**
- Remove conflicting password flags
- Use only one of: `--ask-become-pass`, `--sudo-pass-file`, or `--sudo-pass`

**"become is not supported on windows"**
- Privilege escalation only works on Linux and macOS
- Use platform-specific conditionals with `when`

**Permission denied without sudo**
- Add `become: true` to the step

**Command not found**
- Check if command exists: `which <command>`
- Some commands need full paths with sudo

## Next Steps

→ Continue to [10-multi-file-configs](../10-multi-file-configs/) to learn about organizing large configurations.
