#!/usr/bin/env python3
"""
Simple Flask application for Docker stack demonstration
"""

from flask import Flask, jsonify, request
import os
import socket
from datetime import datetime

app = Flask(__name__)

@app.route('/')
def hello():
    return '''
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>Mooncake Docker Stack</title>
        <style>
            body {
                font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
                max-width: 900px;
                margin: 50px auto;
                padding: 20px;
                background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
                color: white;
            }
            .container {
                background: rgba(255, 255, 255, 0.1);
                padding: 40px;
                border-radius: 10px;
                backdrop-filter: blur(10px);
            }
            h1 {
                text-align: center;
                font-size: 2.5em;
                margin-bottom: 10px;
            }
            .docker-icon {
                text-align: center;
                font-size: 5em;
                margin: 20px 0;
            }
            .info-box {
                background: rgba(0, 0, 0, 0.2);
                padding: 20px;
                border-radius: 5px;
                margin-top: 20px;
            }
            .info-box h3 {
                margin-top: 0;
                color: #ffd700;
            }
            .info-box p {
                margin: 10px 0;
            }
            code {
                background: rgba(0, 0, 0, 0.3);
                padding: 2px 6px;
                border-radius: 3px;
                font-family: 'Courier New', monospace;
            }
            .api-links {
                text-align: center;
                margin-top: 20px;
            }
            .api-links a {
                color: #ffd700;
                text-decoration: none;
                margin: 0 10px;
            }
            .api-links a:hover {
                text-decoration: underline;
            }
        </style>
    </head>
    <body>
        <div class="container">
            <div class="docker-icon">🐳</div>
            <h1>Hello from Mooncake Docker Stack!</h1>
            <p style="text-align: center; font-size: 1.2em;">
                Your containerized Flask application is running
            </p>

            <div class="info-box">
                <h3>🚀 Stack Components</h3>
                <ul>
                    <li><strong>Flask App</strong> - Python web framework</li>
                    <li><strong>Nginx</strong> - Reverse proxy and load balancer</li>
                    <li><strong>Docker Compose</strong> - Multi-container orchestration</li>
                </ul>
            </div>

            <div class="info-box">
                <h3>📦 Container Info</h3>
                <p><strong>Hostname:</strong> <code>''' + socket.gethostname() + '''</code></p>
                <p><strong>Python Version:</strong> <code>''' + os.sys.version.split()[0] + '''</code></p>
            </div>

            <div class="api-links">
                <strong>Try the API:</strong><br><br>
                <a href="/api/info">📊 /api/info</a>
                <a href="/api/health">💚 /api/health</a>
                <a href="/api/env">🔧 /api/env</a>
            </div>
        </div>
    </body>
    </html>
    '''

@app.route('/api/info')
def info():
    """Return application and container information"""
    return jsonify({
        'status': 'ok',
        'message': 'Hello from Mooncake Docker Stack API!',
        'timestamp': datetime.now().isoformat(),
        'hostname': socket.gethostname(),
        'python_version': os.sys.version,
        'container': {
            'name': os.environ.get('HOSTNAME', 'unknown'),
            'platform': os.sys.platform,
        }
    })

@app.route('/api/health')
def health():
    """Health check endpoint"""
    return jsonify({
        'status': 'healthy',
        'timestamp': datetime.now().isoformat()
    })

@app.route('/api/env')
def env():
    """Return environment variables (filtered)"""
    safe_vars = {
        k: v for k, v in os.environ.items()
        if not any(secret in k.lower() for secret in ['password', 'secret', 'key', 'token'])
    }
    return jsonify({
        'environment': safe_vars
    })

@app.route('/api/request')
def request_info():
    """Return information about the current request"""
    return jsonify({
        'method': request.method,
        'url': request.url,
        'headers': dict(request.headers),
        'remote_addr': request.remote_addr,
    })

if __name__ == '__main__':
    # Run on all interfaces so it's accessible from outside the container
    app.run(host='0.0.0.0', port=5000, debug=False)
