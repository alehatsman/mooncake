# Unarchive - Extract Archive Files

Learn how to extract archive files with automatic format detection and security protections.

## What You'll Learn

- Extracting tar, tar.gz, tgz, and zip archives
- Using `strip_components` to remove leading directories
- Idempotency with `creates` parameter
- Handling different archive formats
- Security protections against path traversal

## Quick Start

```bash
cd examples/12-unarchive
mooncake run --config config.yml
```

## What It Does

1. Downloads sample archives (or uses provided ones)
2. Extracts various archive formats
3. Demonstrates path stripping
4. Shows idempotent extraction
5. Extracts to system directories with sudo

## Key Concepts

### Basic Extraction

Extract an archive to a destination directory:

```yaml
- name: Extract Node.js
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    mode: "0755"
```

The destination directory is created if it doesn't exist.

### Supported Formats

Mooncake automatically detects the archive format from the file extension:

| Format | Extensions | Compression |
|--------|-----------|-------------|
| tar | `.tar` | None |
| tar.gz | `.tar.gz`, `.tgz` | Gzip |
| zip | `.zip` | ZIP compression |

Detection is case-insensitive (`.TAR`, `.TGZ`, `.ZIP` all work).

### Strip Components

Remove leading directory levels from extracted paths:

```yaml
# Archive contains: node-v20/bin/node, node-v20/lib/...
- name: Extract without top-level directory
  unarchive:
    src: /tmp/node-v20.tar.gz
    dest: /opt/node
    strip_components: 1
    # Result: /opt/node/bin/node, /opt/node/lib/...
```

**How it works:**

```
Archive structure:
  project-1.0/src/main.go
  project-1.0/src/utils.go
  project-1.0/README.md

strip_components: 0 (default) → dest/project-1.0/src/main.go
strip_components: 1           → dest/src/main.go
strip_components: 2           → dest/main.go
```

Files with fewer path components than specified are skipped.

### Idempotency with Creates

Skip extraction if a marker file already exists:

```yaml
- name: Extract application
  unarchive:
    src: /tmp/myapp.tar.gz
    dest: /opt/myapp
    creates: /opt/myapp/bin/myapp
    mode: "0755"
```

On subsequent runs, if `/opt/myapp/bin/myapp` exists, extraction is skipped. This prevents unnecessary re-extraction and maintains idempotency.

### Custom Directory Permissions

Set permissions for created directories:

```yaml
- name: Extract with custom permissions
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/myapp
    mode: "0700"  # rwx------
```

File permissions are preserved from the archive. The `mode` parameter only affects directories created during extraction.

### Extract with Privilege Escalation

Extract to system directories using sudo:

```yaml
- name: Extract to system directory
  unarchive:
    src: /tmp/app.tar.gz
    dest: /opt/myapp
    strip_components: 1
    mode: "0755"
  become: true
```

### Using Variables

Use template variables in all paths:

```yaml
- vars:
    app_version: "1.2.3"
    install_dir: "/opt/myapp"

- name: Extract versioned release
  unarchive:
    src: "/tmp/app-{{app_version}}.tar.gz"
    dest: "{{install_dir}}"
    creates: "{{install_dir}}/bin/app"
    strip_components: 1
```

### Extract Multiple Archives

Use loops to extract multiple archives:

```yaml
- vars:
    packages:
      - name: app
        file: app-v1.2.3.tar.gz
        strip: 1
      - name: data
        file: data.zip
        strip: 0

- name: Extract {{item.name}}
  unarchive:
    src: /tmp/{{item.file}}
    dest: /opt/{{item.name}}
    strip_components: "{{item.strip}}"
    creates: /opt/{{item.name}}/.installed
  with_items: "{{packages}}"
```

## Security Features

Mooncake automatically protects against path traversal attacks:

### Blocked Patterns

These malicious patterns are automatically blocked:

```yaml
# ❌ Path traversal with ../
Archive entry: ../../../etc/passwd

# ❌ Absolute paths
Archive entry: /etc/passwd

# ❌ Traversal in nested paths
Archive entry: legit/../../sensitive

# ❌ Symlinks escaping destination
Symlink target: ../../../etc/shadow
```

All extracted paths are validated to ensure they stay within the destination directory.

### Security Guarantees

1. **Path Traversal Protection**: All entries with `../` are rejected
2. **Absolute Path Blocking**: Absolute paths are not allowed
3. **Symlink Validation**: Symlink targets are checked for escapes
4. **Safe Joining**: Uses `pathutil.SafeJoin()` for all paths

These protections are always active and cannot be disabled.

## Complete Example

Here's a complete example showing common patterns:

```yaml
version: "1.0"

vars:
  node_version: "20.11.0"
  install_dir: "/opt/node"
  backup_dir: "/var/backups"

steps:
  # Download archive if needed
  - name: Download Node.js
    shell: "curl -fsSL https://nodejs.org/dist/v{{node_version}}/node-v{{node_version}}-linux-x64.tar.gz -o /tmp/node.tar.gz"
    creates: "/tmp/node.tar.gz"

  # Extract with strip_components
  - name: Extract Node.js
    unarchive:
      src: "/tmp/node.tar.gz"
      dest: "{{install_dir}}"
      strip_components: 1
      creates: "{{install_dir}}/bin/node"
      mode: "0755"
    register: node_extracted

  # Verify installation
  - name: Check Node.js version
    shell: "{{install_dir}}/bin/node --version"
    when: node_extracted.changed

  # Extract ZIP archive
  - name: Extract application data
    unarchive:
      src: "/tmp/app-data.zip"
      dest: "{{install_dir}}/data"
      mode: "0755"

  # Extract backup with sudo
  - name: Restore system backup
    unarchive:
      src: "{{backup_dir}}/system-backup.tar.gz"
      dest: "/etc/myapp"
      creates: "/etc/myapp/.restored"
      mode: "0755"
    become: true
```

## Common Use Cases

### Software Installation

Extract and install precompiled binaries:

```yaml
- name: Install Go
  unarchive:
    src: /tmp/go1.21.linux-amd64.tar.gz
    dest: /usr/local
    creates: /usr/local/go/bin/go
  become: true
```

### Application Deployment

Deploy application releases:

```yaml
- name: Deploy application
  unarchive:
    src: /tmp/myapp-{{version}}.tar.gz
    dest: /opt/myapp
    strip_components: 1
    mode: "0755"
  become: true

- name: Create version marker
  file:
    path: /opt/myapp/.version
    content: "{{version}}"
    state: file
  become: true
```

### Backup Restoration

Restore from tar backups:

```yaml
- name: Restore user data
  unarchive:
    src: /backups/user-data-{{date}}.tar.gz
    dest: /home/{{username}}
    creates: /home/{{username}}/.restored
```

### Multi-platform Distribution

Extract platform-specific archives:

```yaml
- name: Extract platform binary
  unarchive:
    src: "/tmp/app-{{os}}-{{arch}}.tar.gz"
    dest: /opt/app
    strip_components: 1
    creates: /opt/app/bin/app
```

## Real-World Example

Complete Node.js installation workflow:

```yaml
version: "1.0"

vars:
  node_version: "20.11.0"
  node_base_url: "https://nodejs.org/dist"
  install_dir: "/opt/node"

steps:
  - name: Detect platform
    shell: "uname -s | tr '[:upper:]' '[:lower:]'"
    register: platform_result

  - name: Detect architecture
    shell: "uname -m"
    register: arch_result

  - name: Set Node.js archive name
    vars:
      platform_map:
        linux: "linux"
        darwin: "darwin"
      arch_map:
        x86_64: "x64"
        aarch64: "arm64"
        arm64: "arm64"
      platform: "{{platform_result.stdout}}"
      arch: "{{arch_result.stdout}}"
      node_platform: "{{platform_map[platform]}}"
      node_arch: "{{arch_map[arch]}}"
      archive_name: "node-v{{node_version}}-{{node_platform}}-{{node_arch}}.tar.gz"

  - name: Download Node.js
    shell: "curl -fsSL {{node_base_url}}/v{{node_version}}/{{archive_name}} -o /tmp/node.tar.gz"
    creates: "/tmp/node.tar.gz"
    timeout: 10m
    retries: 3
    retry_delay: 30s

  - name: Extract Node.js
    unarchive:
      src: "/tmp/node.tar.gz"
      dest: "{{install_dir}}"
      strip_components: 1
      creates: "{{install_dir}}/bin/node"
      mode: "0755"
    become: true
    register: node_install

  - name: Create symlinks
    shell: |
      ln -sf {{install_dir}}/bin/node /usr/local/bin/node
      ln -sf {{install_dir}}/bin/npm /usr/local/bin/npm
      ln -sf {{install_dir}}/bin/npx /usr/local/bin/npx
    when: node_install.changed
    become: true

  - name: Verify installation
    shell: "node --version && npm --version"
    register: versions

  - name: Show installed versions
    shell: "echo 'Node.js installed: {{versions.stdout}}'"
```

## See Also

- [File Operations](03-files-and-directories.md) - File and directory management
- [Loops](06-loops.md) - Iterating over multiple items
- [Sudo](09-sudo.md) - Privilege escalation
- [Actions Reference](../guide/config/actions.md#unarchive) - Complete action documentation
- [Configuration Reference](../guide/config/reference.md#unarchive) - Property reference
