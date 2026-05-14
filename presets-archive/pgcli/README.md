# pgcli - PostgreSQL CLI with Autocomplete

Interactive command-line interface for PostgreSQL with auto-completion and syntax highlighting.

## Quick Start
```yaml
- preset: pgcli
```

## Features
- **Smart autocomplete**: Context-aware suggestions for SQL commands
- **Syntax highlighting**: Color-coded SQL queries
- **History**: Navigate previous queries with arrow keys
- **Multi-line queries**: Easy editing of complex queries
- **Cross-platform**: Linux and macOS support

## Basic Usage
```bash
# Connect to local database
pgcli -h localhost -U postgres -d mydb

# Connect with password prompt
pgcli -h localhost -U postgres -d mydb -W

# Connection string
pgcli postgresql://user:password@host:5432/database

# Execute query and exit
pgcli -h localhost -U postgres -c "SELECT * FROM users;"
```

## Advanced Configuration
```yaml
- preset: pgcli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove pgcli |

## Platform Support
- ✅ Linux (pip3)
- ✅ macOS (Homebrew)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `~/.config/pgcli/config` (Linux), `~/Library/Application Support/pgcli/config` (macOS)
- **History**: `~/.config/pgcli/history` (Linux), `~/Library/Application Support/pgcli/history` (macOS)
- **Log file**: `~/.config/pgcli/log` (Linux), `~/Library/Application Support/pgcli/log` (macOS)

## Real-World Examples

### Data Exploration
```bash
# Interactive session
pgcli postgresql://user@localhost/mydb

# List tables
\dt

# Describe table
\d users

# Execute query
SELECT * FROM users WHERE active = true;

# Export results
\o output.txt
SELECT * FROM users;
\o
```

### CI/CD Database Queries
```yaml
- preset: pgcli

- name: Check database schema
  shell: |
    pgcli postgresql://{{ db_user }}:{{ db_pass }}@{{ db_host }}/{{ db_name }} \
      -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';"
  register: schema

- name: Verify expected tables
  assert:
    command:
      cmd: echo "{{ schema.stdout }}" | grep -q "users"
      exit_code: 0
```

### Data Export
```bash
# Export to CSV
pgcli -h localhost -U postgres -d mydb \
  -c "COPY users TO STDOUT WITH CSV HEADER" > users.csv

# Export query results
pgcli -h localhost -U postgres -d mydb \
  -c "SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '7 days'" \
  --csv > weekly_orders.csv
```

## Agent Use
- Execute automated database queries in pipelines
- Extract data for reports and analysis
- Verify database schema and migrations
- Export data for backup or transfer
- Interactive database exploration with better UX than psql

## Troubleshooting

### Connection refused
Check PostgreSQL is running:
```bash
# Linux
sudo systemctl status postgresql

# macOS
brew services list | grep postgresql
```

### Authentication failed
Verify credentials and pg_hba.conf settings:
```bash
# Check PostgreSQL config
cat /etc/postgresql/*/main/pg_hba.conf  # Linux
cat /usr/local/var/postgres/pg_hba.conf  # macOS
```

## Uninstall
```yaml
- preset: pgcli
  with:
    state: absent
```

## Resources
- Official docs: https://www.pgcli.com/
- GitHub: https://github.com/dbcli/pgcli
- Search: "pgcli tutorial", "pgcli configuration"
