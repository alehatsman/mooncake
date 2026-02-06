# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: yq
```

## Common Usage
```bash
# Process JSON
cat data.json | yq '.'

# Query specific fields
yq '.field' data.json

# Transform data
yq 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "yq examples" or "yq tutorial"
