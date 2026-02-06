# mycli - MySQL Client with Auto-Completion and Syntax Highlighting

A terminal-based MySQL client with auto-completion, syntax highlighting, and helpful features like automatic database selection and query history.

## Quick Start

```yaml
- preset: mycli
```

## Features

- **Auto-completion**: Smart field and table name suggestions
- **Syntax highlighting**: Colorized SQL queries for readability
- **Smart prompt**: Shows current database and connection status
- **Query history**: Persistent command history across sessions
- **Formatted output**: Tables displayed with proper alignment
- **Multiple connection formats**: Host/port, socket, or connection strings
- **Cross-platform**: Linux and macOS support

## Basic Usage

```bash
# Interactive mode (prompted for connection details)
mycli

# Connect to default localhost
mycli -u root

# Connect to specific host and database
mycli -h localhost -u root -d mydb

# With password prompt
mycli -h localhost -u root -p

# Execute query and exit
mycli -u root -e "SELECT * FROM users LIMIT 5"

# Format output as JSON
mycli -u root -e "SELECT * FROM users" --format json

# View version
mycli --version
```

## Advanced Configuration

```yaml
# Basic installation
- preset: mycli

# With uninstall
- preset: mycli
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, pip3)
- ✅ macOS (Homebrew, pip3)
- ❌ Windows (not supported)

## Configuration

- **Config file**: `~/.myclirc` (Linux/macOS)
- **History file**: `~/.mycli_history` (Linux/macOS)
- **Default port**: 3306
- **Socket location**: `/var/run/mysqld/mysqld.sock` (Linux), `/tmp/mysql.sock` (macOS)

## Real-World Examples

### Development Database Queries

```bash
# Quick check for recent data
mycli -u dev_user -d development -e "SELECT COUNT(*) FROM users WHERE created_at > DATE_SUB(NOW(), INTERVAL 24 HOUR)"

# Inspect schema
mycli -u dev_user -d development -e "DESCRIBE users"
```

### Database Administration

```bash
# Backup database schema
mycli -u root -e "SHOW CREATE TABLE users" > schema.sql

# Monitor slow queries in interactive mode
mycli -u admin -d production

# Export data for analysis
mycli -u analyst -d analytics -e "SELECT * FROM metrics" --format json > metrics.json
```

### CI/CD Integration

```bash
# Health check in deployment script
if mycli -u health_check -h db.internal -e "SELECT 1" 2>/dev/null; then
  echo "Database is healthy"
else
  echo "Database connection failed"
  exit 1
fi
```

## Agent Use

- Execute SQL queries for data extraction and analysis
- Validate database connectivity in deployment pipelines
- Monitor database schema and structure changes
- Extract metrics and monitoring data
- Perform database migrations and schema updates
- Automate data backup and validation tasks

## Troubleshooting

### Connection refused

Check if MySQL is running and listening:
```bash
# Test connection
mysql -u root -h 127.0.0.1 -e "SELECT 1"

# Check if socket exists
ls -la /var/run/mysqld/mysqld.sock  # Linux
ls -la /tmp/mysql.sock             # macOS
```

### Authentication failure

Verify credentials and permissions:
```bash
# Reset root password (requires sudo)
sudo mysql -u root -e "ALTER USER 'root'@'localhost' IDENTIFIED BY 'newpassword'"
```

### Port already in use

MySQL may already be running on port 3306:
```bash
# Check what's using the port
lsof -i :3306

# Use different port for new instance
mycli -h localhost -P 3307
```

## Uninstall

```yaml
- preset: mycli
  with:
    state: absent
```

## Resources

- Official docs: https://www.mycli.net/
- GitHub: https://github.com/dbcli/mycli
- Search: "mycli tutorial", "mycli database queries", "mycli connection guide"
