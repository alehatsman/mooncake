# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: miller
```

## Common Usage
```bash
# Process JSON
cat data.json | miller '.'

# Query specific fields
miller '.field' data.json

# Transform data
miller 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "miller examples" or "miller tutorial"
