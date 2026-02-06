# fx - Interactive JSON Viewer

Terminal JSON viewer with JavaScript expressions and interactive mode.

## Quick Start
```yaml
- preset: fx
```

## Usage
```bash
# Interactive
echo '{"name":"John","age":30}' | fx

# JavaScript expressions
echo '[1,2,3]' | fx 'x => x.map(n => n * 2)'

# Object access
cat data.json | fx .users[0].name
```

## Resources
GitHub: https://github.com/antonmedv/fx
