# Nginx Ubuntu Setup

A simple "hello world" example that sets up an nginx web server on Ubuntu.

## What This Does

This scenario demonstrates:
- Installing nginx via apt
- Creating site configurations using templates
- Deploying static content
- Managing nginx service
- Verifying the setup with assertions

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed

## Files

- `setup.yml` - Main playbook
- `templates/nginx.conf.j2` - Nginx main configuration template
- `templates/site.conf.j2` - Site-specific configuration template
- `files/index.html` - Welcome page

## How to Run

```bash
# Run the setup
mooncake run setup.yml

# Or with custom variables
mooncake run setup.yml --var site_name=myapp --var site_port=9090
```

## Variables

You can customize these variables:

- `site_name` (default: `mysite`) - Name of your site
- `site_port` (default: `8080`) - Port to listen on
- `document_root` (default: `/var/www/{{ site_name }}`) - Root directory for site files

## What Gets Created

- Nginx installation via apt
- Site directory: `/var/www/mysite/`
- Nginx config: `/etc/nginx/nginx.conf`
- Site config: `/etc/nginx/sites-available/mysite`
- Symlink: `/etc/nginx/sites-enabled/mysite`
- Welcome page with styled HTML

## Testing

After running, test your site:

```bash
# Check nginx status
sudo systemctl status nginx

# Test the site
curl http://localhost:8080

# View in browser
firefox http://localhost:8080
```

## Cleanup

To remove the setup:

```bash
sudo systemctl stop nginx
sudo apt-get remove --purge nginx nginx-common
sudo rm -rf /var/www/mysite
sudo rm -f /etc/nginx/sites-available/mysite /etc/nginx/sites-enabled/mysite
```

## Learning Points

This example teaches:
- Installing packages with shell actions
- Using templates for configuration files
- File management (directories, copies, symlinks)
- Service management
- Using assert to verify success
- Using register and print for debugging
