# MySQL Preset

**Status:** ✓ Installed successfully

## Quick Start

```bash
# Connect to MySQL
mysql -u root

# Check status
sudo systemctl status mysql  # Linux
brew services list | grep mysql  # macOS

# Connect to created database (if specified)
mysql -u [username] -p [database_name]
```

## Configuration

- **Config file:** `/etc/mysql/mysql.conf.d/mysqld.cnf` (Linux), `/usr/local/etc/my.cnf` (macOS)
- **Data directory:** `/var/lib/mysql` (Linux), `/usr/local/var/mysql` (macOS)
- **Default port:** 3306
- **Socket:** `/var/run/mysqld/mysqld.sock` (Linux), `/tmp/mysql.sock` (macOS)

## Common Operations

```bash
# Restart MySQL
sudo systemctl restart mysql  # Linux
brew services restart mysql  # macOS

# Create database
mysql -u root -e "CREATE DATABASE mydb"

# Create user with permissions
mysql -u root -e "CREATE USER 'user'@'localhost' IDENTIFIED BY 'password'"
mysql -u root -e "GRANT ALL PRIVILEGES ON mydb.* TO 'user'@'localhost'"
mysql -u root -e "FLUSH PRIVILEGES"

# Backup database
mysqldump -u root database_name > backup.sql

# Restore database
mysql -u root database_name < backup.sql

# Show databases
mysql -u root -e "SHOW DATABASES"
```

## Security

```bash
# Secure installation (recommended)
sudo mysql_secure_installation
```

## Uninstall

```yaml
- preset: mysql
  with:
    state: absent
```

**Note:** Data directory `/var/lib/mysql` is preserved after uninstall.
