# 08 - Tags

Learn how to use tags to selectively run parts of your configuration.

## What You'll Learn

- Adding tags to steps
- Filtering execution with `--tags` flag
- Organizing workflows with tags
- Combining tags with conditionals

## Quick Start

```bash
cd examples/08-tags

# Run all steps (no tag filter)
mooncake run --config config.yml

# Run only development steps
mooncake run --config config.yml --tags dev

# Run only production steps
mooncake run --config config.yml --tags prod

# Run test-related steps
mooncake run --config config.yml --tags test

# Run multiple tag categories
mooncake run --config config.yml --tags dev,test
```

## What It Does

Demonstrates different tagged workflows:
- Development setup
- Production deployment
- Testing
- Security audits
- Staging deployment

## Key Concepts

### Adding Tags

```yaml
- name: Install dev tools
  shell: echo "Installing tools"
  tags:
    - dev
    - tools
```

### Tag Filtering Behavior

**No tags specified:**
- All steps run (including untagged steps)

**Tags specified (`--tags dev`):**
- Only steps with matching tags run
- Untagged steps are skipped

**Multiple tags (`--tags dev,prod`):**
- Steps run if they have ANY of the specified tags
- OR logic: matches `dev` OR `prod`

### Tag Organization Strategies

**By Environment:**
```yaml
tags: [dev, staging, prod]
```

**By Phase:**
```yaml
tags: [setup, deploy, test, cleanup]
```

**By Component:**
```yaml
tags: [database, webserver, cache]
```

**By Role:**
```yaml
tags: [install, configure, security]
```

### Multiple Tags Per Step

Steps can have multiple tags:
```yaml
- name: Security audit
  shell: run-security-scan
  tags:
    - test
    - prod
    - security
```

This runs with:
- `--tags test` ✓
- `--tags prod` ✓
- `--tags security` ✓
- `--tags dev` ✗

## Real-World Examples

### Development Workflow

```bash
# Install dev tools only
mooncake run --config config.yml --tags dev,tools

# Run tests
mooncake run --config config.yml --tags test
```

### Production Deployment

```bash
# Deploy to production
mooncake run --config config.yml --tags prod,deploy

# Run security checks
mooncake run --config config.yml --tags security,prod
```

### Staging Environment

```bash
# Deploy to staging
mooncake run --config config.yml --tags staging,deploy
```

## Combining Tags and Conditionals

```yaml
- name: Install Linux dev tools
  shell: apt install build-essential
  become: true
  when: os == "linux"
  tags:
    - dev
    - tools
```

Both must match:
1. Condition must be true (`os == "linux"`)
2. Tag must match (if `--tags` specified)

## Testing Different Tag Filters

```bash
# Preview what runs with dev tag
mooncake run --config config.yml --tags dev --dry-run

# Run dev and test steps
mooncake run --config config.yml --tags dev,test

# Run only setup steps
mooncake run --config config.yml --tags setup
```

## Best Practices

1. **Use consistent naming** - Pick a scheme (env, phase, role) and stick to it
2. **Multiple tags per step** - Makes filtering more flexible
3. **Document your tags** - In README or comments
4. **Combine with conditionals** - For environment + OS filtering

## Next Steps

Continue to [09-sudo](09-sudo.md) to learn about privilege escalation.
