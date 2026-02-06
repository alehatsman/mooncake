# vegeta - HTTP Load Tester

HTTP load testing tool and library with constant throughput.

## Quick Start
```yaml
- preset: vegeta
```

## Usage
```bash
# Attack for 30s at 100 req/s
echo "GET http://localhost:8080" | vegeta attack -duration=30s -rate=100 | tee results.bin | vegeta report

# Complex scenario
cat targets.txt | vegeta attack -duration=10s -rate=50 | vegeta report -type=json

# Plot
vegeta plot results.bin > plot.html
```

## Resources
GitHub: https://github.com/tsenart/vegeta
