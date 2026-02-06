# Miller - Structured Data Processing with Named Fields

Miller (mlr) is a command-line tool for processing CSV, JSON, TSV, and other structured data formats using named fields and a domain-specific language. Like awk, sed, and cut combined for modern data workflows.

## Quick Start

```yaml
- preset: miller
```

## Features

- **Format Conversion**: Seamlessly convert between CSV, JSON, TSV, NIDX, and custom delimited formats
- **Data Transformation**: Add computed fields, rename columns, and reshape data with inline expressions
- **Streaming Processing**: Process large files with constant memory usage
- **Named Field Operations**: Work with column names instead of positional indices (easier than awk)
- **Statistical Functions**: Built-in aggregation, grouping, and statistical operations (mean, median, percentiles)
- **Cross-Platform**: Available on Linux, macOS, and Windows via package managers

## Basic Usage

```bash
# View CSV file
mlr --csv cat data.csv

# Pretty print (aligned columns)
mlr --c2p cat data.csv

# Convert CSV to JSON
mlr --c2j cat data.csv

# Convert JSON to CSV
mlr --j2c cat data.json
```

## Advanced Configuration

```yaml
# Install with version control (optional)
- preset: miller
  with:
    state: present
```

Miller has no additional parameters beyond state. Configuration happens through command-line flags at runtime.

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install (present) or remove (absent) miller |

## Platform Support

- ✅ Linux (apt, dnf, pacman, yum)
- ✅ macOS (Homebrew)
- ✅ Windows (via package managers or official binary)

## Real-World Examples

### ETL Data Pipeline - CSV to JSON Conversion

Process raw CSV export from database and convert to JSON for API:

```bash
# Transform employee data: validate, enrich, convert
mlr --csv put '
  $full_name = $first_name . " " . $last_name;
  $start_year = substr($hire_date, 0, 4);
  $is_manager = ($title =~ "Manager" || $title =~ "Lead") ? true : false
' employees.csv | \
mlr --csv filter '$email != null && $salary > 0' | \
mlr --c2j > employees.json
```

### Log Analysis - Filter and Aggregate

Process application JSON logs to identify issues:

```bash
# Count errors by type and find top 5
mlr --json stats1 -a count -g error_type logs.json | \
mlr --json sort -nr count | \
mlr --json head -n 5
```

### Data Quality Validation

Validate structured data before processing:

```bash
# Check for required fields, remove invalid rows
mlr --csv filter '
  $id != null &&
  is_numeric($age) && $age >= 0 && $age <= 150 &&
  $email =~ "@.*\." &&
  $status == "active"
' raw_data.csv | \
mlr --csv stats1 -a count data_validated.csv
```

### CI/CD Integration - Validate Configuration Changes

```bash
# Validate CSV structure and required columns
mlr --csv cat config.csv > /dev/null || {
  echo "ERROR: Invalid CSV structure"
  exit 1
}

# Ensure all required fields present
FIELDS=$(mlr --csv head -n 1 config.csv | tr ',' '\n')
for field in "server" "port" "enabled"; do
  if ! echo "$FIELDS" | grep -q "^$field$"; then
    echo "ERROR: Missing required field: $field"
    exit 1
  fi
done

echo "✓ Configuration validated"
```

### Aggregation - Sales Report Generation

```bash
# Generate sales summary by region and product
mlr --csv stats1 -a count,sum,mean -f amount -g region,product sales.csv | \
mlr --csv sort -nr count > sales_report.csv

# Show top products by revenue
mlr --csv put '$revenue = $amount * $quantity' sales.csv | \
mlr --csv stats1 -a sum -f revenue -g product | \
mlr --csv sort -nr revenue_sum | \
mlr --csv head -n 10
```

### Stream Processing - Real-Time Log Transformation

```bash
# Monitor logs and convert to structured CSV
tail -f app.log | \
mlr --ijson --ocsv put '
  $severity = toupper($level);
  $timestamp = strftime($ts, "%Y-%m-%d %H:%M:%S")
' | tee processed_logs.csv
```

### Data Cleaning - Handle Missing Values

```bash
# Remove duplicates, fill nulls, trim whitespace
mlr --csv put '
  if ($age == null) { $age = 0 };
  $name = strip($name);
  $email = tolower($email)
' raw.csv | \
mlr --csv uniq -g email > cleaned.csv
```

## Agent Use

AI agents can leverage Miller for:

- **Log Analysis Pipelines**: Parse JSON logs, filter errors, aggregate by type/severity, and generate alerts
- **Data Format Transformation**: Convert between CSV/JSON/TSV in ETL workflows without external dependencies
- **Validation Automation**: Check data quality (required fields, type validation, format constraints) in deployment pipelines
- **Report Generation**: Group, aggregate, and transform data into formatted reports for monitoring systems
- **Real-Time Stream Processing**: Monitor file streams, transform structured data, and pipeline to downstream systems
- **Configuration Validation**: Verify CSV/JSON configs have required fields and valid values before deployment

## Troubleshooting

### "mlr: command not found"

Miller is not installed. Run the preset with `state: present` to install.

### Performance Issues with Large Files

Miller streams by default (constant memory), but some operations buffer. For large files:

```bash
# Use head to test on sample first
mlr --csv head -n 1000 huge_file.csv | mlr --csv filter '$condition'

# Use tee to save intermediate results
mlr --csv filter '$condition1' data.csv | \
tee intermediate.csv | \
mlr --csv stats1 -a sum -f amount -g category
```

### Escaping Special Characters in DSL

Miller expressions use DSL (Domain-Specific Language). Escape backslashes and quotes:

```bash
# Regex with backslash - needs double backslash in shell
mlr --csv filter '$email =~ "@example\\.com$"' data.csv

# Single quotes within expression - use double quotes for outer shell
mlr --csv put "$field = 'value'" data.csv
```

### JSON Parsing Errors

Ensure JSON is valid before processing:

```bash
# Validate JSON first
jq empty logs.json || {
  echo "ERROR: Invalid JSON"
  exit 1
}

mlr --json cat logs.json
```

## Uninstall

```yaml
- preset: miller
  with:
    state: absent
```

## Resources

- **Official Documentation**: https://miller.readthedocs.io/
- **GitHub Repository**: https://github.com/johnkerl/miller
- **Interactive Tutorial**: https://miller.readthedocs.io/en/latest/getting-started.html
- **Search Terms**: "miller mlr tutorial", "miller CSV processing", "miller data transformation", "mlr DSL examples"
