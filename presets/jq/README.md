# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: jq
```

## Common Usage
```bash
# Process JSON
cat data.json | jq '.'

# Query specific fields
jq '.field' data.json

# Transform data
jq 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "jq examples" or "jq tutorial"
