# Database CLI

Interactive database client with autocomplete and syntax highlighting.

## Quick Start
```yaml
- preset: mongosh
```

## Connection
```bash
# Connect to database
mongosh -h host -u user -d database

# With password
mongosh -h host -u user -p

# Connection string
mongosh "connection://user:pass@host/db"
```

## Usage
```bash
# Execute query
mongosh -e "SELECT * FROM users"

# Interactive mode
mongosh

# Export results
mongosh -e "SELECT * FROM users" --format json > users.json
```

## Agent Use
- Automated database queries
- Data extraction
- Schema inspection
- Backup and restore
- Migration scripts

## Resources
Search: "mongosh documentation"
