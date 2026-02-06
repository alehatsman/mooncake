# Database CLI

Interactive database client with autocomplete and syntax highlighting.

## Quick Start
```yaml
- preset: litecli
```

## Connection
```bash
# Connect to database
litecli -h host -u user -d database

# With password
litecli -h host -u user -p

# Connection string
litecli "connection://user:pass@host/db"
```

## Usage
```bash
# Execute query
litecli -e "SELECT * FROM users"

# Interactive mode
litecli

# Export results
litecli -e "SELECT * FROM users" --format json > users.json
```

## Agent Use
- Automated database queries
- Data extraction
- Schema inspection
- Backup and restore
- Migration scripts

## Resources
Search: "litecli documentation"
