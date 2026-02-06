# dog - Modern DNS Client

Command-line DNS client with colorful output and DoT/DoH support.

## Quick Start
```yaml
- preset: dog
```

## Features
- **Colored output**: Easy-to-read, syntax-highlighted DNS responses
- **DNS over TLS (DoT)**: Encrypted DNS queries over port 853
- **DNS over HTTPS (DoH)**: DNS queries via HTTPS
- **JSON output**: Machine-readable output format
- **Multiple record types**: A, AAAA, MX, TXT, NS, SOA, and more
- **Fast**: Written in Rust for performance

## Basic Usage
```bash
# Query A record
dog example.com

# Query specific record type
dog example.com A
dog example.com AAAA
dog example.com MX
dog example.com TXT
dog example.com NS

# Query all record types
dog example.com ANY

# Use specific DNS server
dog example.com @8.8.8.8
dog example.com @1.1.1.1

# Query multiple domains
dog example.com google.com github.com
```

## Advanced Queries
```bash
# DNS over HTTPS (DoH)
dog example.com --https @https://dns.google/dns-query
dog example.com --https @https://cloudflare-dns.com/dns-query

# DNS over TLS (DoT)
dog example.com --tls @dns.google

# Short output (no colors, just results)
dog example.com --short

# JSON output
dog example.com --json

# Reverse DNS lookup
dog -x 8.8.8.8

# Query with timeout
dog example.com --timeout 5s

# TCP instead of UDP
dog example.com --tcp
```

## Record Types
```bash
# IPv4 address
dog example.com A

# IPv6 address
dog example.com AAAA

# Mail servers
dog example.com MX

# Name servers
dog example.com NS

# Text records
dog example.com TXT

# Start of authority
dog example.com SOA

# Canonical name
dog example.com CNAME

# Service records
dog _service._tcp.example.com SRV

# Certificate records
dog example.com CERT

# Host info
dog example.com HINFO
```

## Advanced Configuration
```yaml
# Install dog
- preset: dog

# Uninstall
- preset: dog
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, pacman, zypper, apk, Homebrew)
- ✅ macOS (Homebrew)
- ❌ Windows

## Real-World Examples

### Debug DNS Issues
```bash
# Check if domain resolves
dog example.com

# Compare different DNS servers
dog example.com @8.8.8.8          # Google DNS
dog example.com @1.1.1.1          # Cloudflare DNS
dog example.com @208.67.222.222   # OpenDNS

# Check DNS propagation
dog newdomain.com @8.8.8.8
dog newdomain.com @1.1.1.1
```

### Verify Email Configuration
```bash
# Check MX records
dog example.com MX

# Verify SPF record
dog example.com TXT | grep spf

# Check DMARC policy
dog _dmarc.example.com TXT

# Verify DKIM
dog default._domainkey.example.com TXT
```

### Security Analysis
```bash
# Check DNSSEC
dog example.com --dnssec

# Use secure DNS
dog example.com --tls @dns.google
dog example.com --https @https://cloudflare-dns.com/dns-query

# Verify CAA records
dog example.com CAA
```

### CI/CD Integration
```bash
# Verify DNS before deployment
if dog staging.example.com --short | grep -q "192.168.1.1"; then
  echo "DNS configured correctly"
  ./deploy.sh
else
  echo "ERROR: DNS not propagated"
  exit 1
fi

# JSON output for parsing
dog api.example.com --json | jq '.answers[0].data'
```

## Output Format

### Standard Output
```
A example.com. 300 IN 93.184.216.34
```

### Detailed Output
```
;; QUESTION SECTION:
;; example.com.   IN   A

;; ANSWER SECTION:
example.com.   86400   IN   A   93.184.216.34

;; Query time: 23ms
;; Server: 8.8.8.8:53
;; Size: 54 bytes
```

### JSON Output
```json
{
  "responses": [{
    "queries": [{"name": "example.com", "class": "IN", "type": "A"}],
    "answers": [{
      "name": "example.com",
      "class": "IN",
      "ttl": 86400,
      "type": "A",
      "data": "93.184.216.34"
    }]
  }]
}
```

## Comparison with dig

| Feature | dog | dig |
|---------|-----|-----|
| Colored output | ✅ | ❌ |
| JSON output | ✅ | ❌ |
| DoH support | ✅ | ❌ |
| DoT support | ✅ | ❌ |
| Installation | Simple (single binary) | Often pre-installed |
| Learning curve | Easier | Steeper |

## DNS Server Options
```bash
# Popular public DNS servers
dog example.com @8.8.8.8          # Google
dog example.com @1.1.1.1          # Cloudflare
dog example.com @9.9.9.9          # Quad9
dog example.com @208.67.222.222   # OpenDNS
dog example.com @8.26.56.26       # Comodo Secure DNS

# DoH endpoints
dog example.com --https @https://dns.google/dns-query
dog example.com --https @https://cloudflare-dns.com/dns-query
dog example.com --https @https://dns.quad9.net/dns-query

# DoT endpoints
dog example.com --tls @dns.google
dog example.com --tls @cloudflare-dns.com
dog example.com --tls @dns.quad9.net
```

## Scripting Examples
```bash
# Get IP address only
IP=$(dog example.com --short)
echo "IP: $IP"

# Check if domain exists
if dog nonexistent.example.com 2>&1 | grep -q "NXDOMAIN"; then
  echo "Domain does not exist"
fi

# Parse JSON output
dog example.com --json | jq -r '.responses[0].answers[0].data'

# Batch queries
while IFS= read -r domain; do
  echo "Querying $domain"
  dog "$domain" --short
done < domains.txt
```

## Agent Use
- Verify DNS configuration in deployment pipelines
- Monitor DNS propagation
- Validate email infrastructure (MX, SPF, DKIM)
- Security audits (DNSSEC, CAA records)
- Troubleshoot DNS issues
- Parse DNS responses programmatically

## Troubleshooting

### No response
```bash
# Try different server
dog example.com @1.1.1.1

# Use TCP instead of UDP
dog example.com --tcp

# Increase timeout
dog example.com --timeout 10s
```

### DNSSEC validation failed
```bash
# Check DNSSEC status
dog example.com --dnssec

# Query without DNSSEC
dog example.com
```

## Uninstall
```yaml
- preset: dog
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/ogham/dog
- DNS over HTTPS: https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/
- Search: "dog dns client", "dns over https"
