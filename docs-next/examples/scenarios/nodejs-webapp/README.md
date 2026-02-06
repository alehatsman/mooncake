# Node.js Web App Deployment

Deploy a simple Node.js Express application with PM2 process manager and nginx reverse proxy.

## What This Does

This scenario demonstrates a complete web application deployment stack:
- Installing Node.js and npm from NodeSource
- Creating an Express.js web application
- Managing the app with PM2 process manager
- Setting up nginx as a reverse proxy
- Configuring logging and health checks
- Verifying the deployment

## Prerequisites

- Ubuntu 20.04 or later
- Root/sudo access
- Mooncake installed
- Internet connection

## Files

- `deploy.yml` - Main deployment playbook
- `files/app.js` - Express.js application
- `files/package.json` - Node.js dependencies
- `templates/ecosystem.config.js.j2` - PM2 configuration template
- `templates/nginx-proxy.conf.j2` - Nginx reverse proxy config template

## How to Run

```bash
# Run the deployment
mooncake run deploy.yml

# Or with custom settings
mooncake run deploy.yml --var app_name=mywebapp --var app_port=3000 --var nginx_port=80
```

## Variables

You can customize these variables:

- `app_name` (default: `myapp`) - Application name
- `app_port` (default: `3000`) - Application port
- `app_dir` (default: `/opt/{{ app_name }}`) - Application directory
- `nginx_port` (default: `80`) - Nginx listen port
- `node_user` (default: `www-data`) - User to run the app

## What Gets Deployed

### System Components
- Node.js LTS (from NodeSource)
- npm (Node Package Manager)
- PM2 (Process Manager)
- nginx (Reverse Proxy)

### Application Stack
- Express.js web framework
- PM2 process management with auto-restart
- Nginx reverse proxy with proper headers
- Logging to files and systemd
- Health check endpoint

### File Structure
```
/opt/myapp/
├── app.js                    # Main application
├── package.json              # Dependencies
├── ecosystem.config.js       # PM2 config
├── node_modules/             # Installed packages
└── logs/                     # Application logs
```

## Using Your Application

### Access the App

```bash
# Through nginx (public facing)
curl http://localhost

# Direct access
curl http://localhost:3000

# API endpoint
curl http://localhost/api/status

# Health check
curl http://localhost/health
```

### PM2 Management

```bash
# View running apps
sudo -u www-data pm2 list

# View logs
sudo -u www-data pm2 logs myapp

# Restart app
sudo -u www-data pm2 restart myapp

# Stop app
sudo -u www-data pm2 stop myapp

# Monitor
sudo -u www-data pm2 monit
```

### View Logs

```bash
# Application logs
sudo -u www-data pm2 logs

# Nginx access logs
sudo tail -f /var/log/nginx/myapp_access.log

# Nginx error logs
sudo tail -f /var/log/nginx/myapp_error.log
```

### Nginx Management

```bash
# Check status
sudo systemctl status nginx

# Restart nginx
sudo systemctl restart nginx

# Test configuration
sudo nginx -t
```

## Modifying the Application

Edit the application code:

```bash
sudo nano /opt/myapp/app.js
```

After making changes, restart with PM2:

```bash
sudo -u www-data pm2 restart myapp
```

## Cleanup

To remove the deployment:

```bash
# Stop and remove PM2 process
sudo -u www-data pm2 delete myapp
sudo -u www-data pm2 save

# Remove application
sudo rm -rf /opt/myapp

# Remove nginx config
sudo rm -f /etc/nginx/sites-{available,enabled}/myapp
sudo systemctl restart nginx

# Optionally remove Node.js
sudo apt-get remove --purge nodejs npm
```

## Learning Points

This example teaches:
- Installing Node.js from NodeSource repository
- Creating Express.js applications
- Using PM2 for process management
- Configuring nginx as a reverse proxy
- Managing file ownership and permissions
- Setting up proper logging
- Health checks and monitoring
- Service management with systemd

## Production Considerations

For production use, also consider:
- SSL/TLS certificates with Let's Encrypt
- Environment variable management
- Database connections
- Monitoring and alerting
- Load balancing with multiple instances
- Log rotation
- Security hardening
- Firewall configuration

## Next Steps

After deployment, try:
- Modify the app to add new routes
- Scale with PM2: `pm2 scale myapp 4`
- Add SSL with certbot
- Connect to a database
- Deploy your own Node.js application
