# httpstat - curl Statistics

Visualize curl statistics: DNS lookup, TCP connection, TLS handshake, etc.

## Quick Start
```yaml
- preset: httpstat
```

## Usage
```bash
httpstat https://example.com
httpstat -X POST -d '{"key":"value"}' https://api.example.com
```

## Output Shows
- DNS Lookup
- TCP Connection  
- TLS Handshake
- Server Processing
- Content Transfer

## Resources
GitHub: https://github.com/reorx/httpstat
