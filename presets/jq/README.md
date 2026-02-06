# jq Preset

Lightweight command-line JSON processor. Filter, transform, and query JSON data with ease.

## Quick Start

```yaml
- preset: jq
```

## Common Usage

```bash
# Pretty print
echo '{"name":"John"}' | jq '.'

# Extract field
jq '.name' data.json

# Filter array
jq '.[] | select(.age > 25)' users.json

# Map transformation
jq 'map(.price * 1.1)' products.json

# Get keys
jq 'keys' object.json

# Combine filters
jq '.users[] | {name, email}' data.json
```

## Examples

```bash
# API response processing
curl https://api.github.com/users/github | jq '.name, .followers'

# Extract specific fields from array
jq '.[].{name, price}' products.json

# Conditional filtering
jq '.[] | select(.status == "active")' items.json

# Count items
jq '. | length' array.json

# Pretty print with colors
jq -C '.' data.json | less -R
```

## Resources
- Docs: https://jqlang.github.io/jq/
- Tutorial: https://jqlang.github.io/jq/tutorial/
