# gron - Make JSON Greppable

Transform JSON into discrete assignments to make it greppable.

## Quick Start
```yaml
- preset: gron
```

## Usage
```bash
# Make greppable
gron data.json | grep "email"

# Ungron back to JSON
gron data.json | grep "active" | gron --ungron
```

## Resources
GitHub: https://github.com/tomnomnom/gron
