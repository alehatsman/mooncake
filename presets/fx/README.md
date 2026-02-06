# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: fx
```

## Common Usage
```bash
# Process JSON
cat data.json | fx '.'

# Query specific fields
fx '.field' data.json

# Transform data
fx 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "fx examples" or "fx tutorial"
