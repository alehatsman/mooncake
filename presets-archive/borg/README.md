# BorgBackup - Deduplicating Backup Program

Efficient, encrypted backup tool with compression and deduplication for secure data archival.

## Quick Start
```yaml
- preset: borg
```

## Features
- **Deduplication**: Store only unique data chunks, saving space
- **Compression**: Multiple algorithms (lz4, zstd, zlib)
- **Encryption**: AES-256 authenticated encryption
- **Mountable archives**: Browse backups as filesystem
- **Efficient**: Fast incremental backups
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# Initialize repository
borg init --encryption=repokey /path/to/repo

# Create backup
borg create /path/to/repo::backup-2024-01-15 /home/user

# List archives
borg list /path/to/repo

# Extract archive
borg extract /path/to/repo::backup-2024-01-15

# Mount archive
borg mount /path/to/repo::backup-2024-01-15 /mnt/backup
umount /mnt/backup

# Delete archive
borg delete /path/to/repo::backup-2024-01-15

# Prune old backups
borg prune --keep-daily=7 --keep-weekly=4 --keep-monthly=6 /path/to/repo
```

## Advanced Configuration

```yaml
# Install BorgBackup
- preset: borg
  become: true

# Initialize repository
- name: Create Borg repository
  shell: |
    export BORG_PASSPHRASE="{{ vault_borg_password }}"
    borg init --encryption=repokey {{ backup_repo }}
  no_log: true
  creates: "{{ backup_repo }}/config"

# Create backup
- name: Backup application data
  shell: |
    export BORG_PASSPHRASE="{{ vault_borg_password }}"
    borg create --stats --compression lz4 \
      {{ backup_repo }}::$(hostname)-$(date +%Y%m%d-%H%M%S) \
      /opt/app/data /etc/app
  no_log: true
  register: backup

# Prune old backups
- name: Prune old backups
  shell: |
    export BORG_PASSPHRASE="{{ vault_borg_password }}"
    borg prune --keep-daily=7 --keep-weekly=4 \
      --keep-monthly=12 {{ backup_repo }}
  no_log: true
```

## Repository Initialization

```bash
# No encryption (not recommended)
borg init --encryption=none /path/to/repo

# Repository key (stored in repo)
borg init --encryption=repokey /path/to/repo

# Keyfile (stored locally)
borg init --encryption=keyfile /path/to/repo

# Repository key with Blake2 (faster)
borg init --encryption=repokey-blake2 /path/to/repo

# Set passphrase via environment
export BORG_PASSPHRASE='your-strong-passphrase'
borg init --encryption=repokey /path/to/repo
```

## Creating Backups

### Basic Backup
```bash
# Backup with timestamp
borg create /path/to/repo::$(hostname)-$(date +%Y%m%d) \
  /home/user /etc

# With compression
borg create --compression lz4 \
  /path/to/repo::backup-$(date +%Y%m%d) \
  /home/user

# With statistics
borg create --stats --progress \
  /path/to/repo::backup-$(date +%Y%m%d) \
  /home/user
```

### Exclude Patterns
```bash
# Exclude patterns
borg create /path/to/repo::backup-$(date +%Y%m%d) \
  /home/user \
  --exclude '*.pyc' \
  --exclude '*/.cache/*' \
  --exclude '*/node_modules/*'

# Exclude from file
borg create /path/to/repo::backup-$(date +%Y%m%d) \
  /home/user \
  --exclude-from /etc/borg/exclude.txt
```

### Compression Options
```bash
# LZ4 (fast)
borg create --compression lz4 /path/to/repo::backup /data

# ZSTD (balanced)
borg create --compression zstd,3 /path/to/repo::backup /data

# ZLIB (high compression)
borg create --compression zlib,9 /path/to/repo::backup /data

# Auto compression (borg decides)
borg create --compression auto,lzma /path/to/repo::backup /data
```

## Restoring Backups

```bash
# List files in archive
borg list /path/to/repo::backup-20240115

# Extract entire archive
borg extract /path/to/repo::backup-20240115

# Extract specific files
borg extract /path/to/repo::backup-20240115 path/to/file

# Extract with original paths
borg extract --strip-components 0 /path/to/repo::backup-20240115

# Extract to specific directory
cd /restore/location
borg extract /path/to/repo::backup-20240115
```

## Mounting Archives

```bash
# Mount latest archive
borg mount /path/to/repo::backup-latest /mnt/backup

# Mount entire repository (all archives)
borg mount /path/to/repo /mnt/backup

# Browse files
ls /mnt/backup/backup-20240115/home/user

# Copy files
cp -a /mnt/backup/backup-20240115/home/user/important.txt /restore/

# Unmount
umount /mnt/backup
```

## Pruning and Maintenance

### Prune Old Backups
```bash
# Keep rule-based archives
borg prune --keep-daily=7 --keep-weekly=4 --keep-monthly=12 \
  --keep-yearly=5 /path/to/repo

# Dry run
borg prune --dry-run --keep-daily=7 --keep-weekly=4 /path/to/repo

# With prefix filter
borg prune --prefix hostname- --keep-daily=7 /path/to/repo
```

### Compact Repository
```bash
# Free space by compacting segments
borg compact /path/to/repo
```

### Check Repository
```bash
# Quick check
borg check /path/to/repo

# Full check (slow)
borg check --verify-data /path/to/repo

# Check specific archive
borg check /path/to/repo::backup-20240115
```

## Real-World Examples

### Automated Daily Backup Script
```bash
#!/bin/bash
# /usr/local/bin/borg-backup.sh

export BORG_REPO="/backups/borg-repo"
export BORG_PASSPHRASE="your-passphrase"

# Backup paths
borg create --stats --compression lz4 \
  $BORG_REPO::$(hostname)-$(date +%Y%m%d-%H%M%S) \
  /home \
  /etc \
  /var/log \
  --exclude '/home/*/.cache' \
  --exclude '*/node_modules'

# Prune old backups
borg prune --keep-daily=7 --keep-weekly=4 --keep-monthly=12 $BORG_REPO

# Check repository health
borg check --last 5 $BORG_REPO

unset BORG_PASSPHRASE
```

### Cron Job for Scheduled Backups
```bash
# /etc/cron.d/borg-backup
0 2 * * * root /usr/local/bin/borg-backup.sh >> /var/log/borg-backup.log 2>&1
```

### Remote Backup to SSH Server
```yaml
# Backup to remote server
- preset: borg

- name: Create SSH key for Borg
  shell: ssh-keygen -t ed25519 -f /root/.ssh/borg_backup -N ""
  creates: /root/.ssh/borg_backup

- name: Initialize remote repository
  shell: |
    export BORG_PASSPHRASE="{{ borg_passphrase }}"
    borg init --encryption=repokey \
      ssh://backup@backup.example.com/~/borg-repo
  no_log: true
  creates: /root/.borg-initialized

- name: Backup to remote
  shell: |
    export BORG_PASSPHRASE="{{ borg_passphrase }}"
    borg create --stats --compression zstd \
      ssh://backup@backup.example.com/~/borg-repo::$(hostname)-$(date +%Y%m%d) \
      /opt/app
  no_log: true
```

### Docker Volume Backup
```bash
# Backup Docker volumes
borg create /backups/borg-repo::docker-$(date +%Y%m%d) \
  /var/lib/docker/volumes

# Or use Borg in container
docker run --rm \
  -v /backups:/backups \
  -v /data:/data:ro \
  -e BORG_PASSPHRASE=secret \
  borgbackup/borg-docker \
  borg create /backups/repo::backup-$(date +%Y%m%d) /data
```

## Environment Variables

```bash
# Repository location
export BORG_REPO=/path/to/repo

# Passphrase
export BORG_PASSPHRASE='your-passphrase'

# Key file location
export BORG_KEY_FILE=/path/to/keyfile

# Cache directory
export BORG_CACHE_DIR=/var/cache/borg

# SSH command
export BORG_RSH='ssh -i /path/to/key'

# Logging
export BORG_LOGGING_CONF=/etc/borg/logging.conf
```

## Troubleshooting

### Repository Locked
```bash
# Error: "Failed to create/acquire the lock"
# Solution: Break lock (ensure no other Borg process running)
borg break-lock /path/to/repo
```

### Corrupted Repository
```bash
# Check repository
borg check --repair /path/to/repo

# If badly corrupted, extract what you can
borg list /path/to/repo
borg extract /path/to/repo::working-archive
```

### Out of Space
```bash
# Compact repository to free space
borg compact /path/to/repo

# Prune aggressively
borg prune --keep-daily=3 --keep-weekly=2 /path/to/repo

# Delete specific archive
borg delete /path/to/repo::old-archive
```

### Slow Backups
```bash
# Use faster compression
borg create --compression lz4 /path/to/repo::backup /data

# Exclude large/unimportant files
borg create /path/to/repo::backup /data \
  --exclude '*.iso' --exclude '*/Cache/*'

# Increase upload buffer for remote repos
export BORG_RSH='ssh -o ServerAliveInterval=60 -o TCPKeepAlive=yes'
```

## Security Best Practices

```bash
# Strong passphrase
export BORG_PASSPHRASE=$(pwgen -s 64 1)

# Store passphrase securely
echo "$BORG_PASSPHRASE" > /root/.borg-passphrase
chmod 600 /root/.borg-passphrase

# Export encryption key
borg key export /path/to/repo /root/.borg-key-backup
chmod 600 /root/.borg-key-backup

# Test restore regularly
borg extract --dry-run /path/to/repo::latest
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, Homebrew)
- ✅ macOS (Homebrew, MacPorts)
- ✅ BSD (pkg, ports)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automate server backups in infrastructure
- Implement 3-2-1 backup strategy
- Create disaster recovery procedures
- Backup databases before migrations
- Archive application data securely
- Implement compliance-required data retention
- Secure offsite backup automation

## Uninstall
```yaml
- preset: borg
  with:
    state: absent
```

## Resources
- Official docs: https://borgbackup.readthedocs.io/
- GitHub: https://github.com/borgbackup/borg
- Quick start: https://borgbackup.readthedocs.io/en/stable/quickstart.html
- Community: https://github.com/borgbackup/borg/discussions
- Search: "borg backup tutorial", "borg backup automation", "borg vs restic"
