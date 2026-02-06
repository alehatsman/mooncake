# miller - Name-Indexed Data Processor

Like awk, sed, cut, join, and sort for CSV, TSV, JSON, and more. Stream processing with named fields.

## Quick Start
```yaml
- preset: miller
```

## Basic Usage
```bash
# View CSV
mlr --csv cat data.csv

# Pretty print
mlr --c2p cat data.csv  # CSV to pretty table

# CSV to JSON
mlr --c2j cat data.csv

# JSON to CSV
mlr --j2c cat data.json
```

## Format Conversions
```bash
# CSV to JSON
mlr --c2j cat data.csv > data.json

# JSON to CSV
mlr --j2c cat data.json > data.csv

# TSV to CSV
mlr --t2c cat data.tsv > data.csv

# Pretty print (aligned columns)
mlr --c2p cat data.csv

# NIDX (numeric index)
mlr --n2c cat data.txt > data.csv

# Multiple formats
mlr --icsv --ojson cat input.csv > output.json
```

## Selecting Fields
```bash
# Select columns
mlr --csv cut -f name,email data.csv

# Reorder columns
mlr --csv cut -o -f email,name,age data.csv

# Exclude columns
mlr --csv cut -x -f password,secret data.csv

# Regex selection
mlr --csv cut -r -f '^user_' data.csv
```

## Filtering Rows
```bash
# Filter condition
mlr --csv filter '$age > 25' data.csv

# Multiple conditions
mlr --csv filter '$age > 25 && $status == "active"' data.csv

# Regex match
mlr --csv filter '$email =~ "@gmail\.com$"' data.csv

# Null check
mlr --csv filter '$phone != null' data.csv

# Numeric comparison
mlr --csv filter '$salary >= 50000' data.csv
```

## Transformation
```bash
# Add computed field
mlr --csv put '$total = $price * $quantity' data.csv

# Modify field
mlr --csv put '$name = toupper($name)' data.csv

# Conditional field
mlr --csv put '
  $category = ($age < 18) ? "minor" : "adult"
' data.csv

# Multiple operations
mlr --csv put '
  $full_name = $first . " " . $last;
  $age_group = floor($age / 10) * 10
' data.csv
```

## Aggregation
```bash
# Count rows
mlr --csv count data.csv

# Group by
mlr --csv stats1 -a count -f status -g department data.csv

# Sum
mlr --csv stats1 -a sum -f amount -g category data.csv

# Multiple statistics
mlr --csv stats1 -a mean,median,stddev -f salary -g department data.csv

# Count unique
mlr --csv stats1 -a distinct_count -f email data.csv
```

## Sorting
```bash
# Sort by field
mlr --csv sort -f age data.csv

# Reverse sort
mlr --csv sort -r age data.csv

# Multiple fields
mlr --csv sort -f department,salary data.csv

# Numeric sort
mlr --csv sort -n amount data.csv
```

## Joining
```bash
# Join two files
mlr --csv join -j id -f users.csv departments.csv

# Left join
mlr --csv join -j id -l -f users.csv departments.csv

# Different join keys
mlr --csv join -l user_id -r id -f users.csv orders.csv

# Multiple keys
mlr --csv join -j user_id,product_id -f file1.csv file2.csv
```

## Reshaping
```bash
# Rename fields
mlr --csv rename old_name,new_name data.csv

# Reorder all fields alphabetically
mlr --csv reorder -f data.csv

# Transpose
mlr --csv transpose data.csv

# Flatten nested JSON
mlr --json flatten data.json
```

## String Operations
```bash
# Uppercase
mlr --csv put '$name = toupper($name)' data.csv

# Lowercase
mlr --csv put '$email = tolower($email)' data.csv

# Substring
mlr --csv put '$initials = substr($name, 0, 1)' data.csv

# Replace
mlr --csv put '$phone = gsub($phone, "-", "")' data.csv

# Split
mlr --csv put '
  @parts = splita($name, " ");
  $first = @parts[1];
  $last = @parts[2]
' data.csv
```

## Date/Time Operations
```bash
# Parse timestamp
mlr --csv put '$date = strftime($timestamp, "%Y-%m-%d")' data.csv

# Current time
mlr --csv put '$processed_at = systime()' data.csv

# Date arithmetic
mlr --csv put '$days_old = (systime() - $created_at) / 86400' data.csv

# Format date
mlr --csv put '$formatted = strftime($ts, "%Y-%m-%d %H:%M:%S")' data.csv
```

## Statistical Functions
```bash
# Mean
mlr --csv stats1 -a mean -f salary data.csv

# Median
mlr --csv stats1 -a median -f age data.csv

# Percentiles
mlr --csv stats1 -a p25,p50,p75,p90 -f response_time data.csv

# Min/max
mlr --csv stats1 -a min,max -f price data.csv

# Standard deviation
mlr --csv stats1 -a stddev -f score data.csv
```

## Grouping and Aggregation
```bash
# Count by group
mlr --csv stats1 -a count -g status data.csv

# Sum by group
mlr --csv stats1 -a sum -f amount -g category data.csv

# Multiple aggregations
mlr --csv stats1 -a count,sum,mean -f amount -g category data.csv

# Nested grouping
mlr --csv stats1 -a mean -f salary -g department,level data.csv
```

## CI/CD Integration
```bash
# Validate CSV structure
mlr --csv cat data.csv > /dev/null || {
  echo "Invalid CSV"
  exit 1
}

# Check for required fields
if ! mlr --csv head -n 1 data.csv | grep -q 'required_field'; then
  echo "Missing required field"
  exit 1
fi

# Transform and validate
mlr --csv put '$total = $price * $quantity' data.csv | \
  mlr --csv filter '$total > 0' > output.csv

# Generate report
mlr --csv stats1 -a count,sum,mean -f amount -g status data.csv > report.csv
```

## Log Processing
```bash
# Parse logs
tail -f app.log | \
  mlr --ijson --ocsv cat

# Filter errors
mlr --json filter '$level == "ERROR"' logs.json

# Count by error type
mlr --json stats1 -a count -g error_type errors.json

# Time-based filtering
mlr --json filter '$timestamp > 1640000000' logs.json
```

## Data Cleaning
```bash
# Remove duplicates
mlr --csv uniq -a data.csv

# Fill nulls
mlr --csv put 'if ($age == null) { $age = 0 }' data.csv

# Trim whitespace
mlr --csv put '$name = strip($name)' data.csv

# Validate email format
mlr --csv filter '$email =~ "@.*\."' data.csv

# Remove invalid rows
mlr --csv filter '
  is_numeric($age) && $age >= 0 && $age <= 150
' data.csv
```

## Advanced Examples
```bash
# Top N by value
mlr --csv sort -nr amount data.csv | mlr --csv head -n 10

# Pivot table
mlr --csv reshape -s category,amount data.csv

# Running total
mlr --csv put '
  @running_total += $amount;
  $cumulative = @running_total
' data.csv

# Window functions (previous/next row)
mlr --csv step -a delta -f amount data.csv

# Deduplicate keeping first
mlr --csv uniq -g email data.csv

# Complex transformation
mlr --csv put '
  $profit = $revenue - $cost;
  $margin = ($profit / $revenue) * 100;
  $category = ($margin > 20) ? "high" : "low"
' data.csv | mlr --csv filter '$profit > 0'
```

## Format Options
```bash
# CSV options
mlr --csv --rs lf --fs comma data.csv

# Custom delimiter
mlr --fs '|' --rs lf cat data.psv

# No header
mlr --csv --implicit-csv-header cat data.csv

# JSON arrays
mlr --json --jvstack cat data.json

# Pretty JSON
mlr --ijson --ojson --jvstack --no-jvstack cat data.json
```

## Comparison
| Feature | miller | awk | csvkit | xsv |
|---------|--------|-----|--------|-----|
| Named fields | Yes | No | Yes | Yes |
| Multiple formats | Yes | No | Limited | No |
| Streaming | Yes | Yes | No | Yes |
| Speed | Fast | Fastest | Slow | Fastest |
| Syntax | DSL | AWK | CLI | CLI |

## Best Practices
- **Use --c2p** for quick data inspection
- **Chain operations** with pipes for complex workflows
- **Use stats1** for quick aggregations
- **Filter early** to reduce data size
- **Use put for transformations**, filter for selection
- **Test with head** before processing large files
- **Use --j2c** for JSON → CSV conversions

## Tips
- Handles CSV, TSV, JSON, NIDX seamlessly
- Named-field operations easier than awk
- Streaming (low memory for large files)
- Built-in stats functions
- Great for log analysis
- No external dependencies
- Cross-platform

## Agent Use
- Log file analysis (JSON logs)
- Data format conversion
- ETL preprocessing
- Real-time stream processing
- Data validation pipelines
- Report generation

## Uninstall
```yaml
- preset: miller
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/johnkerl/miller
- Docs: https://miller.readthedocs.io/
- Search: "miller mlr examples", "miller csv"
