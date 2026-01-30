# Mooncake Examples

This directory contains example configurations demonstrating various Mooncake features.

## Basic Examples

### Hello World
```bash
mooncake run --config basic/hello.yml
```
Simple example showing shell commands and global variables.

### Files and Templates
```bash
mooncake run --config basic/files-and-templates.yml
```
Demonstrates creating files and directories with different permissions.

### Conditionals and Tags
```bash
# Run all steps
mooncake run --config basic/conditionals-and-tags.yml

# Run only dev-tagged steps
mooncake run --config basic/conditionals-and-tags.yml --tags dev

# Run production steps
mooncake run --config basic/conditionals-and-tags.yml --tags prod
```
Shows conditional execution with `when` and filtering with tags.

## Advanced Examples

### Multi-file Configuration
```bash
# Development environment
mooncake run --config advanced/main.yml

# Production environment (requires modifying vars in main.yml)
# Change: env: production
mooncake run --config advanced/main.yml

# Run only installation steps
mooncake run --config advanced/main.yml --tags install

# Run only dev tools setup
mooncake run --config advanced/main.yml --tags dev
```
Demonstrates:
- Loading variables from external files with `include_vars`
- Including other configuration files with `include`
- Organizing configuration into multiple files
- Conditional includes based on OS
- Combining `when` conditions with tags

## Running Examples

All examples can be run from the project root:
```bash
# Basic examples
mooncake run --config examples/basic/hello.yml
mooncake run --config examples/basic/files-and-templates.yml
mooncake run --config examples/basic/conditionals-and-tags.yml

# Advanced example
mooncake run --config examples/advanced/main.yml

# With tags filter
mooncake run --config examples/basic/conditionals-and-tags.yml --tags dev

# With debug logging
mooncake run --config examples/basic/hello.yml --log-level debug
```
