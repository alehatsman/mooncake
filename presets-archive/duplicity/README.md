# Duplicity - Encrypted Backup Tool

Bandwidth-efficient encrypted backup tool supporting various storage backends with incremental backups.

## Quick Start
```yaml
- preset: duplicity
```

## Features
- **Encryption**: GPG encryption for secure backups
- **Incremental**: Efficient incremental backups save bandwidth and storage
- **Remote storage**: S3, GCS, Azure, SFTP, FTP, Rclone, and more
- **Bandwidth-efficient**: rsync algorithm transfers only changed data
- **Compression**: Built-in gzip/bzip2 compression
- **Verification**: Backup integrity checking and restoration testing

## Basic Usage
```bash
# Full backup to local directory
export PASSPHRASE="your-secure-passphrase"
duplicity /home/user file:///backup/user-backup

# Backup to S3
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
export PASSPHRASE="your-passphrase"
duplicity /home/user s3://s3.amazonaws.com/my-bucket/backup

# Restore from backup
duplicity s3://s3.amazonaws.com/my-bucket/backup /home/user-restored

# List backup files
duplicity list-current-files s3://s3.amazonaws.com/my-bucket/backup

# Check backup status
duplicity collection-status s3://s3.amazonaws.com/my-bucket/backup

# Remove old backups
duplicity remove-older-than 90D s3://s3.amazonaws.com/my-bucket/backup --force
```

## Advanced Configuration
```yaml
# Basic installation
- preset: duplicity

# Uninstall
- preset: duplicity
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Storage Backends

### Amazon S3
```bash
export AWS_ACCESS_KEY_ID="xxx"
export AWS_SECRET_ACCESS_KEY="xxx"
export PASSPHRASE="secure"

duplicity /data s3://s3.amazonaws.com/bucket/backup
```

### Google Cloud Storage
```bash
export GCS_ACCESS_KEY_ID="xxx"
export GCS_SECRET_ACCESS_KEY="xxx"
export PASSPHRASE="secure"

duplicity /data gs://bucket/backup
```

### Azure Blob Storage
```bash
export AZURE_ACCOUNT_NAME="xxx"
export AZURE_ACCOUNT_KEY="xxx"
export PASSPHRASE="secure"

duplicity /data azure://container/backup
```

### SFTP
```bash
export PASSPHRASE="secure"
export FTP_PASSWORD="ssh-password"

duplicity /data sftp://user@server.com//backup
```

### Rclone (any cloud)
```bash
export PASSPHRASE="secure"

duplicity /data rclone://remote:bucket/backup
```

## Configuration
- **Environment variables**: Set credentials and options
- **GPG keys**: Use for encryption/signing
- **Passphrase**: Required for symmetric encryption
- **Cache directory**: `~/.cache/duplicity/` (Linux), `~/Library/Caches/duplicity/` (macOS)

## Real-World Examples

### Automated Daily Backup Script
```bash
#!/bin/bash
# backup-home.sh

export AWS_ACCESS_KEY_ID="xxx"
export AWS_SECRET_ACCESS_KEY="xxx"
export PASSPHRASE="$(cat /root/.backup-passphrase)"

BACKUP_SOURCE="/home"
BACKUP_TARGET="s3://s3.amazonaws.com/backups/home"

# Incremental backup (full every 30 days)
duplicity \
  --full-if-older-than 30D \
  --exclude /home/*/.cache \
  --exclude /home/*/Downloads \
  $BACKUP_SOURCE $BACKUP_TARGET

# Cleanup old backups (keep 90 days)
duplicity remove-older-than 90D $BACKUP_TARGET --force

# Verify backup integrity
duplicity verify $BACKUP_TARGET $BACKUP_SOURCE

# Check status
duplicity collection-status $BACKUP_TARGET
```

### Selective Backup with Excludes
```bash
export PASSPHRASE="secure"

duplicity \
  --exclude /var/log \
  --exclude /var/cache \
  --exclude '**/.git' \
  --exclude '**/*.tmp' \
  /var s3://bucket/var-backup
```

### Restore Specific Files
```bash
# Restore single file
duplicity --file-to-restore home/user/document.txt \
  s3://bucket/backup /tmp/restored-document.txt

# Restore directory
duplicity --file-to-restore home/user/projects \
  s3://bucket/backup /tmp/restored-projects

# Restore from specific time
duplicity --restore-time 7D \
  s3://bucket/backup /tmp/restored-7-days-ago
```

### Encryption with GPG Keys
```bash
# List GPG keys
gpg --list-keys

# Backup with GPG key (asymmetric encryption)
export GPG_KEY="user@example.com"
duplicity --encrypt-key $GPG_KEY /data s3://bucket/backup

# Restore (will use private key)
duplicity s3://bucket/backup /data-restored
```

## Backup Strategies

### 3-2-1 Backup Rule
```bash
# 1. Local backup (NAS)
duplicity /home file:///mnt/nas/backups/home

# 2. Cloud backup (S3)
duplicity /home s3://bucket/home-backup

# 3. Offsite backup (different region)
duplicity /home s3://s3.eu-west-1.amazonaws.com/backup-eu/home
```

### Full vs Incremental
```bash
# Force full backup
duplicity full /data s3://bucket/backup

# Incremental backup (automatic)
duplicity /data s3://bucket/backup

# Full backup if older than 30 days
duplicity --full-if-older-than 30D /data s3://bucket/backup
```

## Restoration

### Full Restoration
```bash
# Restore entire backup
duplicity s3://bucket/backup /data-restored

# Restore with time travel
duplicity --restore-time 3D s3://bucket/backup /data-3-days-ago
```

### Incremental Verification
```bash
# Verify backup matches source
duplicity verify s3://bucket/backup /data

# Verify specific files
duplicity verify --file-to-restore home/user/important \
  s3://bucket/backup /data
```

## Maintenance

### Cleanup Operations
```bash
# Remove backups older than 90 days
duplicity remove-older-than 90D s3://bucket/backup --force

# Remove all but last N full backups
duplicity remove-all-but-n-full 3 s3://bucket/backup --force

# Remove incremental backups older than full
duplicity remove-all-inc-of-but-n-full 1 s3://bucket/backup --force

# Cleanup failed backups
duplicity cleanup s3://bucket/backup --force
```

### Status and Listing
```bash
# Show backup sets
duplicity collection-status s3://bucket/backup

# List files in backup
duplicity list-current-files s3://bucket/backup

# Show backup chain
duplicity collection-status s3://bucket/backup | grep Chain
```

## Performance Options
```bash
# Increase volume size (larger chunks)
duplicity --volsize 250 /data s3://bucket/backup

# Use multiple threads
duplicity --s3-multipart-chunk-size 25 \
  --s3-use-multiprocessing /data s3://bucket/backup

# Compression level (0-9)
duplicity --compression-level 6 /data s3://bucket/backup
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Backup to S3
  env:
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
    PASSPHRASE: ${{ secrets.BACKUP_PASSPHRASE }}
  run: |
    duplicity --full-if-older-than 7D \
      ./data s3://bucket/ci-backup
```

### Cron Job
```bash
# /etc/cron.d/duplicity-backup
0 2 * * * root /usr/local/bin/backup-script.sh > /var/log/backup.log 2>&1
```

## Agent Use
- Automate encrypted backups to cloud storage
- Schedule incremental backup jobs
- Restore files programmatically in disaster recovery
- Manage backup retention policies
- Verify backup integrity automatically
- Secure sensitive data with encryption
- Implement 3-2-1 backup strategy
- Compliance and audit requirements

## Troubleshooting

### GPG/Passphrase errors
```bash
# Check GPG key
gpg --list-keys

# Test passphrase
echo "test" | gpg --passphrase "$PASSPHRASE" --batch -c | gpg --passphrase "$PASSPHRASE" --batch -d

# Clear cache
rm -rf ~/.cache/duplicity/
```

### S3 authentication failed
```bash
# Test AWS credentials
aws s3 ls s3://bucket/

# Check environment variables
echo $AWS_ACCESS_KEY_ID
echo $AWS_SECRET_ACCESS_KEY
```

### Backup too slow
```bash
# Increase volume size
duplicity --volsize 500 /data s3://bucket/backup

# Use multipart uploads
duplicity --s3-multipart-chunk-size 50 /data s3://bucket/backup

# Reduce compression
duplicity --no-compression /data s3://bucket/backup
```

### Corrupted backup
```bash
# Verify integrity
duplicity verify s3://bucket/backup /data

# Cleanup failed backup
duplicity cleanup --force s3://bucket/backup

# Rebuild cache
rm -rf ~/.cache/duplicity/
duplicity collection-status s3://bucket/backup
```

## Security Best Practices
- **Strong passphrase**: Use 20+ character passphrase
- **GPG keys**: Prefer asymmetric encryption for better key management
- **Encrypted transfer**: Use HTTPS/S3 with TLS
- **Key rotation**: Periodically rotate GPG keys and passphrases
- **Access control**: Restrict S3 bucket IAM permissions
- **Test restores**: Regularly test restoration process

## Uninstall
```yaml
- preset: duplicity
  with:
    state: absent
```

**Note**: Backups are preserved after uninstall. Delete manually if needed.

## Resources
- Official site: https://duplicity.gitlab.io/
- Documentation: https://duplicity.gitlab.io/duplicity-web/
- Man page: `man duplicity`
- Search: "duplicity backup tutorial", "duplicity s3 backup", "duplicity encryption"
