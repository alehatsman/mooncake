# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: xsv
```

## Common Usage
```bash
# Process JSON
cat data.json | xsv '.'

# Query specific fields
xsv '.field' data.json

# Transform data
xsv 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "xsv examples" or "xsv tutorial"
