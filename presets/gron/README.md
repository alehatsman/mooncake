# JSON/Data Processing Tool

Process, query, and transform JSON/YAML/CSV data.

## Quick Start
```yaml
- preset: gron
```

## Common Usage
```bash
# Process JSON
cat data.json | gron '.'

# Query specific fields
gron '.field' data.json

# Transform data
gron 'map(.)' input.json > output.json
```

## Agent Use
- Parse API responses
- Transform data formats
- Extract specific fields
- Filter and map collections
- Automate data processing pipelines

## Resources
Search: "gron examples" or "gron tutorial"
