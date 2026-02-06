# asciinema - Terminal Recorder

Record and share terminal sessions. Create demos, tutorials, and documentation with real terminal output.

## Quick Start
```yaml
- preset: asciinema
```

## Basic Usage
```bash
# Start recording
asciinema rec

# Record to file
asciinema rec demo.cast

# Record with title
asciinema rec -t "My Demo"

# Stop recording
# Press Ctrl+D or type 'exit'

# Play recording
asciinema play demo.cast

# Upload and share
asciinema upload demo.cast
```

## Recording
```bash
# Basic recording
asciinema rec demo.cast

# With title
asciinema rec -t "Git Tutorial" demo.cast

# Append to existing
asciinema rec --append demo.cast

# Overwrite protection
asciinema rec --overwrite demo.cast

# With command (record specific command)
asciinema rec -c "htop" demo.cast

# Idle time limit (skip long pauses)
asciinema rec -i 2 demo.cast  # Max 2 seconds idle

# With environment vars
asciinema rec -e "SHELL,TERM" demo.cast
```

## Playback
```bash
# Play recording
asciinema play demo.cast

# Play from URL
asciinema play https://asciinema.org/a/123456

# Speed control
asciinema play -s 2 demo.cast  # 2x speed
asciinema play -s 0.5 demo.cast  # 0.5x speed

# Idle time limit
asciinema play -i 2 demo.cast  # Max 2 seconds idle

# Loop playback
asciinema play -l demo.cast
```

## Sharing
```bash
# Upload to asciinema.org
asciinema upload demo.cast

# Auth (link account)
asciinema auth

# Get shareable URL
asciinema upload demo.cast
# Returns: https://asciinema.org/a/abc123
```

## Embedding
```html
<!-- On website -->
<script src="https://asciinema.org/a/abc123.js" id="asciicast-abc123" async></script>

<!-- Self-hosted player -->
<asciinema-player src="demo.cast"></asciinema-player>
<link rel="stylesheet" type="text/css" href="/asciinema-player.css" />
<script src="/asciinema-player.min.js"></script>
```

## Format Conversion
```bash
# Cast to GIF (requires agg)
agg demo.cast demo.gif

# Cast to SVG
svg-term --in demo.cast --out demo.svg

# Cast to PNG frames
asciinema-gif-generator demo.cast output-dir/

# Cast to video (with ffmpeg)
asciinema play demo.cast | ffmpeg -f rawvideo -pix_fmt rgb24 \
  -s 800x600 -r 30 -i - output.mp4
```

## Editing Recordings
```bash
# Trim recording
# Edit .cast file (JSON format)
{
  "version": 2,
  "width": 80,
  "height": 24,
  "timestamp": 1234567890,
  "env": {"SHELL": "/bin/bash", "TERM": "xterm-256color"},
  "stdout": [
    [0.5, "$ ls\n"],
    [1.0, "file1.txt  file2.txt\n"]
  ]
}

# Remove idle time
cat demo.cast | jq -c '.[] | if .type == "o" then .time = (.time | if . > 2 then 2 else . end) else . end' > trimmed.cast
```

## Automation
```bash
# Record script execution
asciinema rec -c "./demo-script.sh" demo.cast

# demo-script.sh
#!/bin/bash
echo "# Installing dependencies"
npm install
sleep 1
echo "# Running tests"
npm test
sleep 1
echo "# Build complete"

# Automated demo
expect << 'EOF'
spawn asciinema rec demo.cast
sleep 1
send "echo 'Hello World'\r"
sleep 1
send "ls -la\r"
sleep 1
send "exit\r"
expect eof
EOF
```

## CI/CD Integration
```bash
# Record test execution
asciinema rec -c "npm test" test-output.cast

# GitHub Actions
- name: Record demo
  run: |
    asciinema rec -c "./demo.sh" demo.cast
    asciinema upload demo.cast > url.txt

- name: Upload artifact
  uses: actions/upload-artifact@v3
  with:
    name: demo
    path: demo.cast
```

## Configuration
```bash
# ~/.config/asciinema/config

[api]
token = your-api-token
url = https://asciinema.org

[record]
command = /bin/bash
maxwait = 2
yes = true
idle_time_limit = 2

[play]
maxwait = 2
speed = 1
```

## Advanced Features
```bash
# Record with custom size
asciinema rec --cols 120 --rows 40 demo.cast

# Record with custom command
asciinema rec -c "ssh user@server" server-session.cast

# Pause/resume (manual)
# Press Ctrl+Z to pause, fg to resume

# Multiple terminal multiplexer
tmux new -s demo
asciinema rec -c "tmux attach -t demo" demo.cast
```

## Self-Hosted Player
```html
<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" type="text/css" href="asciinema-player.css" />
</head>
<body>
  <div id="demo"></div>
  <script src="asciinema-player.min.js"></script>
  <script>
    AsciinemaPlayer.create('demo.cast', document.getElementById('demo'), {
      speed: 1,
      theme: 'monokai',
      loop: true,
      autoPlay: true
    });
  </script>
</body>
</html>
```

## Use Cases
```bash
# Tutorial recording
asciinema rec -t "Git Basics" -i 2 git-tutorial.cast

# Bug reproduction
asciinema rec -t "Issue #123" bug-repro.cast

# CLI demo
asciinema rec -c "./cli-demo.sh" cli-demo.cast

# Documentation
asciinema rec -t "Installation Guide" install.cast

# Onboarding
asciinema rec -t "Dev Environment Setup" onboarding.cast
```

## Tips for Better Recordings
```bash
# Clear screen before starting
clear

# Set prompt
export PS1='\$ '

# Slow down typing (for demos)
# Use a script with sleep commands

# Hide sensitive data
export HISTFILE=/dev/null

# Set terminal size
stty cols 80 rows 24

# Record script
cat > demo.sh <<'EOF'
#!/bin/bash
set -e
clear
echo "$ npm install"
sleep 1
npm install
sleep 1
echo "$ npm test"
sleep 1
npm test
EOF

chmod +x demo.sh
asciinema rec -c "./demo.sh" demo.cast
```

## Comparison
| Feature | asciinema | terminalizer | ttyrec | script |
|---------|-----------|--------------|--------|--------|
| Format | JSON | YAML | Binary | Text |
| Playback | Web/CLI | GIF | CLI | Text only |
| Sharing | Easy | GIF | Hard | N/A |
| Editing | JSON | YAML | Hard | N/A |
| Size | Small | Large | Small | Large |

## Best Practices
- **Use idle_time_limit** to skip long pauses
- **Clear sensitive data** before recording
- **Set consistent terminal size** (80x24 or 120x40)
- **Use scripts** for automated demos
- **Add titles** with `-t` for organization
- **Upload to asciinema.org** for easy sharing
- **Convert to GIF** for embedding in docs

## Tips
- Records timing, not pixels
- Tiny file sizes (text-based)
- Searchable output
- Copy-paste from player
- Works over SSH
- No video encoding
- Fast playback

## Agent Use
- Automated demo generation
- Tutorial creation
- Bug reproduction
- Documentation automation
- CLI testing
- Onboarding materials

## Uninstall
```yaml
- preset: asciinema
  with:
    state: absent
```

## Resources
- Website: https://asciinema.org/
- GitHub: https://github.com/asciinema/asciinema
- Player: https://github.com/asciinema/asciinema-player
- Search: "asciinema tutorial", "asciinema embed"
