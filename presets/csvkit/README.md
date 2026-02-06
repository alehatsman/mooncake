# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: csvkit
```

## Common Usage
```bash
# Process JSON
cat data.json | csvkit '.'

# Query specific fields
csvkit '.field' data.json

# Transform data
csvkit 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "csvkit examples" or "csvkit tutorial"
