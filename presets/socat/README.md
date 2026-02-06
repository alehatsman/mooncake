# socat - Multipurpose Relay Tool

SOcket CAT - bidirectional data relay between two independent data channels. Network Swiss Army knife for TCP, UDP, Unix sockets, SSL, SOCKS, and more.

## Quick Start
```yaml
- preset: socat
```

## Features
- **Bidirectional**: Transfer data in both directions simultaneously
- **Protocol support**: TCP, UDP, Unix sockets, SSL/TLS, SOCKS, HTTP
- **Port forwarding**: Forward ports with protocol conversion
- **File descriptors**: Connect any file descriptor type
- **Encryption**: Built-in SSL/TLS support
- **SOCKS proxy**: Act as or connect through SOCKS proxy
- **Serial ports**: Connect to serial devices

## Basic Usage
```bash
# Check version
socat -V

# TCP port forwarding
socat TCP-LISTEN:8080,fork TCP:remote-host:80

# UDP relay
socat UDP-LISTEN:53,fork UDP:8.8.8.8:53

# Connect to Unix socket
socat - UNIX-CONNECT:/var/run/docker.sock

# Read from serial port
socat /dev/ttyS0,raw,echo=0 -
```

## Port Forwarding

### TCP Forwarding
```bash
# Forward local port 8080 to remote port 80
socat TCP-LISTEN:8080,fork TCP:example.com:80

# Bind to specific interface
socat TCP-LISTEN:8080,bind=127.0.0.1,fork TCP:backend:3000

# Allow reuse of port
socat TCP-LISTEN:8080,reuseaddr,fork TCP:server:80
```

### UDP Forwarding
```bash
# UDP port forward
socat UDP-LISTEN:5353,fork UDP:dns-server:53

# Multicast relay
socat UDP4-RECVFROM:1234,ip-add-membership=239.255.1.2:eth0,fork \
  UDP4-SENDTO:239.255.1.2:1234
```

## SSL/TLS Connections

### SSL Client
```bash
# Connect to HTTPS
socat - SSL:example.com:443,verify=0

# With certificate verification
socat - SSL:example.com:443,cafile=/etc/ssl/certs/ca-bundle.crt

# Client certificate
socat - SSL:server:443,cert=client.pem,key=client-key.pem
```

### SSL Server
```bash
# SSL listener
socat SSL-LISTEN:8443,cert=server.pem,key=server-key.pem,verify=0,fork \
  TCP:localhost:8080

# Require client certificates
socat SSL-LISTEN:8443,cert=server.pem,cafile=ca.pem,verify=1,fork \
  EXEC:/bin/bash
```

## Unix Sockets

### Connect to Socket
```bash
# Docker socket
socat - UNIX-CONNECT:/var/run/docker.sock

# HTTP request via Unix socket
echo -e "GET / HTTP/1.0\r\n\r\n" | \
  socat - UNIX-CONNECT:/var/run/docker.sock

# PostgreSQL socket
socat TCP-LISTEN:5432,fork UNIX-CONNECT:/var/run/postgresql/.s.PGSQL.5432
```

### Create Socket Server
```bash
# Listen on Unix socket
socat UNIX-LISTEN:/tmp/mysocket,fork EXEC:/bin/cat

# Bidirectional relay
socat UNIX-LISTEN:/tmp/input,fork UNIX-CONNECT:/tmp/output
```

## Network Testing

### Port Scanner
```bash
# Check if port is open
socat - TCP:hostname:80,connect-timeout=2

# Scan multiple ports
for port in {20..25}; do
  socat - TCP:example.com:$port,connect-timeout=1 2>&1 | \
    grep -q "succeeded" && echo "Port $port open"
done
```

### Simple HTTP Server
```bash
# Single request
echo "HTTP/1.0 200 OK\r\n\r\nHello" | \
  socat TCP-LISTEN:8000,reuseaddr,fork -

# Serve file
while true; do
  socat TCP-LISTEN:8000,reuseaddr SYSTEM:"echo HTTP/1.0 200; echo; cat index.html"
done
```

### Traffic Monitoring
```bash
# Log traffic
socat -v TCP-LISTEN:8080,fork TCP:server:80

# With timestamps
socat -d -d TCP-LISTEN:8080,fork TCP:server:80 2>&1 | \
  while read line; do echo "$(date): $line"; done
```

## Serial Port Communication
```bash
# Connect to serial device
socat /dev/ttyUSB0,raw,echo=0,b9600 -

# Serial port bridge
socat /dev/ttyS0,raw,echo=0 /dev/ttyS1,raw,echo=0

# Serial to TCP
socat TCP-LISTEN:5000,fork /dev/ttyUSB0,raw,echo=0,b115200
```

## SOCKS Proxy

### SOCKS4 Client
```bash
# Connect through SOCKS proxy
socat - SOCKS4:proxy.example.com:destination.com:80,socksport=1080

# With authentication
socat - SOCKS4A:proxy:target:443,socksport=1080,socksuser=myuser
```

### SOCKS Server
```bash
# Simple SOCKS4 server
socat TCP-LISTEN:1080,fork SOCKS4:localhost:0.0.0.0:0,bind=0.0.0.0
```

## File Operations

### File Transfer
```bash
# Send file over network
socat -u FILE:data.bin TCP:receiver:9999

# Receive file
socat -u TCP-LISTEN:9999 OPEN:received.bin,create,trunc

# Pipe file through network
socat -u OPEN:input.txt TCP:host:9999 | socat -u TCP-LISTEN:9999 OPEN:output.txt
```

### File Monitoring
```bash
# Tail file over network
socat -u OPEN:logfile,seek-end=0,forever TCP-LISTEN:9999

# Receive and display
socat TCP:server:9999 -
```

## Real-World Examples

### Database Tunnel
```yaml
- name: Create PostgreSQL tunnel
  shell: |
    socat TCP-LISTEN:5432,bind=127.0.0.1,reuseaddr,fork \
      TCP:db-server.internal:5432 &
  register: tunnel

- name: Connect through tunnel
  shell: psql -h localhost -U myuser mydb
```

### Docker Socket Proxy
```bash
# Expose Docker socket over TCP (insecure, use with caution)
socat TCP-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock

# SSL-secured Docker socket
socat SSL-LISTEN:2376,cert=server.pem,verify=1,fork \
  UNIX-CONNECT:/var/run/docker.sock
```

### Load Balancer
```bash
# Simple round-robin (requires scripting)
socat TCP-LISTEN:80,fork,reuseaddr \
  SYSTEM:"if [ \$((RANDOM%2)) -eq 0 ]; then \
    socat - TCP:backend1:80; \
  else \
    socat - TCP:backend2:80; \
  fi"
```

### Kubernetes Port Forward Alternative
```bash
# Forward pod port
kubectl get pod mypod -o jsonpath='{.status.podIP}' | \
  xargs -I {} socat TCP-LISTEN:8080,fork TCP:{}:80
```

## Advanced Usage

### Protocol Conversion
```bash
# TCP to UDP
socat TCP-LISTEN:8080,fork UDP:target:5000

# UDP to TCP
socat UDP-LISTEN:5000,fork TCP:target:8080
```

### Multi-Connection Handling
```bash
# Fork for each connection
socat TCP-LISTEN:8080,fork,max-children=10 EXEC:/usr/bin/myprogram

# Connection limit
socat TCP-LISTEN:8080,fork,max-children=100,children-shutup \
  TCP:backend:80
```

### Timeouts
```bash
# Connection timeout
socat TCP-LISTEN:8080,connect-timeout=5,fork TCP:slow-server:80

# Idle timeout
socat TCP-LISTEN:8080,idle-timeout=60,fork TCP:backend:80
```

## CI/CD Integration
```yaml
- name: Setup SSH tunnel
  shell: |
    socat TCP-LISTEN:3306,bind=127.0.0.1,reuseaddr,fork \
      TCP:db.internal:3306 &
    echo $! > /tmp/socat-tunnel.pid
  register: tunnel

- name: Run database migrations
  shell: ./migrate -database "mysql://localhost:3306/db"
  when: tunnel.rc == 0

- name: Cleanup tunnel
  shell: |
    kill $(cat /tmp/socat-tunnel.pid)
    rm /tmp/socat-tunnel.pid
  when: tunnel.rc == 0
```

## Troubleshooting

### Connection Issues
```bash
# Enable debug output
socat -d -d TCP-LISTEN:8080,fork TCP:server:80

# Verbose mode
socat -v TCP-LISTEN:8080,fork TCP:server:80

# Show all traffic
socat -x TCP-LISTEN:8080,fork TCP:server:80
```

### Permission Denied
```bash
# Bind to low port (requires root)
sudo socat TCP-LISTEN:80,fork TCP:localhost:8080

# Or use setcap
sudo setcap 'cap_net_bind_service=+ep' /usr/bin/socat
```

## Comparison with Alternatives
| Feature | socat | netcat | ssh tunnel |
|---------|-------|--------|------------|
| Bidirectional | Yes | Limited | Yes |
| SSL/TLS | Built-in | No | Built-in |
| Unix sockets | Yes | Limited | No |
| Protocol conversion | Yes | No | No |
| Complexity | High | Low | Medium |

## Security Considerations
- Disable SSL verification only for testing
- Use client certificates for mutual TLS
- Bind to localhost for local-only forwarding
- Set connection limits to prevent DoS
- Monitor and log all connections
- Use firewall rules to restrict access
- Avoid exposing sensitive sockets over network

## Platform Support
- ✅ Linux (all distributions)
- ✅ macOS (via Homebrew)
- ✅ BSD systems
- ✅ Solaris
- ❌ Windows (use WSL)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Network service debugging and testing
- Port forwarding automation
- Protocol conversion in pipelines
- Unix socket exposure for tools
- SSL/TLS proxy for legacy services
- Serial device communication

## Advanced Configuration
```yaml
- preset: socat
  with:
    state: present
```

## Uninstall
```yaml
- preset: socat
  with:
    state: absent
```

## Resources
- Man page: `man socat`
- Website: http://www.dest-unreach.org/socat/
- Examples: http://www.dest-unreach.org/socat/doc/socat.html
- Search: "socat examples", "socat port forward", "socat ssl"
