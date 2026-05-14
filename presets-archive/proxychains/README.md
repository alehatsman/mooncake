# proxychains-ng - Force TCP Through Proxy

Route any TCP connection through SOCKS4, SOCKS5, or HTTP proxies. Essential for network testing, privacy, and accessing geo-restricted content.

## Quick Start
```yaml
- preset: proxychains
```

## Features
- **Force proxy**: Route any program through proxies
- **Multiple proxies**: Chain multiple proxies together
- **Dynamic chain**: Auto-skip dead proxies
- **DNS proxy**: Prevent DNS leaks
- **Random chain**: Random proxy selection
- **Cross-platform**: Linux, macOS, BSD

## Basic Usage
```bash
# Run command through proxy
proxychains curl https://api.ipify.org
proxychains wget https://example.com
proxychains ssh user@remote-host
proxychains git clone https://github.com/user/repo

# Check your IP
proxychains curl https://ifconfig.me

# Run interactive session
proxychains bash
proxychains zsh
```

## Advanced Configuration

### Single SOCKS5 proxy
```yaml
- name: Install proxychains
  preset: proxychains
  become: true

- name: Configure SOCKS5 proxy
  copy:
    dest: /etc/proxychains.conf
    content: |
      strict_chain
      proxy_dns
      tcp_read_time_out 15000
      tcp_connect_time_out 8000
      [ProxyList]
      socks5 127.0.0.1 1080
```

### Multiple proxy chain
```yaml
- name: Configure proxy chain
  copy:
    dest: ~/.proxychains/proxychains.conf
    content: |
      strict_chain
      proxy_dns
      [ProxyList]
      socks5 proxy1.example.com 1080
      socks5 proxy2.example.com 1080
      socks5 proxy3.example.com 1080
```

### Dynamic chain with fallback
```yaml
- name: Configure dynamic chain
  template:
    dest: /etc/proxychains.conf
    content: |
      dynamic_chain
      proxy_dns
      tcp_read_time_out 15000
      tcp_connect_time_out 8000
      [ProxyList]
      socks5 {{ proxy1_host }} {{ proxy1_port }}
      socks5 {{ proxy2_host }} {{ proxy2_port }}
      http {{ proxy3_host }} {{ proxy3_port }}
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove proxychains |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper)
- ✅ macOS (Homebrew)
- ✅ BSD (pkg)
- ❌ Windows (not supported)

## Configuration
- **Config file**: `/etc/proxychains.conf` (system), `~/.proxychains/proxychains.conf` (user)
- **Chain modes**: strict, dynamic, random
- **Proxy types**: SOCKS4, SOCKS5, HTTP
- **DNS**: Proxy DNS to prevent leaks

## Real-World Examples

### Development behind corporate proxy
```bash
# Create user config
mkdir -p ~/.proxychains
cat > ~/.proxychains/proxychains.conf <<EOF
strict_chain
proxy_dns
[ProxyList]
http corporate-proxy.company.com 8080
EOF

# Install packages through proxy
proxychains pip install requests
proxychains npm install express
proxychains apt-get update
```

### Tor integration
```yaml
- name: Install proxychains
  preset: proxychains
  become: true

- name: Configure Tor proxy
  copy:
    dest: /etc/proxychains.conf
    content: |
      strict_chain
      proxy_dns
      [ProxyList]
      socks5 127.0.0.1 9050
```

```bash
# Start Tor service
sudo systemctl start tor

# Use through Tor
proxychains curl https://check.torproject.org
proxychains firefox
```

### CI/CD through proxy
```yaml
- name: Install proxychains
  preset: proxychains
  become: true

- name: Configure proxy
  copy:
    dest: /etc/proxychains.conf
    content: |
      strict_chain
      proxy_dns
      [ProxyList]
      socks5 {{ ci_proxy_host }} {{ ci_proxy_port }}

- name: Run tests through proxy
  shell: proxychains pytest tests/
  cwd: /app
```

## Configuration File

### /etc/proxychains.conf
```ini
# Chain type: strict, dynamic, random
strict_chain
# dynamic_chain
# random_chain

# Quiet mode (no output)
# quiet_mode

# Proxy DNS requests
proxy_dns

# TCP timeouts (milliseconds)
tcp_read_time_out 15000
tcp_connect_time_out 8000

# Localnet exclusions (don't proxy local traffic)
localnet 127.0.0.0/255.0.0.0
localnet 10.0.0.0/255.0.0.0
localnet 172.16.0.0/255.240.0.0
localnet 192.168.0.0/255.255.0.0

[ProxyList]
# Format: type host port [username] [password]
socks5 127.0.0.1 1080
# socks4 proxy.example.com 1080
# http proxy.example.com 8080
# http proxy.example.com 8080 username password
```

## Chain Modes

### Strict chain
All proxies must be online and working. Connection fails if any proxy is dead.
```ini
strict_chain
[ProxyList]
socks5 proxy1.com 1080
socks5 proxy2.com 1080
```

### Dynamic chain
Auto-skips dead proxies. Connection succeeds as long as one proxy works.
```ini
dynamic_chain
[ProxyList]
socks5 proxy1.com 1080
socks5 proxy2.com 1080
socks5 proxy3.com 1080
```

### Random chain
Randomly selects N proxies from list.
```ini
random_chain
chain_len = 2
[ProxyList]
socks5 proxy1.com 1080
socks5 proxy2.com 1080
socks5 proxy3.com 1080
socks5 proxy4.com 1080
```

## Proxy Types

### SOCKS5 (recommended)
```ini
socks5 127.0.0.1 1080
socks5 proxy.example.com 1080 username password
```

### SOCKS4
```ini
socks4 proxy.example.com 1080
```

### HTTP/HTTPS
```ini
http proxy.example.com 8080
http proxy.example.com 8080 username password
```

## Common Use Cases

### SSH through proxy
```bash
proxychains ssh user@remote-server
proxychains ssh -D 8080 user@jump-host
```

### Git operations
```bash
proxychains git clone https://github.com/user/repo
proxychains git push origin main
proxychains git pull
```

### Package managers
```bash
proxychains apt-get update
proxychains yum update
proxychains brew install package
proxychains pip install requests
```

### Web browsers
```bash
proxychains firefox
proxychains chromium
```

### Network tools
```bash
proxychains nmap -sT target.com
proxychains curl -I https://example.com
proxychains wget https://example.com/file.zip
```

## Environment Variables
```bash
# Use custom config
export PROXYCHAINS_CONF_FILE=~/.proxychains/custom.conf

# Quiet mode
export PROXYCHAINS_QUIET_MODE=1

# Force SOCKS5 username
export PROXYCHAINS_SOCKS5_USER=username
export PROXYCHAINS_SOCKS5_PASS=password
```

## DNS Leak Prevention
```ini
# Enable proxy_dns to prevent DNS leaks
proxy_dns

# Test for leaks
proxychains dig +short myip.opendns.com @resolver1.opendns.com
```

## Agent Use
- Route automated tools through corporate proxies
- CI/CD pipelines behind firewalls
- Web scraping with IP rotation
- Security testing through anonymizing networks
- Geo-restricted content access for testing
- Development behind restrictive networks
- Automated penetration testing

## Troubleshooting

### Connection timeout
```ini
# Increase timeouts
tcp_read_time_out 30000
tcp_connect_time_out 15000
```

### DNS not working
```ini
# Enable proxy_dns
proxy_dns

# Or use remote_dns_subnet
remote_dns_subnet 224
```

### Local connections failing
```ini
# Add local network exclusions
localnet 127.0.0.0/255.0.0.0
localnet 192.168.0.0/255.255.0.0
```

### Proxy authentication failing
```ini
# Add credentials to proxy line
socks5 proxy.com 1080 myuser mypass
```

### Programs not being proxied
```bash
# Use proxyresolv for DNS
proxyresolv hostname

# Check if program uses UDP (not supported)
# proxychains only works with TCP connections
```

## Limitations
- **TCP only**: UDP connections not supported
- **Static binaries**: Won't work with statically linked programs
- **ICMP**: ping and similar tools won't work
- **DNS leaks**: Requires proxy_dns configuration
- **Performance**: Adds latency to connections

## Alternatives
- **torsocks**: Specifically for Tor
- **tsocks**: Older transparent proxy
- **redsocks**: Transparent TCP to SOCKS5 redirector
- **dante**: Full-featured SOCKS server/client
- **ssh -D**: Built-in SOCKS5 proxy

## Security Considerations
- **Trust**: Only use trusted proxy servers
- **Encryption**: Proxies can see unencrypted traffic
- **Logging**: Proxy operators may log connections
- **DNS leaks**: Always enable proxy_dns
- **Authentication**: Use credentials for authenticated proxies
- **HTTPS**: Prefer HTTPS for end-to-end encryption

## Best Practices
- **Test proxies**: Verify proxies work before relying on them
- **Use SOCKS5**: More features than SOCKS4
- **Enable proxy_dns**: Prevent DNS leaks
- **Dynamic chain**: More reliable than strict chain
- **Timeout config**: Adjust for your network
- **Local exclusions**: Don't proxy local traffic
- **User config**: Use ~/.proxychains/ for user-specific settings
- **Verify anonymity**: Check IP after proxying

## Uninstall
```yaml
- preset: proxychains
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/rofl0r/proxychains-ng
- Original: https://github.com/haad/proxychains
- Search: "proxychains tutorial", "proxychains tor", "proxychains configuration"
