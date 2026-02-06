# miller - Data Processing Tool

Like awk/sed/cut/join for CSV, TSV, JSON, and more.

## Quick Start
```yaml
- preset: miller
```

## Usage
```bash
# CSV to JSON
mlr --icsv --ojson cat data.csv

# Filter
mlr --csv filter '$age > 25' data.csv

# Stats
mlr --csv stats1 -a sum -f amount data.csv

# Join
mlr --csv join -f file1.csv -j id file2.csv
```

## Resources
Docs: https://miller.readthedocs.io/
