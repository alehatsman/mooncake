# csvkit - CSV Tools Suite

Comprehensive suite of tools for working with CSV files.

## Quick Start
```yaml
- preset: csvkit
```

## Usage
```bash
# Stats
csvstat data.csv

# SQL queries
csvsql --query "SELECT * FROM data WHERE age > 25" data.csv

# Convert Excel to CSV
in2csv data.xlsx > data.csv

# Pretty print
csvlook data.csv

# Join files
csvjoin -c id file1.csv file2.csv
```

## Resources
Docs: https://csvkit.readthedocs.io/
