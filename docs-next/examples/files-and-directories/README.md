# 03 - Files and Directories

Learn how to create and manage files and directories with Mooncake.

## What You'll Learn

- Creating directories with `state: directory`
- Creating files with `state: file`
- Setting file permissions with `mode`
- Adding content to files

## Quick Start

```bash
mooncake run --config config.yml
```

## What It Does

1. Creates application directory structure
2. Creates files with specific content
3. Sets appropriate permissions (755 for directories, 644 for files)
4. Creates executable scripts

## Key Concepts

### Creating Directories

```yaml
- name: Create directory
  file:
    path: /tmp/myapp
    state: directory
    mode: "0755"  # rwxr-xr-x
```

### Creating Empty Files

```yaml
- name: Create empty file
  file:
    path: /tmp/file.txt
    state: file
    mode: "0644"  # rw-r--r--
```

### Creating Files with Content

```yaml
- name: Create config file
  file:
    path: /tmp/config.txt
    state: file
    content: |
      Line 1
      Line 2
    mode: "0644"
```

### File Permissions

Use octal notation in quotes:

- `"0644"` - rw-r--r-- (readable by all, writable by owner)
- `"0755"` - rwxr-xr-x (executable by all, writable by owner)
- `"0600"` - rw------- (only owner can read/write)

### Using Variables

```yaml
- vars:
    app_dir: /tmp/myapp

- file:
    path: "{{app_dir}}/config"
    state: directory
```

## Permission Examples

| Mode | Meaning | Use Case |
|------|---------|----------|
| 0755 | rwxr-xr-x | Directories, executable scripts |
| 0644 | rw-r--r-- | Regular files, configs |
| 0600 | rw------- | Private files, secrets |
| 0700 | rwx------ | Private directories |

## Next Steps

→ Continue to [04-conditionals](../04-conditionals/) to learn about conditional execution.
