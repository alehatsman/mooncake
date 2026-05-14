# mdl - Markdown Linter and Style Checker

A Ruby-based Markdown linter that checks Markdown documents for common style violations and enforces consistent formatting across your documentation.

## Quick Start

```yaml
- preset: mdl
```

## Features

- **Configurable rules**: Enable/disable specific linting rules to match your style guide
- **Multiple output formats**: Plain text, JSON-compatible output for CI/CD pipelines
- **CI/CD friendly**: Exit codes for automated validation in build pipelines
- **Fast scanning**: Quickly scan single files or entire documentation directories
- **Rule customization**: Create `.mdlrc` configuration files for project-specific rules
- **Compatible with Markdown**: Works with standard Markdown and GitHub Flavored Markdown (GFM)

## Basic Usage

```bash
# Check a single Markdown file
mdl README.md

# Check all Markdown files in a directory
mdl docs/

# Show all available rules
mdl --list-rules

# Check version
mdl --version

# Get help
mdl --help
```

## Advanced Configuration

```yaml
# Basic installation
- preset: mdl

# Install with custom version
- preset: mdl
  with:
    version: "0.12.0"

# Install with configuration
- preset: mdl
  with:
    state: present

# Uninstall
- preset: mdl
  with:
    state: absent
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove mdl |
| version | string | latest | Version to install (e.g., '0.12.0', 'latest') |

## Platform Support

- ✅ Linux (apt, dnf, pacman, requires Ruby/gem)
- ✅ macOS (Homebrew, gem)
- ❌ Windows (not directly supported)

## Configuration

- **Config file**: `.mdlrc` (project root or `~/.mdlrc`)
- **Rules**: All built-in rules documented in official repository
- **Style rules**: MD001-MD048 for various formatting checks
- **Rule format**: Enable/disable rules in `.mdlrc` or via command-line

### Common Configuration Example

```ruby
# .mdlrc - Project root configuration
rules do
  # Enable all rules by default
  all

  # Disable specific rules
  exclude :MD003  # Inconsistent indentation in lists
  exclude :MD013  # Line too long

  # Configure rule parameters
  rule :MD003, :style => :consistent
  rule :MD004, :style => :consistent
  rule :MD005
  rule :MD007, :indent => 2
end
```

## Real-World Examples

### Documentation Quality Checks

```bash
# Verify all documentation meets style standards
mdl docs/

# Create a linting report for CI/CD
mdl docs/ > lint-report.txt
```

### CI/CD Integration

```yaml
# Example: Adding mdl check to your pipeline
- preset: mdl

# Create a verification script
- name: Lint documentation
  shell: |
    mdl docs/
    if [ $? -eq 0 ]; then
      echo "Documentation passes style checks"
    else
      echo "Documentation has style violations"
      exit 1
    fi
```

### Project Setup

```bash
# 1. Install mdl
mdl

# 2. Generate initial .mdlrc in your project
echo 'rules do
  all
  exclude :MD003, :MD013
  rule :MD007, :indent => 2
end' > .mdlrc

# 3. Lint all documentation
mdl docs/

# 4. Fix violations and run again
mdl docs/
```

## Agent Use

- Validate documentation in CI/CD pipelines before deployment
- Parse linting output to identify documentation issues
- Enforce consistent Markdown formatting across documentation repositories
- Check pull requests for documentation style violations
- Generate reports of documentation quality metrics
- Integrate with documentation build systems to catch formatting errors early

## Troubleshooting

### "mdl: command not found"

Ensure mdl is installed and available in your PATH:

```bash
# Verify installation
command -v mdl

# Check version
mdl --version

# If not found, reinstall
gem install mdl  # or use Homebrew: brew install mdl
```

### "No such file or directory" errors

mdl needs actual Markdown files to lint:

```bash
# Verify target files exist
ls docs/*.md

# Check with current directory
mdl .

# Check specific file
mdl README.md
```

### Configuration not being loaded

Ensure `.mdlrc` is in the correct location:

```bash
# Check if .mdlrc exists in project root
ls -la .mdlrc

# Verify configuration syntax
cat .mdlrc

# Run with explicit config
mdl --config .mdlrc docs/
```

### Rules you want disabled are still triggering

Check your `.mdlrc` syntax:

```ruby
# Correct syntax
exclude :MD003, :MD013

# Incorrect syntax (missing colons)
exclude MD003, MD013
```

## Uninstall

```yaml
- preset: mdl
  with:
    state: absent
```

## Resources

- Official docs: https://github.com/markdownlint/markdownlint
- Rule reference: https://github.com/markdownlint/markdownlint/blob/master/docs/RULES.md
- Configuration guide: https://github.com/markdownlint/markdownlint#configuration
- Search: "mdl markdown linter", "markdownlint rules", "markdown style guide"
