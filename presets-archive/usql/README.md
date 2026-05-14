# usql - Universal SQL CLI

Universal command-line interface for SQL databases. Connect to PostgreSQL, MySQL, SQLite, Oracle, and 20+ databases with consistent commands.

## Quick Start
```yaml
- preset: usql
```

## Features
- **20+ Database Support**: PostgreSQL, MySQL, SQLite, Oracle, MSSQL, and more
- **Unified Interface**: Same commands across all databases
- **psql Compatibility**: Familiar PostgreSQL commands (\d, \dt, etc.)
- **Multiple Formats**: Table, CSV, JSON, XML output
- **Cross-platform**: Linux, macOS, and Windows support
- **No Dependencies**: Single binary, no drivers needed

## Basic Usage
```bash
# Connect to PostgreSQL
usql postgres://user:pass@localhost/dbname
usql pg://user:pass@localhost/dbname

# Connect to MySQL
usql mysql://user:pass@localhost/dbname
usql my://user:pass@localhost/dbname

# Connect to SQLite
usql sqlite:///path/to/database.db
usql sq://./local.db

# Connect to MSSQL
usql mssql://user:pass@localhost/dbname
usql ms://user:pass@localhost/dbname
```

## Supported Databases
```bash
# PostgreSQL variants
usql postgres://...
usql cockroach://...

# MySQL variants
usql mysql://...
usql mariadb://...

# NoSQL
usql mongodb://...
usql redis://...

# Other SQL
usql sqlite://...
usql oracle://...
usql mssql://...
usql snowflake://...
usql clickhouse://...
usql bigquery://...
usql cassandra://...
usql elasticsearch://...
```

## Interactive Commands
```bash
# List tables
\d
\dt

# Describe table
\d table_name

# List databases
\l

# Connect to different database
\c dbname

# Execute file
\i script.sql

# Show help
\?

# Quit
\q
```

## Query Execution
```bash
# Run query
usql postgres://localhost/db -c "SELECT * FROM users"

# Execute file
usql postgres://localhost/db -f queries.sql

# Multiple statements
usql postgres://localhost/db -c "CREATE TABLE test (id INT); INSERT INTO test VALUES (1);"

# With variables
usql postgres://localhost/db --set=env=production -c "SELECT * FROM users WHERE environment = :env"
```

## Output Formats
```bash
# Table format (default)
usql postgres://localhost/db -c "SELECT * FROM users"

# CSV
usql postgres://localhost/db --csv -c "SELECT * FROM users"

# JSON
usql postgres://localhost/db --json -c "SELECT * FROM users"

# JSON Lines
usql postgres://localhost/db --jsonl -c "SELECT * FROM users"

# Vertical
usql postgres://localhost/db --expanded -c "SELECT * FROM users"
```

## Real-World Examples

### Database Migration Check
```bash
#!/bin/bash
# Check if migration needed
usql postgres://localhost/mydb -c "
  SELECT version FROM schema_migrations
  ORDER BY version DESC LIMIT 1
" --tuples-only --csv
```

### Multi-Database Query
```bash
# Query PostgreSQL
usql pg://localhost/db1 -c "SELECT COUNT(*) FROM users" --tuples-only

# Query MySQL
usql my://localhost/db2 -c "SELECT COUNT(*) FROM orders" --tuples-only

# Query SQLite
usql sq://./local.db -c "SELECT COUNT(*) FROM logs" --tuples-only
```

### Export to CSV
```bash
# Export table
usql postgres://localhost/db -c "
  SELECT id, email, created_at
  FROM users
  WHERE created_at > NOW() - INTERVAL '7 days'
" --csv > users_last_week.csv
```

### Backup and Restore
```bash
# Dump schema
usql postgres://localhost/db -c "\d" > schema.sql

# Execute backup
usql postgres://localhost/db -f backup.sql
```

## Connection String Formats
```bash
# Full format
usql driver://user:pass@host:port/database?param=value

# PostgreSQL
usql postgres://user:password@localhost:5432/mydb
usql postgresql://user:password@localhost/mydb?sslmode=disable

# MySQL
usql mysql://root:password@localhost:3306/mydb
usql my://root:password@tcp(localhost:3306)/mydb

# SQLite (file path)
usql sqlite:///absolute/path/to/db.sqlite
usql sq://./relative/path/to/db.sqlite

# Environment variable
export DATABASE_URL="postgres://localhost/mydb"
usql $DATABASE_URL
```

## Configuration File
```yaml
# ~/.usqlrc or .usqlrc
set autocommit on
set timing on
set null '∅'
set prompt1 '%n@%m/%/%R%#%x '
set prompt2 '%R%#%x '

# Aliases
\set sales 'SELECT * FROM sales WHERE year = 2024'
\set users 'SELECT id, email FROM users'
```

## Scripting Examples

### CI/CD Health Check
```bash
#!/bin/bash
set -e

# Check database connectivity
if usql postgres://localhost/db -c "SELECT 1" > /dev/null 2>&1; then
  echo "Database is up"
else
  echo "Database is down"
  exit 1
fi

# Check table exists
if usql postgres://localhost/db -c "\dt users" | grep -q users; then
  echo "Users table exists"
else
  echo "Users table missing"
  exit 1
fi
```

### Data Quality Check
```bash
# Check for null emails
null_count=$(usql postgres://localhost/db \
  -c "SELECT COUNT(*) FROM users WHERE email IS NULL" \
  --tuples-only --csv)

if [ "$null_count" -gt 0 ]; then
  echo "Warning: $null_count users have null emails"
fi
```

### Multi-Environment Query
```bash
# Query all environments
for env in dev staging prod; do
  echo "Environment: $env"
  usql "postgres://$env.db.company.com/mydb" \
    -c "SELECT COUNT(*) FROM users" \
    --tuples-only
done
```

## Advanced Configuration
```yaml
- preset: usql
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove usql |

## Platform Support
- ✅ Linux (binary download)
- ✅ macOS (Homebrew or binary download)
- ✅ Windows (binary download)

## Environment Variables
```bash
# Default database URL
export DATABASE_URL="postgres://localhost/mydb"
usql  # Connects to DATABASE_URL

# Disable password prompt
export PGPASSWORD="secretpassword"
usql postgres://user@localhost/db

# Config file location
export USQLRC="/path/to/custom/.usqlrc"
```

## Troubleshooting

### Connection refused
```bash
# Check if database is running
usql postgres://localhost/db -c "SELECT 1"

# Use IP instead of hostname
usql postgres://127.0.0.1/db
```

### SSL/TLS errors
```bash
# Disable SSL
usql "postgres://localhost/db?sslmode=disable"

# Require SSL
usql "postgres://localhost/db?sslmode=require"
```

### Driver not found
```bash
# List available drivers
usql --drivers

# Check version
usql --version
```

## Agent Use
- Multi-database queries in CI/CD pipelines
- Database health checks and monitoring
- Data exports in various formats (CSV, JSON)
- Schema inspection and validation
- Cross-database migrations and comparisons
- Automated data quality checks

## Uninstall
```yaml
- preset: usql
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/xo/usql
- Official docs: https://github.com/xo/usql/blob/master/README.md
- Search: "usql examples", "usql database support", "usql connection strings"
