# Database CLI

Interactive database client with autocomplete and syntax highlighting.

## Quick Start
```yaml
- preset: redis-cli
```

## Connection
```bash
# Connect to database
redis-cli -h host -u user -d database

# With password
redis-cli -h host -u user -p

# Connection string
redis-cli "connection://user:pass@host/db"
```

## Usage
```bash
# Execute query
redis-cli -e "SELECT * FROM users"

# Interactive mode
redis-cli

# Export results
redis-cli -e "SELECT * FROM users" --format json > users.json
```

## Agent Use
- Automated database queries
- Data extraction
- Schema inspection
- Backup and restore
- Migration scripts

## Resources
Search: "redis-cli documentation"
