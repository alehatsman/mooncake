# Nmap - Network Discovery and Security Auditing

Network exploration tool and security scanner for discovering hosts and services on a computer network.

## Quick Start

```yaml
- preset: nmap
```

## Features

- **Host discovery**: Find live hosts on a network
- **Port scanning**: Identify open ports and services
- **Version detection**: Determine application versions
- **OS detection**: Identify operating systems
- **Scriptable**: NSE (Nmap Scripting Engine) for advanced tasks
- **Flexible**: Dozens of scanning techniques
- **Output formats**: XML, JSON, and text output

## Basic Usage

```bash
# Scan single host
nmap 192.168.1.1

# Scan multiple hosts
nmap 192.168.1.1 192.168.1.2 192.168.1.3

# Scan IP range
nmap 192.168.1.1-254

# Scan subnet
nmap 192.168.1.0/24

# Scan specific ports
nmap -p 22,80,443 192.168.1.1

# Scan all ports
nmap -p- 192.168.1.1

# Fast scan (top 100 ports)
nmap -F 192.168.1.1

# Service version detection
nmap -sV 192.168.1.1

# OS detection
sudo nmap -O 192.168.1.1

# Aggressive scan (OS, version, script, traceroute)
sudo nmap -A 192.168.1.1
```

## Advanced Configuration

```yaml
# Install Nmap
- preset: nmap

# Run security audit
- name: Scan network for open ports
  shell: nmap -p 1-65535 -oX scan-results.xml 192.168.1.0/24
  register: scan_results

# Parse results and alert on dangerous ports
- name: Check for dangerous services
  shell: |
    nmap --script vuln 192.168.1.0/24 -oN vuln-scan.txt
  register: vuln_scan

# Generate report
- name: Convert to HTML report
  shell: xsltproc scan-results.xml -o scan-report.html
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Nmap |

## Platform Support

- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ✅ Windows (installer)

## Configuration

- **Data files**: `/usr/share/nmap/` (scripts, service fingerprints)
- **NSE scripts**: `/usr/share/nmap/scripts/`
- **User scripts**: `~/.nmap/`
- **Output formats**: Normal, XML, Grepable, Script kiddie

## Real-World Examples

### Network Discovery
```bash
# Find live hosts (ping scan)
nmap -sn 192.168.1.0/24

# Find hosts with specific port open
nmap -p 22 --open 192.168.1.0/24

# Scan using hostname
nmap example.com
```

### Security Audit
```bash
# Comprehensive security scan
sudo nmap -sS -sV -O -A --script vuln 192.168.1.0/24 -oA security-audit

# Check for SSL/TLS vulnerabilities
nmap --script ssl-enum-ciphers -p 443 example.com

# Test for common vulnerabilities
nmap --script vuln,exploit 192.168.1.1
```

### Service Enumeration
```bash
# Detect web server version
nmap -p 80,443 -sV --script http-headers example.com

# Enumerate SMB shares
nmap -p 445 --script smb-enum-shares 192.168.1.1

# Check database versions
nmap -p 3306,5432,1433 -sV 192.168.1.0/24
```

### Automated Scanning in CI/CD
```yaml
# Security scanning in deployment pipeline
- name: Install Nmap
  preset: nmap

- name: Scan deployed application
  shell: |
    nmap -p 80,443 -sV --script http-security-headers {{ app_domain }} \
      -oX /tmp/nmap-results.xml
  register: scan_output

- name: Parse scan results
  shell: |
    xmllint --xpath '//port[@portid="443"]/state/@state' /tmp/nmap-results.xml
  register: https_check

- name: Fail if HTTPS not available
  fail:
    msg: "HTTPS port 443 is not open"
  when: '"open" not in https_check.stdout'

- name: Check security headers
  shell: |
    nmap --script http-security-headers -p 443 {{ app_domain }} \
      | grep -i "x-frame-options"
  register: security_headers

- name: Alert if headers missing
  debug:
    msg: "Security headers may be misconfigured"
  when: security_headers.rc != 0
```

### Infrastructure Inventory
```yaml
# Create infrastructure inventory
- name: Discover network hosts
  shell: nmap -sn -oX /tmp/hosts.xml {{ network_range }}
  register: host_discovery

- name: Port scan discovered hosts
  shell: |
    nmap -iL /tmp/hosts.xml -p 22,80,443,3306,5432,6379,9200 \
      -sV -oA /tmp/inventory
  register: port_scan

- name: Generate inventory report
  shell: |
    nmap --stylesheet https://svn.nmap.org/nmap/docs/nmap.xsl \
      /tmp/inventory.xml > /tmp/inventory.html
```

### Firewall Testing
```bash
# Test firewall rules
# SYN scan (stealthy)
sudo nmap -sS -p 1-1000 firewall.example.com

# ACK scan (check filtering)
sudo nmap -sA -p 1-1000 firewall.example.com

# FIN scan (bypass some firewalls)
sudo nmap -sF -p 1-1000 firewall.example.com
```

## Common Scan Types

### TCP Connect Scan (-sT)
```bash
# Full TCP connection (no root needed)
nmap -sT 192.168.1.1
```

### SYN Scan (-sS)
```bash
# Stealth scan (requires root)
sudo nmap -sS 192.168.1.1
```

### UDP Scan (-sU)
```bash
# Scan UDP ports (requires root, slower)
sudo nmap -sU -p 53,161,123 192.168.1.1
```

### Comprehensive Scan
```bash
# Everything: TCP, UDP, OS, version, scripts
sudo nmap -sSU -T4 -A -v 192.168.1.1
```

## NSE Scripts

### Web Application Scanning
```bash
# HTTP methods allowed
nmap --script http-methods -p 80 example.com

# Common directories
nmap --script http-enum -p 80 example.com

# SQL injection check
nmap --script http-sql-injection -p 80 example.com

# WordPress scan
nmap --script http-wordpress-enum -p 80 blog.example.com
```

### SSL/TLS Testing
```bash
# Check certificate
nmap --script ssl-cert -p 443 example.com

# Test ciphers
nmap --script ssl-enum-ciphers -p 443 example.com

# Check for Heartbleed
nmap --script ssl-heartbleed -p 443 example.com
```

### Database Enumeration
```bash
# MySQL
nmap --script mysql-info,mysql-databases -p 3306 db.example.com

# PostgreSQL
nmap --script pgsql-brute -p 5432 db.example.com

# MongoDB
nmap --script mongodb-info -p 27017 db.example.com
```

## Performance Tuning

```bash
# Timing templates (-T0 to -T5)
nmap -T0 192.168.1.1  # Paranoid (slowest, evade IDS)
nmap -T3 192.168.1.1  # Normal (default)
nmap -T4 192.168.1.1  # Aggressive (faster)
nmap -T5 192.168.1.1  # Insane (fastest, may miss)

# Parallel scanning
nmap --min-parallelism 100 192.168.1.0/24

# Faster host discovery
nmap -n -sn -PE -PP -PS80,443 192.168.1.0/24
```

## Agent Use

- Automate network security audits and vulnerability assessments
- Discover and inventory infrastructure assets
- Validate firewall rules and network segmentation
- Monitor for unauthorized services and open ports
- Integrate security scanning into CI/CD pipelines
- Generate compliance reports for security standards

## Troubleshooting

### Permission denied errors
```bash
# Some scans require root (SYN, OS detection)
sudo nmap -sS -O 192.168.1.1

# Use TCP connect if no root
nmap -sT 192.168.1.1
```

### Scan too slow
```bash
# Increase timing
nmap -T4 192.168.1.1

# Scan fewer ports
nmap --top-ports 100 192.168.1.1

# Skip host discovery for known hosts
nmap -Pn 192.168.1.1
```

### Blocked by firewall
```bash
# Try different scan types
sudo nmap -sF 192.168.1.1  # FIN scan
sudo nmap -sX 192.168.1.1  # Xmas scan

# Fragment packets
sudo nmap -f 192.168.1.1

# Use decoys
sudo nmap -D RND:10 192.168.1.1
```

## Uninstall

```yaml
- preset: nmap
  with:
    state: absent
```

## Resources

- Official docs: https://nmap.org/book/
- NSE documentation: https://nmap.org/nsedoc/
- Reference guide: https://nmap.org/docs.html
- GitHub: https://github.com/nmap/nmap
- Search: "nmap tutorial", "nmap nse scripts", "nmap port scanning"
