# Advanced File Operations

This example demonstrates Mooncake's expanded file management capabilities:

## File States

### `state: file` - Create or update files
```yaml
- file:
    path: /tmp/config.txt
    state: file
    content: "key: value"
    mode: "0644"
```

### `state: directory` - Create directories
```yaml
- file:
    path: /tmp/app
    state: directory
    mode: "0755"
```

### `state: absent` - Remove files or directories
```yaml
- file:
    path: /tmp/old-file
    state: absent
```

Remove non-empty directory:
```yaml
- file:
    path: /tmp/old-dir
    state: absent
    force: true
```

### `state: touch` - Create empty file or update timestamp
```yaml
- file:
    path: /tmp/.marker
    state: touch
```

### `state: link` - Create symbolic links
```yaml
- file:
    path: /usr/local/bin/app
    src: /opt/app/bin/app
    state: link
```

### `state: hardlink` - Create hard links
```yaml
- file:
    path: /backup/data.txt
    src: /data/data.txt
    state: hardlink
```

### `state: perms` - Change permissions without creating
```yaml
- file:
    path: /opt/app
    state: perms
    mode: "0755"
    owner: app
    group: app
    recurse: true
```

## Copy Action

Copy files with checksum verification:

```yaml
- copy:
    src: ./app-v1.2.3
    dest: /usr/local/bin/app
    mode: "0755"
    checksum: "sha256:abc123..."
    backup: true
```

## Ownership Management

Set file owner and group:

```yaml
- file:
    path: /opt/app/config.yml
    state: file
    owner: app
    group: app
    mode: "0600"
  become: true
```

## Running the Example

```bash
# Dry-run to see what would happen
mooncake run config.yml --dry-run

# Execute the configuration
mooncake run config.yml

# View the created structure
tree /tmp/mooncake-demo
```

## Features Demonstrated

- ✅ Creating directory structures with loops
- ✅ Creating files with inline content
- ✅ Touch files (timestamp updates)
- ✅ Symbolic and hard links
- ✅ Permission-only changes
- ✅ File copying with backup
- ✅ Conditional file removal
- ✅ Force removal of non-empty directories
- ✅ Ownership management with become
