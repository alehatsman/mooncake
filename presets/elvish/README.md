# Elvish - Friendly Interactive Shell

Expressive programming language and versatile interactive shell with modern features and structured data support.

## Quick Start
```yaml
- preset: elvish
```

## Features
- **Modern syntax**: Clean, readable shell language
- **Structured data**: Native support for lists and maps
- **Powerful pipelines**: Data processing with structured values
- **Interactive**: Advanced line editor with syntax highlighting
- **Cross-platform**: Linux, macOS, Windows, BSD
- **Module system**: Organize code into reusable modules

## Basic Usage
```bash
# Start elvish
elvish

# Basic commands work as expected
ls
cd /home/user
echo "Hello, World"

# Variables
var name = "Alice"
echo $name

# Lists
var fruits = [apple orange banana]
echo $fruits[0]  # apple

# Maps
var person = [&name=Bob &age=30]
echo $person[name]  # Bob

# Pipelines
ls | each {|x| echo "File: "$x}

# Functions
fn greet {|name|
  echo "Hello, "$name
}
greet Alice
```

## Advanced Configuration
```yaml
# Install elvish
- preset: elvish

# Uninstall
- preset: elvish
  with:
    state: absent
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ✅ Windows

## Configuration
```elvish
# ~/.config/elvish/rc.elv

# Set prompt
set edit:prompt = { tilde-abbr $pwd; put '> ' }

# Aliases
fn ll {|@a| ls -lah $@a }
fn g {|@a| git $@a }

# Custom functions
fn mkcd {|dir|
  mkdir -p $dir
  cd $dir
}

# Environment variables
set E:EDITOR = nvim
set E:LANG = en_US.UTF-8

# Path management
set paths = [
  ~/bin
  ~/.local/bin
  $@paths
]
```

## Real-World Examples

### Data Processing
```elvish
# Count files by extension
ls | each {|f| path:ext $f} | frequency | to-json

# Find large files
ls -l | drop 1 | each {|line|
  re:find '^\S+\s+\S+\s+\S+\s+\S+\s+(\d+)' $line
} | each {|m| $m[groups][1][text]} | to-lines | sort -n | tail

# Process JSON
curl -s https://api.github.com/users/elves | from-json | echo (all)

# Filter and transform
cat data.txt | each {|line|
  if (str:has-prefix $line "ERROR") {
    str:to-upper $line
  }
}
```

### System Administration
```elvish
# Batch rename files
ls *.txt | each {|f|
  mv $f (str:replace .txt .bak $f)
}

# Monitor processes
while $true {
  clear
  ps aux | head -n 20
  sleep 2
}

# Check disk usage
df -h | drop 1 | each {|line|
  var fields = (str:split ' ' $line)
  if (> (str:trim-suffix % $fields[4] | to-num) 80) {
    echo "WARNING: "$fields[0]" is "$fields[4]" full"
  }
}
```

### Scripting
```elvish
#!/usr/bin/env elvish

# Backup script
fn backup {|source dest|
  var timestamp = (date +%Y%m%d-%H%M%S)
  var backup-file = $dest/backup-$timestamp.tar.gz

  echo "Creating backup..."
  tar -czf $backup-file $source

  if (== $? 0) {
    echo "Backup created: "$backup-file
  } else {
    echo "Backup failed!"
    exit 1
  }
}

backup /home/user /backups
```

## Language Features

### Lists
```elvish
# Create list
var colors = [red green blue]

# Access elements
echo $colors[0]     # red
echo $colors[1..3]  # [green blue]

# Iterate
each {|c| echo $c } $colors

# Append
set colors = [$@colors yellow]

# List operations
range 10            # [0 1 2 3 4 5 6 7 8 9]
repeat 3 hello      # [hello hello hello]
```

### Maps
```elvish
# Create map
var config = [
  &host=localhost
  &port=8080
  &debug=$true
]

# Access values
echo $config[host]  # localhost

# Iterate
keys $config | each {|k|
  echo $k": "$config[$k]
}

# Merge maps
var defaults = [&timeout=30 &retries=3]
var config = [&$defaults &host=example.com]
```

### Functions
```elvish
# Simple function
fn greet {|name|
  echo "Hello, "$name
}

# Multiple parameters
fn add {|a b|
  + $a $b
}

# Variable arguments
fn sum {|@nums|
  var total = 0
  each {|n| set total = (+ $total $n) } $nums
  put $total
}

# Return values
fn get-user-info {
  put [&name=Alice &age=30]
}
var user = (get-user-info)
```

### Control Flow
```elvish
# If/elif/else
if (== $E:USER root) {
  echo "Running as root"
} elif (== $E:USER admin) {
  echo "Running as admin"
} else {
  echo "Running as "$E:USER
}

# Try/except
try {
  some-command
} catch e {
  echo "Error: "$e[reason]
}

# Loops
for x [1 2 3] {
  echo $x
}

while (< $i 10) {
  echo $i
  set i = (+ $i 1)
}
```

### Pipelines
```elvish
# Traditional pipeline
ls | grep txt | wc -l

# Elvish structured pipeline
ls *.txt | each {|f|
  var size = (stat -f %z $f)
  put [&name=$f &size=$size]
} | to-json

# Pipeline with error handling
ls | each {|f|
  try {
    cat $f | wc -l
  } catch {
    echo "Error reading "$f
  }
}
```

## Interactive Features

### Line Editor
```elvish
# Keybindings
set edit:insert:binding[Ctrl-T] = $edit:transpose-char~
set edit:insert:binding[Ctrl-W] = $edit:kill-word-left~

# Completion
edit:completion:matcher[''] = $edit:match-prefix~

# Custom prompt
fn prompt {
  var pwd = (tilde-abbr $pwd)
  styled $pwd cyan
  put ' > '
}
set edit:prompt = $prompt~
```

### History
```elvish
# Search history
edit:command-history

# History configuration
set edit:max-cmd-duration = 2  # Warn for long commands
```

## Module System
```elvish
# Create module: ~/.config/elvish/lib/utils.elv
fn say-hello {|name|
  echo "Hello, "$name
}

# Use module: ~/.config/elvish/rc.elv
use utils
utils:say-hello World

# Or import specific functions
use utils [ say-hello ]
say-hello World
```

## Agent Use
- Interactive shell scripting with structured data
- Data processing pipelines with maps and lists
- System automation with modern syntax
- Task orchestration with error handling
- Configuration management scripts
- DevOps automation workflows
- Log processing and analysis
- Build and deployment scripts

## Comparison with Other Shells

| Feature | Elvish | Bash | Zsh | Fish |
|---------|--------|------|-----|------|
| Structured data | ✅ | ❌ | ❌ | ❌ |
| Modern syntax | ✅ | ❌ | ~  | ✅ |
| Built-in lists/maps | ✅ | ❌ | ❌ | ❌ |
| Cross-platform | ✅ | ❌ | ~ | ✅ |
| Scripting | ✅ | ✅ | ✅ | ~ |
| Interactive | ✅ | ~ | ✅ | ✅ |

## Troubleshooting

### Command not found
```elvish
# Check PATH
echo $paths

# Add to PATH
set paths = [~/bin $@paths]

# Verify command exists
which command-name
```

### Config errors
```bash
# Check for syntax errors
elvish -c 'use rc'

# Run with verbose output
elvish -compileonly ~/.config/elvish/rc.elv
```

### Module import fails
```elvish
# Check module path
echo $paths

# Manually load module
use ~/.config/elvish/lib/mymodule
```

## Tips and Tricks
- Use `put` instead of `echo` for structured data
- Leverage pipelines with `each` for data transformation
- Store configuration in maps for easy access
- Use `try/catch` for robust error handling
- Create modules for reusable functions
- Take advantage of syntax highlighting in interactive mode

## Migration from Bash

### Variable Assignment
```bash
# Bash
name="Alice"

# Elvish
var name = "Alice"
```

### Arrays/Lists
```bash
# Bash
colors=(red green blue)
echo ${colors[0]}

# Elvish
var colors = [red green blue]
echo $colors[0]
```

### Functions
```bash
# Bash
greet() {
  echo "Hello, $1"
}

# Elvish
fn greet {|name|
  echo "Hello, "$name
}
```

## Uninstall
```yaml
- preset: elvish
  with:
    state: absent
```

## Resources
- Official site: https://elv.sh/
- Documentation: https://elv.sh/learn/
- Reference: https://elv.sh/ref/
- GitHub: https://github.com/elves/elvish
- Search: "elvish shell tutorial", "elvish vs bash", "elvish scripting"
