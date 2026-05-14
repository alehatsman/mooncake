# rclone - Cloud Storage Sync

rclone is a command-line program to sync files and directories to and from cloud storage providers.

## Quick Start

```yaml
- preset: rclone
```

## Features

- **70+ providers**: S3, Google Drive, Dropbox, Azure, Backblaze B2, and more
- **Sync and mount**: Bidirectional sync or mount cloud storage as filesystem
- **Encryption**: Built-in encryption for cloud data
- **Bandwidth control**: Limit transfer speeds and schedule transfers
- **Deduplication**: Efficient handling of duplicate files
- **Resume support**: Automatically resume interrupted transfers

## Basic Usage

```bash
# Configure a remote (interactive)
rclone config

# List remotes
rclone listremotes

# Copy local to remote
rclone copy /path/to/local remote:bucket/path

# Sync directories (one-way)
rclone sync /local/path remote:bucket/path

# Bidirectional sync
rclone bisync /local/path remote:bucket/path

# Mount remote as filesystem
rclone mount remote:bucket/path /mnt/point --daemon

# List files in remote
rclone ls remote:bucket/path

# Check differences
rclone check /local/path remote:bucket/path
```

## Advanced Configuration

```yaml
# Simple installation
- preset: rclone

# Remove installation
- preset: rclone
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove (present/absent) |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ❌ Windows (not yet supported by this preset)

## Configuration

- **Config file**: `~/.config/rclone/rclone.conf`
- **Cache directory**: `~/.cache/rclone/`
- **Log file**: Configurable via `--log-file` flag

## Real-World Examples

### Backup to S3

```bash
# Configure S3 remote
rclone config create s3-backup s3 \
  provider AWS \
  access_key_id $AWS_ACCESS_KEY_ID \
  secret_access_key $AWS_SECRET_ACCESS_KEY \
  region us-east-1

# Backup directory to S3
rclone sync /home/user/documents s3-backup:my-bucket/documents \
  --progress \
  --transfers 8 \
  --checkers 16

# Restore from S3
rclone sync s3-backup:my-bucket/documents /home/user/documents-restored
```

### Automated Daily Backups

```yaml
# Create rclone backup job
- name: Configure rclone for backups
  shell: |
    rclone config create backup-remote s3 \
      provider {{ cloud_provider }} \
      access_key_id {{ vault_access_key }} \
      secret_access_key {{ vault_secret_key }}

- name: Run daily backup
  shell: |
    rclone sync {{ backup_source }} backup-remote:{{ bucket_name }} \
      --exclude "*.tmp" \
      --exclude ".cache/**" \
      --log-file /var/log/rclone-backup.log
  register: backup_result

- name: Verify backup
  shell: rclone check {{ backup_source }} backup-remote:{{ bucket_name }}
```

### Mount Cloud Storage

```bash
# Mount Google Drive
rclone config create gdrive drive scope drive
rclone mount gdrive: /mnt/gdrive \
  --vfs-cache-mode writes \
  --daemon

# Access files
ls /mnt/gdrive
cat /mnt/gdrive/document.txt

# Unmount
fusermount -u /mnt/gdrive  # Linux
umount /mnt/gdrive  # macOS
```

### Encrypted Remote

```bash
# Create encrypted remote on top of existing remote
rclone config create encrypted-backup crypt \
  remote s3-backup:my-bucket/encrypted \
  filename_encryption standard \
  directory_name_encryption true \
  password your-password \
  password2 salt-password

# Files are encrypted before upload
rclone copy /sensitive/data encrypted-backup:
```

### Sync Between Two Cloud Providers

```bash
# Copy from Google Drive to Dropbox
rclone copy gdrive:Documents dropbox:Backup/Documents \
  --progress \
  --transfers 4

# Sync (delete files in destination not in source)
rclone sync gdrive:Photos dropbox:Photos --progress
```

## Agent Use

- Automate cloud storage backups and restores
- Sync data between different cloud providers
- Mount cloud storage for application access
- Migrate data between storage services
- Create encrypted offsite backups

## Troubleshooting

### Authentication errors

```bash
# Reconfigure remote
rclone config reconnect remote-name

# Test connection
rclone lsd remote-name:

# Check credentials
rclone config show remote-name
```

### Slow transfers

```bash
# Increase parallelism
rclone copy source dest \
  --transfers 16 \
  --checkers 32 \
  --buffer-size 256M

# Use multiple connections per file
rclone copy source dest --multi-thread-streams 8
```

### Mount not working

```bash
# Install FUSE (required for mounting)
# Ubuntu/Debian
sudo apt-get install fuse3

# macOS
brew install macfuse

# Check if mount is running
ps aux | grep rclone
```

### Out of space errors

```bash
# Check quota
rclone about remote:

# Clean up old files
rclone delete remote:old-backups --min-age 30d

# Use size filters
rclone copy source dest --max-size 100M
```

## Uninstall

```yaml
- preset: rclone
  with:
    state: absent
```

## Resources

- Official docs: https://rclone.org/docs/
- GitHub: https://github.com/rclone/rclone
- Forum: https://forum.rclone.org/
- Search: "rclone tutorial", "rclone backup script", "rclone vs rsync"
