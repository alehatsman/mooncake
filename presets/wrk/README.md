# wrk - HTTP Benchmarking

Modern HTTP benchmarking tool with Lua scripting support.

## Quick Start
```yaml
- preset: wrk
```

## Usage
```bash
# Basic benchmark
wrk -t12 -c400 -d30s http://localhost:8080

# With script
wrk -t12 -c400 -d30s -s script.lua http://localhost:8080

# POST request
wrk -t4 -c100 -d30s -s post.lua http://localhost:8080/api
```

## Resources
GitHub: https://github.com/wg/wrk
