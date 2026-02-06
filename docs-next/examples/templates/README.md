# 05 - Templates

Learn how to render configuration files from templates using pongo2 syntax.

## What You'll Learn

- Rendering `.j2` template files
- Using variables in templates
- Template conditionals (`{% if %}`)
- Template loops (`{% for %}`)
- Passing additional vars to templates

## Quick Start

```bash
mooncake run --config config.yml
```

Check the rendered files:
```bash
ls -lh /tmp/mooncake-templates/
cat /tmp/mooncake-templates/config.yml
```

## What It Does

1. Defines variables for application, server, and database config
2. Renders application config with loops and conditionals
3. Renders nginx config with optional SSL
4. Creates executable script from template
5. Renders same template with different variables

## Key Concepts

### Template Action

```yaml
- name: Render config
  template:
    src: ./templates/config.yml.j2
    dest: /tmp/config.yml
    mode: "0644"
```

### Template Syntax (pongo2)

**Variables:**
```jinja
{{ variable_name }}
{{ nested.property }}
```

**Conditionals:**
```jinja
{% if debug %}
  debug: true
{% else %}
  debug: false
{% endif %}
```

**Loops:**
```jinja
{% for item in items %}
  - {{ item }}
{% endfor %}
```

**Filters:**
```jinja
{{ path | expanduser }}  # Expands ~ to home directory
{{ text | upper }}       # Convert to uppercase
```

### Passing Additional Vars

Override variables for specific templates:
```yaml
- template:
    src: ./templates/config.yml.j2
    dest: /tmp/prod-config.yml
    vars:
      environment: production
      debug: false
```

## Template Files

### config.yml.j2
Application configuration with:
- Conditional debug settings
- Loops over features list
- Variable substitution

### nginx.conf.j2
Web server config with:
- Conditional SSL configuration
- Dynamic port and paths

### script.sh.j2
Executable shell script with:
- Shebang line
- Variable expansion
- Command loops

## Common Use Cases

- **Config files** - app.yml, nginx.conf, etc.
- **Shell scripts** - deployment scripts, setup scripts
- **Systemd units** - service files
- **Dotfiles** - .bashrc, .vimrc with customization

## Testing Templates

```bash
# Render templates
mooncake run --config config.yml

# View rendered output
cat /tmp/mooncake-templates/config.yml

# Check executable permissions
ls -la /tmp/mooncake-templates/deploy.sh
```

## Next Steps

→ Continue to [06-loops](../06-loops/) to learn about iterating over lists and files.
