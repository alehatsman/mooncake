const express = require('express');
const app = express();
const port = process.env.PORT || 3000;

// Middleware to log requests
app.use((req, res, next) => {
  console.log(`${new Date().toISOString()} - ${req.method} ${req.path}`);
  next();
});

// Root route
app.get('/', (req, res) => {
  res.send(`
    <!DOCTYPE html>
    <html lang="en">
    <head>
      <meta charset="UTF-8">
      <meta name="viewport" content="width=device-width, initial-scale=1.0">
      <title>Mooncake Express App</title>
      <style>
        body {
          font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
          max-width: 800px;
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
        .moon {
          text-align: center;
          font-size: 5em;
          margin: 20px 0;
        }
        .info {
          background: rgba(0, 0, 0, 0.2);
          padding: 20px;
          border-radius: 5px;
          margin-top: 20px;
        }
        .info p {
          margin: 10px 0;
        }
        code {
          background: rgba(0, 0, 0, 0.3);
          padding: 2px 6px;
          border-radius: 3px;
          font-family: 'Courier New', monospace;
        }
        a {
          color: #ffd700;
          text-decoration: none;
        }
        a:hover {
          text-decoration: underline;
        }
      </style>
    </head>
    <body>
      <div class="container">
        <div class="moon">🌙</div>
        <h1>Hello from Mooncake Express!</h1>
        <p style="text-align: center; font-size: 1.2em;">
          Your Node.js application is running!
        </p>
        <div class="info">
          <p><strong>🚀 Stack:</strong></p>
          <ul>
            <li>Node.js v${process.version}</li>
            <li>Express.js web framework</li>
            <li>PM2 process manager</li>
            <li>Nginx reverse proxy</li>
          </ul>
          <p><strong>⚙️ Environment:</strong></p>
          <ul>
            <li>Port: ${port}</li>
            <li>Node ENV: ${process.env.NODE_ENV || 'development'}</li>
            <li>Platform: ${process.platform}</li>
          </ul>
        </div>
        <p style="text-align: center; margin-top: 20px;">
          Try the API: <a href="/api/status">/api/status</a>
        </p>
      </div>
    </body>
    </html>
  `);
});

// API endpoint
app.get('/api/status', (req, res) => {
  res.json({
    status: 'ok',
    message: 'Hello from Mooncake Express API!',
    timestamp: new Date().toISOString(),
    uptime: process.uptime(),
    nodeVersion: process.version,
    platform: process.platform
  });
});

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({ status: 'healthy' });
});

// 404 handler
app.use((req, res) => {
  res.status(404).send('404 - Not Found');
});

// Start server
app.listen(port, () => {
  console.log(`🌙 Mooncake Express app listening on port ${port}`);
  console.log(`Environment: ${process.env.NODE_ENV || 'development'}`);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('SIGTERM signal received: closing HTTP server');
  server.close(() => {
    console.log('HTTP server closed');
  });
});
