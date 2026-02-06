# Database CLI

Interactive database client with autocomplete and syntax highlighting.

## Quick Start
```yaml
- preset: mycli
```

## Connection
```bash
# Connect to database
mycli -h host -u user -d database

# With password
mycli -h host -u user -p

# Connection string
mycli "connection://user:pass@host/db"
```

## Usage
```bash
# Execute query
mycli -e "SELECT * FROM users"

# Interactive mode
mycli

# Export results
mycli -e "SELECT * FROM users" --format json > users.json
```

## Agent Use
- Automated database queries
- Data extraction
- Schema inspection
- Backup and restore
- Migration scripts

## Resources
Search: "mycli documentation"
