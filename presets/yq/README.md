# yq - YAML/XML/JSON Processor

jq for YAML. Query, modify, and format YAML/XML/JSON files.

## Quick Start
```yaml
- preset: yq
```

## Usage
```bash
# Read value
yq '.name' config.yaml

# Update value
yq '.version = "2.0"' -i config.yaml

# Merge files
yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' file1.yaml file2.yaml

# Convert YAML to JSON
yq -o=json '.' config.yaml

# Filter array
yq '.items[] | select(.active == true)' data.yaml
```

## Resources
Docs: https://github.com/mikefarah/yq
