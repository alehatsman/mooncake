# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: jless
```

## Common Usage
```bash
# Process JSON
cat data.json | jless '.'

# Query specific fields
jless '.field' data.json

# Transform data
jless 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "jless examples" or "jless tutorial"
