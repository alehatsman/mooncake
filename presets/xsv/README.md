# xsv - Fast CSV Toolkit

Blazing fast CSV command-line toolkit written in Rust.

## Quick Start
```yaml
- preset: xsv
```

## Usage
```bash
# Stats
xsv stats data.csv

# Select columns
xsv select name,email data.csv

# Search
xsv search -s name "John" data.csv

# Sort
xsv sort -s age data.csv

# Count rows
xsv count data.csv
```

## Resources
GitHub: https://github.com/BurntSushi/xsv
