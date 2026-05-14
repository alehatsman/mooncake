# restic - Fast, Secure Backup Program

restic is a fast and secure backup program supporting multiple storage backends with encryption and deduplication.

## Quick Start

```yaml
- preset: restic
```

## Features

- **Fast and secure**: Strong encryption with minimal performance overhead
- **Deduplication**: Content-defined chunking saves storage space
- **Multiple backends**: Local, SFTP, S3, Azure, GCS, Backblaze B2, and more
- **Incremental backups**: Only changed data is backed up
- **Snapshots**: Point-in-time recovery with easy browsing
- **Verification**: Built-in integrity checking

## Basic Usage

```bash
# Initialize repository
restic init --repo /backup/repo

# Backup directory
restic backup ~/documents --repo /backup/repo

# List snapshots
restic snapshots --repo /backup/repo

# Restore latest snapshot
restic restore latest --repo /backup/repo --target /restore/path

# Check repository integrity
restic check --repo /backup/repo

# Prune old snapshots
restic forget --keep-last 30 --prune --repo /backup/repo
```

## Advanced Configuration

```yaml
# Simple installation
- preset: restic

# Remove installation
- preset: restic
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

- **Repository**: Can be local directory or remote (S3, SFTP, etc.)
- **Password**: Set via `RESTIC_PASSWORD` environment variable
- **Repository location**: Set via `RESTIC_REPOSITORY` environment variable

## Real-World Examples

### Local Backup

```bash
# Set environment variables
export RESTIC_REPOSITORY=/backup/repo
export RESTIC_PASSWORD=mySecurePassword

# Initialize repository
restic init

# Backup home directory
restic backup ~ \
  --exclude="*.tmp" \
  --exclude=".cache" \
  --exclude="node_modules"

# List snapshots
restic snapshots

# Restore specific file
restic restore latest --target /restore --include home/user/important.txt
```

### S3 Backup

```bash
# Set AWS credentials
export AWS_ACCESS_KEY_ID=your_key_id
export AWS_SECRET_ACCESS_KEY=your_secret_key

# Initialize S3 repository
restic init --repo s3:s3.amazonaws.com/my-backup-bucket

# Backup with tags
restic backup /data \
  --repo s3:s3.amazonaws.com/my-backup-bucket \
  --tag production \
  --tag daily
```

### Automated Backup Script

```yaml
# Daily backup job
- name: Set restic environment
  shell: |
    export RESTIC_REPOSITORY={{ backup_repo }}
    export RESTIC_PASSWORD={{ vault_backup_password }}
  register: restic_env

- name: Run backup
  shell: |
    restic backup {{ backup_paths }} \
      --exclude-file={{ exclude_file }} \
      --tag daily
  environment:
    RESTIC_REPOSITORY: "{{ backup_repo }}"
    RESTIC_PASSWORD: "{{ vault_backup_password }}"

- name: Clean old snapshots
  shell: |
    restic forget \
      --keep-daily 7 \
      --keep-weekly 4 \
      --keep-monthly 12 \
      --prune
  environment:
    RESTIC_REPOSITORY: "{{ backup_repo }}"
    RESTIC_PASSWORD: "{{ vault_backup_password }}"
```

### Restore Operations

```bash
# List all snapshots
restic snapshots

# Find specific file in snapshots
restic find --repo /backup/repo "important.txt"

# Restore entire snapshot
restic restore abc123de --target /restore/path

# Restore specific path
restic restore latest \
  --target /restore \
  --include /home/user/documents

# Mount repository as filesystem
mkdir /mnt/restic
restic mount /mnt/restic
# Browse snapshots like regular files
ls /mnt/restic/snapshots/
```

### Maintenance

```bash
# Check repository integrity
restic check

# Check with data verification
restic check --read-data

# Prune repository
restic forget --keep-last 10 --prune

# Show repository statistics
restic stats

# Optimize repository
restic prune --max-unused 5%
```

## Agent Use

- Automate system and data backups
- Create disaster recovery workflows
- Implement retention policies for snapshots
- Verify backup integrity in monitoring systems
- Restore files in incident response scenarios

## Troubleshooting

### Repository locked

```bash
# Check locks
restic list locks

# Remove stale lock (only if no backup running!)
restic unlock

# Force unlock
restic unlock --remove-all
```

### Slow backups

```bash
# Increase parallelism
restic backup /data --pack-size 16

# Use faster compression
restic backup /data --compression auto

# Show progress statistics
restic backup /data --verbose=2
```

### Repository errors

```bash
# Check repository
restic check

# Repair repository
restic rebuild-index

# Verify all data
restic check --read-data --read-data-subset=10%
```

### Out of disk space

```bash
# Check repository size
restic stats

# Clean up old snapshots
restic forget --keep-last 30 --prune

# Compact repository
restic prune
```

## Uninstall

```yaml
- preset: restic
  with:
    state: absent
```

**Note**: This only removes the restic binary. Backup repositories are not deleted.

## Resources

- Official docs: https://restic.readthedocs.io/
- GitHub: https://github.com/restic/restic
- Forum: https://forum.restic.net/
- Search: "restic backup tutorial", "restic s3 backup", "restic restore guide"
