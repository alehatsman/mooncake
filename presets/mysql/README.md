# mysql - Relational Database Management System

MySQL is a reliable, open-source relational database that powers applications from small startups to large enterprises. This preset automates installation, service configuration, and database setup across Linux and macOS.

## Quick Start

```yaml
- preset: mysql
```

## Features

- **Cross-platform**: Installs on Ubuntu, Fedora, CentOS, Arch, and macOS
- **Auto service management**: Configures systemd (Linux) or Homebrew services (macOS)
- **Database creation**: Automatically create databases and users
- **Idempotent**: Safe to run multiple times
- **Security conscious**: Proper permission handling for database files
- **Production ready**: Includes performance tuning parameters in default config

## Basic Usage

```bash
# Connect as root
mysql -u root

# List databases
mysql -u root -e "SHOW DATABASES"

# Execute query
mysql -u root -e "SELECT VERSION()"

# Connect to specific database
mysql -u root -d mydb

# Create backup
mysqldump -u root --all-databases > backup.sql

# Restore backup
mysql -u root < backup.sql

# Check service status (Linux)
sudo systemctl status mysql

# Check service status (macOS)
brew services list | grep mysql
```

## Advanced Configuration

```yaml
# Installation with database and user creation
- preset: mysql
  with:
    state: present
    start_service: true
    create_database: myapp_db
    create_user: myapp_user
    user_password: secure_password
    port: 3306
  become: true

# Custom port
- preset: mysql
  with:
    port: 3307
  become: true

# Uninstall
- preset: mysql
  with:
    state: absent
  become: true
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) |
| start_service | bool | true | Start MySQL service after installation |
| root_password | string | - | Root user password (optional) |
| create_database | string | - | Database name to create |
| create_user | string | - | Username to create |
| user_password | string | - | Password for created user |
| port | number | 3306 | MySQL listening port |

## Platform Support

- ✅ Linux (Ubuntu/Debian via apt, Fedora/RHEL via dnf/yum, Arch via pacman)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration

- **Config file**: `/etc/mysql/mysql.conf.d/mysqld.cnf` (Linux), `/usr/local/etc/my.cnf` (macOS)
- **Data directory**: `/var/lib/mysql` (Linux), `/usr/local/var/mysql` (macOS)
- **Socket file**: `/var/run/mysqld/mysqld.sock` (Linux), `/tmp/mysql.sock` (macOS)
- **Default port**: 3306
- **Default user**: root (no password on fresh installation)

## Real-World Examples

### Development Environment Setup

```yaml
- preset: mysql
  with:
    create_database: development
    create_user: dev
    user_password: dev_password
  become: true

- name: Display connection info
  print: |
    Database created: development
    User: dev
    Password: dev_password
    Command: mysql -u dev -p development
```

### Production Database with Performance Tuning

```yaml
- preset: mysql
  with:
    state: present
    port: 3306
    start_service: true
    create_database: production_db
    create_user: app_user
    user_password: strong_password_here
  become: true

# Verify service is healthy
- name: Health check
  assert:
    command:
      cmd: mysql -u root -e "SELECT 1"
      exit_code: 0
```

### Backup and Restore Pipeline

```bash
# Backup all databases
mysqldump -u root --all-databases --single-transaction --quick > backup-$(date +%Y%m%d).sql

# Backup specific database
mysqldump -u root mydb > mydb-backup.sql

# Restore from backup
mysql -u root < backup-20260206.sql

# Verify restore
mysql -u root -e "SHOW TABLES FROM mydb"
```

### Multi-User Setup with Permissions

```sql
-- Create application database
CREATE DATABASE IF NOT EXISTS myapp;

-- Create service account (limited permissions)
CREATE USER IF NOT EXISTS 'myapp'@'localhost' IDENTIFIED BY 'app_password';
GRANT SELECT, INSERT, UPDATE, DELETE ON myapp.* TO 'myapp'@'localhost';

-- Create admin account (full permissions)
CREATE USER IF NOT EXISTS 'admin'@'localhost' IDENTIFIED BY 'admin_password';
GRANT ALL PRIVILEGES ON myapp.* TO 'admin'@'localhost';

-- Create read-only account
CREATE USER IF NOT EXISTS 'reader'@'localhost' IDENTIFIED BY 'reader_password';
GRANT SELECT ON myapp.* TO 'reader'@'localhost';

FLUSH PRIVILEGES;
```

### Automated Health Check

```bash
#!/bin/bash
# Check MySQL connectivity and performance

echo "Checking MySQL status..."
mysql -u root -e "SELECT 1" || exit 1

echo "Checking database size..."
mysql -u root -e "SELECT table_schema, SUM(data_length) FROM information_schema.tables GROUP BY table_schema"

echo "Checking connections..."
mysql -u root -e "SHOW PROCESSLIST"

echo "✓ MySQL healthy"
```

## Agent Use

- Provision databases for applications during deployment
- Validate database connectivity in CI/CD pipelines
- Create user accounts with appropriate permissions
- Monitor database performance and queries
- Backup and restore data for disaster recovery
- Run automated health checks and diagnostics
- Execute database migrations and schema updates

## Troubleshooting

### Service won't start

Check logs and permissions:

```bash
# Linux - check systemd logs
sudo journalctl -u mysql -f

# macOS - check Homebrew logs
brew services log mysql

# Verify data directory permissions
ls -la /var/lib/mysql     # Should be mysql:mysql
sudo chown -R mysql:mysql /var/lib/mysql
```

### Connection refused

Verify MySQL is running and listening:

```bash
# Check if service is running
sudo systemctl status mysql    # Linux
brew services list | grep mysql  # macOS

# Check listening ports
sudo lsof -i :3306

# Test connection
mysql -u root -h 127.0.0.1 -e "SELECT 1"
```

### Access denied for user 'root'@'localhost'

Reset root password:

```bash
# Stop MySQL
sudo systemctl stop mysql

# Start with skip-grant-tables
sudo mysqld_safe --skip-grant-tables &

# Connect without password
mysql -u root

# Reset password
FLUSH PRIVILEGES;
ALTER USER 'root'@'localhost' IDENTIFIED BY 'newpassword';
```

### Port already in use

MySQL may be running on the port:

```bash
# Find process using port
sudo lsof -i :3306

# Use different port
mysql -u root -P 3307
```

### Insufficient disk space

Check available space and clean up:

```bash
# Check disk usage
df -h

# Check MySQL data directory size
du -sh /var/lib/mysql

# Optimize tables
mysql -u root -e "OPTIMIZE TABLE table_name"
```

## Uninstall

```yaml
- preset: mysql
  with:
    state: absent
  become: true
```

**Note:** Data directory (`/var/lib/mysql`) is preserved after uninstallation for safety. Remove manually if needed:

```bash
sudo rm -rf /var/lib/mysql
```

## Resources

- Official docs: https://dev.mysql.com/doc/
- MySQL 8.0 Reference: https://dev.mysql.com/doc/refman/8.0/en/
- GitHub: https://github.com/mysql/mysql-server
- Search: "MySQL tutorial", "MySQL user management", "MySQL backup restore"
