# rsync - Fast File Synchronization

rsync is a fast and versatile file copying tool for local and remote synchronization.

## Quick Start

```yaml
- preset: rsync
```

## Features

- **Fast delta-transfer**: Only transfers differences between files
- **Bandwidth efficient**: Compression and batching optimizations
- **Preserves metadata**: Permissions, timestamps, ownership, and links
- **Versatile**: Local copying, remote sync via SSH, daemon mode
- **Robust**: Handles interruptions, partial transfers
- **Flexible filtering**: Include/exclude patterns

## Basic Usage

```bash
# Local copy
rsync -avz source/ destination/

# Remote copy (via SSH)
rsync -avz source/ user@remote:/path/

# Download from remote
rsync -avz user@remote:/path/ local/

# Sync with delete (mirror)
rsync -avz --delete source/ destination/

# Dry run (test without changes)
rsync -avzn source/ destination/

# Show progress
rsync -avz --progress source/ destination/
```

## Advanced Configuration

```yaml
# Simple installation
- preset: rsync

# Remove installation
- preset: rsync
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper) - usually pre-installed
- ✅ macOS (pre-installed, Homebrew for latest version)
- ❌ Windows (not yet supported by this preset, use WSL)

## Configuration

- **No global config**: Options passed via command-line
- **Per-directory config**: `.rsync-filter` files for include/exclude rules
- **SSH config**: Uses `~/.ssh/config` for remote connections

## Real-World Examples

### Website Deployment

```bash
# Deploy website to server
rsync -avz --delete \
  --exclude=".git" \
  --exclude="node_modules" \
  --exclude=".env" \
  ./public/ user@webserver:/var/www/html/

# Deploy with dry-run first
rsync -avzn --delete ./public/ user@webserver:/var/www/html/
# Review changes, then run without -n
```

### Backup System

```yaml
# Daily backup to NAS
- name: Backup data to NAS
  shell: |
    rsync -avz --delete \
      --exclude="*.tmp" \
      --exclude=".cache/**" \
      --link-dest=/nas/backups/latest \
      /home/ \
      /nas/backups/$(date +%Y-%m-%d)/
  register: backup_result

- name: Update latest link
  shell: |
    ln -snf /nas/backups/$(date +%Y-%m-%d) /nas/backups/latest
  when: backup_result.rc == 0
```

### Incremental Backups with Hard Links

```bash
# Create dated backup with hard links to save space
TODAY=$(date +%Y-%m-%d)
YESTERDAY=$(date -d "yesterday" +%Y-%m-%d)

rsync -avz \
  --link-dest=/backup/$YESTERDAY \
  /source/ \
  /backup/$TODAY/

# This creates a full backup, but unchanged files
# are hard-linked to previous backup (saves space)
```

### Selective Sync

```bash
# Include only specific file types
rsync -avz \
  --include="*.jpg" \
  --include="*.png" \
  --include="*/" \
  --exclude="*" \
  source/ destination/

# Exclude patterns
rsync -avz \
  --exclude="*.log" \
  --exclude="tmp/**" \
  --exclude=".git/**" \
  source/ destination/
```

### Remote to Remote Copy

```bash
# Copy between two remote servers
rsync -avz \
  -e "ssh -A" \
  server1:/path/source/ \
  server2:/path/destination/
```

### Bandwidth Limiting

```bash
# Limit to 1MB/s
rsync -avz --bwlimit=1024 source/ destination/

# Run during specific hours (via cron)
# 0 2 * * * rsync -avz --bwlimit=5120 /source/ /backup/
```

## Common Options Explained

```bash
# -a (archive): Preserve permissions, times, links, recursion
# -v (verbose): Show files being transferred
# -z (compress): Compress during transfer
# -n (dry-run): Show what would be transferred
# -P (progress + partial): Show progress, keep partial transfers
# --delete: Remove files in dest not in source
# --exclude: Exclude patterns
# --include: Include patterns
# --link-dest: Hard link unchanged files to previous backup
# --bwlimit: Bandwidth limit in KB/s
```

## Agent Use

- Deploy applications and static sites to servers
- Create incremental backup systems
- Synchronize configuration files across servers
- Mirror directories between storage locations
- Distribute files to multiple targets efficiently

## Troubleshooting

### Permission denied

```bash
# Use sudo on remote side
rsync -avz --rsync-path="sudo rsync" source/ user@remote:/root/dest/

# Check SSH keys
ssh user@remote "ls -la /destination"
```

### Connection issues

```bash
# Test SSH connection first
ssh user@remote

# Use specific SSH key
rsync -avz -e "ssh -i ~/.ssh/id_rsa" source/ user@remote:/dest/

# Increase verbosity
rsync -avvvz source/ user@remote:/dest/
```

### Slow transfers

```bash
# Disable compression for local network
rsync -av --no-compress source/ destination/

# Use different compression
rsync -avz --compress-level=3 source/ destination/

# Increase ssh cipher speed
rsync -avz -e "ssh -c aes128-ctr" source/ user@remote:/dest/
```

### Disk space full

```bash
# Check space before sync
df -h /destination

# Remove old backups first
find /backup/ -type d -mtime +30 -exec rm -rf {} +

# Use --max-size to skip large files
rsync -avz --max-size=100M source/ destination/
```

## Uninstall

```yaml
- preset: rsync
  with:
    state: absent
```

**Note**: rsync is often a system package. Removing it may affect other tools.

## Resources

- Official docs: https://rsync.samba.org/
- Man page: `man rsync`
- GitHub: https://github.com/WayneD/rsync
- Search: "rsync tutorial", "rsync backup script", "rsync examples"
