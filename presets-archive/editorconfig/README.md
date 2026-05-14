# EditorConfig - Code Style Consistency Tool

Maintain consistent coding styles across different editors and IDEs using a single `.editorconfig` file.

## Quick Start
```yaml
- preset: editorconfig
```

## Features
- **Universal**: Works with most editors and IDEs
- **Simple**: One `.editorconfig` file per project
- **Automatic**: Editors apply settings automatically
- **Team-friendly**: Share coding style across team members
- **Language-agnostic**: Works with any file type
- **Override support**: Project-specific and directory-specific rules

## Basic Usage
```bash
# EditorConfig is a file format, not a command-line tool
# Create .editorconfig in your project root

cat > .editorconfig << 'EOF'
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
indent_style = space
indent_size = 2

[*.py]
indent_size = 4

[Makefile]
indent_style = tab

[*.md]
trim_trailing_whitespace = false
EOF
```

## Advanced Configuration
```yaml
# Install editorconfig-checker (validation tool)
- preset: editorconfig

# Uninstall
- preset: editorconfig
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ Windows

## Configuration Examples

### General Project
```ini
# .editorconfig
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.{js,jsx,ts,tsx}]
indent_size = 2

[*.{py,rb}]
indent_size = 4

[*.go]
indent_style = tab

[*.md]
trim_trailing_whitespace = false
max_line_length = off

[Makefile]
indent_style = tab
```

### Web Project
```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
indent_style = space

[*.{html,css,scss,sass}]
indent_size = 2

[*.{js,jsx,ts,tsx,vue}]
indent_size = 2
quote_type = single

[*.json]
indent_size = 2

[package.json,package-lock.json]
indent_size = 2
```

### Python Project
```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.py]
indent_style = space
indent_size = 4
max_line_length = 88

[*.{yml,yaml}]
indent_size = 2

[Makefile]
indent_style = tab
```

### JavaScript/TypeScript Project
```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.{js,jsx,ts,tsx}]
indent_style = space
indent_size = 2
quote_type = single

[*.{json,jsonc}]
indent_size = 2

[*.md]
trim_trailing_whitespace = false
max_line_length = off
```

## Properties Reference

### Indentation
```ini
indent_style = space | tab
indent_size = number      # Width of single indent
tab_width = number        # Width of tab character (defaults to indent_size)
```

### Line Endings
```ini
end_of_line = lf | cr | crlf
```

### Character Set
```ini
charset = utf-8 | utf-8-bom | latin1 | utf-16be | utf-16le
```

### Whitespace
```ini
trim_trailing_whitespace = true | false
insert_final_newline = true | false
```

### Line Length
```ini
max_line_length = number | off
```

## Editor Support
- ✅ **VS Code**: Built-in support
- ✅ **JetBrains IDEs**: Built-in support (IntelliJ, PyCharm, WebStorm, etc.)
- ✅ **Sublime Text**: Plugin required
- ✅ **Vim/Neovim**: Plugin required (editorconfig-vim)
- ✅ **Emacs**: Plugin required
- ✅ **Atom**: Built-in support
- ✅ **Visual Studio**: Built-in support (2017+)

## Real-World Examples

### Monorepo with Mixed Languages
```ini
root = true

# Defaults for all files
[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

# JavaScript/TypeScript (frontend)
[apps/web/**/*.{js,jsx,ts,tsx}]
indent_style = space
indent_size = 2

# Python (backend)
[apps/api/**/*.py]
indent_style = space
indent_size = 4
max_line_length = 88

# Go (microservices)
[services/**/*.go]
indent_style = tab
tab_width = 4

# YAML (configs)
[*.{yml,yaml}]
indent_size = 2

# Markdown (docs)
[*.md]
trim_trailing_whitespace = false
max_line_length = off
```

### Docker Project
```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[Dockerfile*]
indent_size = 4

[*.sh]
indent_size = 2

[docker-compose*.yml]
indent_size = 2
```

## Validation

### Install EditorConfig Checker
```bash
# This preset installs editorconfig-checker
# Validates files against .editorconfig rules
editorconfig-checker --version
```

### Check Files
```bash
# Check all files
editorconfig-checker

# Check specific files
editorconfig-checker file1.js file2.py

# Ignore files
editorconfig-checker --exclude node_modules/

# Show details
editorconfig-checker --verbose
```

## CI/CD Integration

### GitHub Actions
```yaml
name: EditorConfig Check
on: [push, pull_request]

jobs:
  editorconfig:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Install editorconfig-checker
        run: |
          curl -sSL https://github.com/editorconfig-checker/editorconfig-checker/releases/download/2.7.0/ec-linux-amd64.tar.gz | tar -xz
          sudo mv bin/ec-linux-amd64 /usr/local/bin/editorconfig-checker
      - name: Check EditorConfig
        run: editorconfig-checker
```

### Pre-commit Hook
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/editorconfig-checker/editorconfig-checker.python
    rev: 2.7.0
    hooks:
      - id: editorconfig-checker
        alias: ec
```

## Best Practices

### Project Structure
```
project/
├── .editorconfig          # Root config
├── src/
│   └── .editorconfig      # Subdirectory overrides (optional)
└── docs/
    └── .editorconfig      # Subdirectory overrides (optional)
```

### Common Patterns
```ini
# Root .editorconfig
root = true

# Defaults apply to all files
[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

# Language-specific rules
[*.{js,ts}]
indent_style = space
indent_size = 2

# File-specific rules
[package.json]
indent_size = 2

# Pattern matching
[*.min.js]
insert_final_newline = false
```

## Troubleshooting

### Editor not applying settings
```bash
# Check if editor has EditorConfig support
# Most modern editors have built-in support or plugins

# VS Code: Should work automatically
# If not, install "EditorConfig for VS Code" extension

# Vim: Install editorconfig-vim plugin
# Add to .vimrc:
# Plug 'editorconfig/editorconfig-vim'
```

### Rules not working
```bash
# Validate .editorconfig syntax
editorconfig-checker --verbose

# Check file matches pattern
# Pattern [*.js] matches all .js files recursively
# Pattern [src/*.js] matches .js files only in src/
```

### Conflicting with formatters
```ini
# EditorConfig should be applied first
# Then formatters like Prettier respect EditorConfig

# .editorconfig
[*.js]
indent_size = 2
max_line_length = 80

# Prettier will respect these settings
```

## Integration with Tools

### Prettier
```json
{
  "editorconfig": true
}
```

### ESLint
```json
{
  "extends": ["plugin:editorconfig/all"]
}
```

### Stylelint
```json
{
  "extends": ["stylelint-config-standard"],
  "plugins": ["stylelint-editorconfig"]
}
```

## Agent Use
- Enforce consistent code formatting across projects
- Standardize coding styles for teams
- Automate style configuration for new projects
- Ensure consistency in multi-language codebases
- Prepare repositories for linting and formatting
- Validate code style in CI/CD pipelines

## Common File Patterns
```ini
[*]                         # All files
[*.py]                      # All Python files
[*.{js,ts}]                 # JavaScript and TypeScript
[src/**/*.js]               # JS files in src/ recursively
[{package,bower}.json]      # Specific filenames
[Makefile]                  # Exact filename
[lib/**.js]                 # JS files two levels deep
```

## Uninstall
```yaml
- preset: editorconfig
  with:
    state: absent
```

**Note:** Removes `editorconfig-checker` tool. `.editorconfig` files in projects are preserved.

## Resources
- Official site: https://editorconfig.org/
- Properties: https://editorconfig.org/#supported-properties
- Editor plugins: https://editorconfig.org/#download
- GitHub: https://github.com/editorconfig/editorconfig
- Checker tool: https://github.com/editorconfig-checker/editorconfig-checker
- Search: "editorconfig examples", "editorconfig setup", "editorconfig vs prettier"
