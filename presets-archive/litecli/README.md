# litecli - SQLite CLI with Auto-Completion

Modern SQLite command-line client with auto-completion, syntax highlighting, and smart completion of SQL keywords, table names, and columns.

## Quick Start
```yaml
- preset: litecli
```

## Features
- **Auto-completion**: Context-aware SQL keyword and table name completion
- **Syntax highlighting**: Color-coded SQL queries
- **Multi-line queries**: Edit complex queries across multiple lines
- **Smart completion**: Completes column names based on context
- **History search**: Navigate previous commands with Ctrl+R
- **Vi/Emacs keybindings**: Choose your preferred editing mode

## Basic Usage
```bash
# Connect to database
litecli mydata.db

# Connect with specific options
litecli --auto-vertical-output mydata.db

# Execute query from command line
litecli mydata.db -e "SELECT * FROM users;"

# Execute SQL file
litecli mydata.db < schema.sql

# Common queries (inside litecli):
SELECT * FROM users;
.tables                    # List tables
.schema users              # Show table schema
.indexes                   # Show indexes
.databases                 # List databases
.output file.txt           # Redirect output to file
.quit                      # Exit
```

## Advanced Configuration
```yaml
- preset: litecli
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove litecli |

## Platform Support
- ✅ Linux (pip install)
- ✅ macOS (Homebrew, pip)
- ✅ Windows (pip install)

## Configuration
- **Config file**: `~/.config/litecli/liteclirc` (Linux/macOS), `%USERPROFILE%\AppData\Local\litecli\liteclirc` (Windows)
- **History**: `~/.config/litecli/history`
- **Keybindings**: Vi or Emacs mode

## Real-World Examples

### Database Queries with Auto-Completion
```bash
# Start litecli
litecli myapp.db

# Type 'SEL' and press Tab - autocompletes to SELECT
# Type 'SELECT * FROM u' and press Tab - shows user table suggestions
# Press Enter to execute
```

### CI/CD Database Migrations
```yaml
- name: Run database migrations
  shell: litecli {{ db_path }} < migrations/schema.sql
  register: migration

- name: Verify migration
  shell: litecli {{ db_path }} -e "SELECT name FROM sqlite_master WHERE type='table';"
  register: tables
```

### Export Query Results
```bash
# Export to CSV
litecli mydata.db --csv -e "SELECT * FROM users;" > users.csv

# Export to JSON
litecli mydata.db --json -e "SELECT * FROM users;" > users.json

# Vertical output for wide tables
litecli mydata.db --auto-vertical-output -e "SELECT * FROM detailed_info;"
```

### Batch Operations
```bash
# Process multiple databases
for db in *.db; do
  echo "Processing $db"
  litecli "$db" -e "VACUUM; ANALYZE;"
done
```

## Agent Use
- Database exploration and debugging
- SQL query development and testing
- Schema inspection and documentation
- Data export and migration
- Database health checks

## Troubleshooting

### Auto-completion not working
Check config file:
```bash
cat ~/.config/litecli/liteclirc | grep auto_completion
```

Enable auto-completion:
```ini
# ~/.config/litecli/liteclirc
[main]
auto_completion = True
multi_line = True
```

### Syntax highlighting disabled
Enable in config:
```ini
[main]
syntax_style = native
```

### Database locked
Close other connections:
```bash
# Check for processes accessing database
lsof mydata.db

# Kill blocking process
kill <PID>
```

## Uninstall
```yaml
- preset: litecli
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/dbcli/litecli
- Documentation: https://litecli.com/
- Search: "litecli sqlite tutorial", "litecli auto-completion"
