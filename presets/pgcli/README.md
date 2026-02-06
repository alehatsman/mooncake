# Database CLI

Interactive database client with autocomplete and syntax highlighting.

## Quick Start
```yaml
- preset: pgcli
```

## Connection
```bash
# Connect to database
pgcli -h host -u user -d database

# With password
pgcli -h host -u user -p

# Connection string
pgcli "connection://user:pass@host/db"
```

## Usage
```bash
# Execute query
pgcli -e "SELECT * FROM users"

# Interactive mode
pgcli

# Export results
pgcli -e "SELECT * FROM users" --format json > users.json
```

## Agent Use
- Automated database queries
- Data extraction
- Schema inspection
- Backup and restore
- Migration scripts

## Resources
Search: "pgcli documentation"
