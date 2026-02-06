# gron - Make JSON Greppable

Flatten JSON to make it greppable. Transform JSON into discrete assignments, filter with grep, then convert back.

## Quick Start
```yaml
- preset: gron
```

## Basic Usage
```bash
# Flatten JSON
gron data.json

# From stdin
curl https://api.github.com/users/octocat | gron

# Flatten and grep
gron data.json | grep email

# Convert back to JSON
gron data.json | grep email | gron --ungron
```

## How It Works
```bash
# Input JSON:
{
  "name": "Alice",
  "age": 30,
  "emails": ["alice@example.com", "alice@work.com"]
}

# gron output:
json = {};
json.name = "Alice";
json.age = 30;
json.emails = [];
json.emails[0] = "alice@example.com";
json.emails[1] = "alice@work.com";
```

Now you can grep it!

## Filtering with Grep
```bash
# Find all email addresses
gron data.json | grep email

# Find specific fields
gron users.json | grep '\.name = '

# Multiple patterns
gron data.json | grep -E 'email|phone'

# Exclude patterns
gron data.json | grep -v password

# Case insensitive
gron data.json | grep -i error
```

## Ungron (Convert Back)
```bash
# Filter and convert back to JSON
gron data.json | grep active | gron --ungron

# Extract subset
gron users.json | grep 'users\[0\]' | gron --ungron

# Remove fields
gron data.json | grep -v password | gron --ungron

# Transform structure
gron data.json | sed 's/old_field/new_field/' | gron --ungron
```

## Practical Examples
```bash
# Find all URLs
gron api-response.json | grep -o 'https://[^"]*'

# Extract user emails
gron users.json | grep '\.email = ' | cut -d'"' -f2

# Count occurrences
gron logs.json | grep error | wc -l

# Find nested values
gron data.json | grep 'metadata.*version'

# Get array indices
gron data.json | grep '\[' | grep -o '\[[0-9]*\]' | sort -u
```

## API Response Processing
```bash
# GitHub API
curl -s https://api.github.com/repos/owner/repo | \
  gron | grep -E 'stargazers_count|forks_count'

# Extract nested data
curl -s https://api.example.com/users | \
  gron | grep '\.id = \|\.name = '

# Filter by value
curl -s https://api.example.com/posts | \
  gron | grep 'published.*true'

# Multiple APIs comparison
diff <(curl -s api1.com | gron) <(curl -s api2.com | gron)
```

## Searching Nested Structures
```bash
# Find all IDs
gron data.json | grep '\.id = '

# Find in arrays
gron data.json | grep 'items\[.*\]\.name'

# Deep nesting
gron data.json | grep 'user\.profile\.settings\.theme'

# All keys containing 'config'
gron data.json | grep config

# Values matching pattern
gron data.json | grep '= "production"'
```

## Modification Workflows
```bash
# Remove sensitive fields
gron config.json | \
  grep -v password | \
  grep -v secret | \
  gron --ungron > sanitized.json

# Rename fields
gron data.json | \
  sed 's/old_name/new_name/' | \
  gron --ungron > updated.json

# Filter array items
gron items.json | \
  grep '\.active = true' | \
  gron --ungron > active-items.json

# Extract specific indices
gron data.json | \
  grep -E 'items\[(0|1|2)\]' | \
  gron --ungron > first-three.json
```

## CI/CD Integration
```bash
# Check for required fields
if ! gron config.json | grep -q '\.apiKey'; then
  echo "Missing apiKey in config"
  exit 1
fi

# Count errors in logs
ERROR_COUNT=$(gron logs.json | grep -c error)
if [ $ERROR_COUNT -gt 10 ]; then
  echo "Too many errors: $ERROR_COUNT"
  exit 1
fi

# Extract version
gron package.json | grep '\.version = ' | cut -d'"' -f2

# Validate structure
REQUIRED_FIELDS="name version description"
for field in $REQUIRED_FIELDS; do
  if ! gron config.json | grep -q "\.$field = "; then
    echo "Missing field: $field"
    exit 1
  fi
done
```

## Debugging
```bash
# See structure
gron data.json | less

# Find where field exists
gron large-file.json | grep fieldname

# Compare two JSON files
diff <(gron file1.json) <(gron file2.json)

# Show only differences
diff <(gron file1.json) <(gron file2.json) | grep '^[<>]'

# Count total fields
gron data.json | wc -l

# Find deepest nesting
gron data.json | awk -F'.' '{print NF-1}' | sort -rn | head -1
```

## Array Operations
```bash
# Count array length
gron data.json | grep 'items\[' | tail -1

# Find max array index
gron data.json | grep -o '\[[0-9]*\]' | tr -d '[]' | sort -rn | head -1

# Extract specific index
gron data.json | grep 'items\[5\]' | gron --ungron

# Filter array by value
gron data.json | grep 'users.*\.role = "admin"' | gron --ungron
```

## Complex Filtering
```bash
# Multiple conditions (AND)
gron data.json | grep active | grep premium | gron --ungron

# Multiple conditions (OR)
gron data.json | grep -E 'active|premium' | gron --ungron

# Nested field filtering
gron data.json | \
  grep 'metadata' | \
  grep 'environment.*production' | \
  gron --ungron

# Range filtering (requires processing)
gron data.json | \
  grep '\.age = ' | \
  awk -F'= ' '$2 > 18' | \
  gron --ungron
```

## Comparison
| Feature | gron | jq | fx | grep |
|---------|------|-----|-----|------|
| Flatten JSON | Yes | No | No | N/A |
| Grep friendly | Yes | No | No | Yes |
| Modify & rebuild | Yes | Yes | Yes | No |
| Learning curve | Low | High | Low | Low |

## Real-World Scenarios
```bash
# Find all email addresses in complex JSON
gron data.json | grep -o '[a-zA-Z0-9._%+-]*@[a-zA-Z0-9.-]*\.[a-zA-Z]*'

# Extract environment variables
gron config.json | grep 'env\.' | cut -d'=' -f2 | tr -d ' ";'

# List all unique field names
gron data.json | sed 's/\[.*\]//' | sed 's/ =.*//' | sort -u

# Find circular references (same ID appears multiple times)
gron data.json | grep '\.id = ' | sort | uniq -d

# Security: find exposed secrets
gron config.json | grep -iE 'password|secret|key|token' | grep -v 'keypress'
```

## Colorized Output
```bash
# With colors (if supported)
gron --colorize data.json

# Pipe through grep with color
gron data.json | grep --color=always email | less -R
```

## Best Practices
- **Use for searching** complex JSON structures
- **Combine with grep** for powerful filtering
- **Ungron to rebuild** JSON after modifications
- **Compare JSON files** with diff
- **Extract specific paths** without jq complexity
- **Debug nested structures** by flattening
- **Find all occurrences** of fields/values

## Tips
- Faster than jq for simple searches
- Perfect for shell scripting
- Works with standard Unix tools (grep, sed, awk)
- Easy to understand output
- Great for learning JSON structure
- No query language to learn
- Bidirectional (gron/ungron)

## Agent Use
- JSON structure exploration
- Field extraction pipelines
- Configuration validation
- Log analysis (JSON logs)
- API response filtering
- Security audits (find secrets)

## Uninstall
```yaml
- preset: gron
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/tomnomnom/gron
- Search: "gron json grep", "gron examples"
