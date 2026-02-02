# Mooncake Examples

Learn Mooncake through practical examples organized by feature and complexity.

## 🚀 Quick Start

New to Mooncake? Start here:
```bash
mooncake run --config 01-hello-world/config.yml
```

## 📚 Learning Path

Follow the numbered examples in order for the best learning experience:

### Beginner (01-04)

| Example | What You'll Learn | Time |
|---------|-------------------|------|
| **[01-hello-world](01-hello-world/)** | Shell commands, global variables | 5 min |
| **[02-variables-and-facts](02-variables-and-facts/)** | Custom variables, system facts | 10 min |
| **[03-files-and-directories](03-files-and-directories/)** | File operations, permissions | 10 min |
| **[04-conditionals](04-conditionals/)** | When conditions, OS detection | 10 min |

### Intermediate (05-07)

| Example | What You'll Learn | Time |
|---------|-------------------|------|
| **[05-templates](05-templates/)** | Template rendering, pongo2 syntax | 15 min |
| **[06-loops](06-loops/)** | List and file tree iteration | 15 min |
| **[07-register](07-register/)** | Capturing and using command output | 15 min |

### Advanced (08-10)

| Example | What You'll Learn | Time |
|---------|-------------------|------|
| **[08-tags](08-tags/)** | Tag-based filtering, workflows | 10 min |
| **[09-sudo](09-sudo/)** | Privilege escalation, system ops | 15 min |
| **[10-multi-file-configs](10-multi-file-configs/)** | Multi-file organization, includes | 20 min |

### Real-World Applications

| Example | Description | Features Used |
|---------|-------------|---------------|
| **[dotfiles-manager](real-world/dotfiles-manager/)** | Complete dotfiles deployment system | Templates, loops, tags, conditionals |

### Platform-Specific Examples

| Example | Description | Platform |
|---------|-------------|----------|
| **[macos-services](macos-services/)** | macOS service management with launchd | macOS |

## 🎯 Find by Feature

Looking for a specific feature?

- **Shell commands** → [01-hello-world](01-hello-world/)
- **Variables** → [02-variables-and-facts](02-variables-and-facts/)
- **Files & directories** → [03-files-and-directories](03-files-and-directories/)
- **Conditionals (when)** → [04-conditionals](04-conditionals/)
- **Templates (.j2)** → [05-templates](05-templates/)
- **Loops (with_items)** → [06-loops](06-loops/)
- **File iteration (with_filetree)** → [06-loops](06-loops/)
- **Capture output (register)** → [07-register](07-register/)
- **Tags** → [08-tags](08-tags/)
- **Sudo/become** → [09-sudo](09-sudo/)
- **Multiple files (include)** → [10-multi-file-configs](10-multi-file-configs/)
- **Load variables (include_vars)** → [10-multi-file-configs](10-multi-file-configs/)
- **Service management (macOS)** → [macos-services](macos-services/)

## 💻 Running Examples

### Basic Execution

```bash
# Run an example
mooncake run --config 01-hello-world/config.yml

# Preview without executing (safe!)
mooncake run --config 01-hello-world/config.yml --dry-run

# With debug output
mooncake run --config 01-hello-world/config.yml --log-level debug

# Disable animated UI
mooncake run --config 01-hello-world/config.yml --raw
```

### Advanced Usage

```bash
# Filter by tags
mooncake run --config 08-tags/config.yml --tags dev

# With sudo password
mooncake run --config 09-sudo/config.yml --sudo-pass <password>

# Load custom variables
mooncake run --config 10-multi-file-configs/main.yml --vars custom-vars.yml
```

## 🔍 System Information

Before running examples, check what system facts are available:

```bash
mooncake facts
```

This shows:
- OS, distribution, architecture
- CPU cores, memory, GPUs
- Storage devices and network interfaces
- Package manager, Python version

These facts are automatically available in all configurations as variables.

## 📖 Example Structure

Each numbered example includes:
- **config.yml** - The configuration file to run
- **README.md** - Detailed explanation and learning objectives
- **Supporting files** - Templates, sample data, etc.

## 🎓 Tips for Learning

1. **Start at 01** - Examples build on each other
2. **Read the README** - Each example has detailed docs
3. **Use dry-run** - Preview before executing: `--dry-run`
4. **Experiment** - Modify examples to learn
5. **Check debug logs** - Use `--log-level debug` to see what's happening

## 🛠️ Modifying Examples

All examples are safe to modify! Try:

1. **Change variables** - See how configs adapt
2. **Add conditions** - Make steps OS-specific
3. **Add tags** - Create custom workflows
4. **Combine examples** - Use multiple techniques together

## ✅ Best Practices

Examples demonstrate these best practices:

- **Clear naming** - Descriptive step names
- **Comments** - Explain non-obvious choices
- **Variables** - Make configs reusable
- **Conditionals** - Handle different environments
- **Tags** - Enable selective execution
- **Dry-run friendly** - Always preview-safe
- **Permissions** - Appropriate file modes
- **Error handling** - Use register to check results

## 🧪 Validation Examples

The [validation-examples/](validation-examples/) directory contains intentionally invalid configurations for testing Mooncake's error detection and validation features.

## 📚 Additional Resources

- **Main Documentation** - [../../README.md](../../README.md)
- **Configuration Guide** - All available options and syntax
- **System Facts** - Use `mooncake facts` to see available variables

## 🤝 Contributing Examples

Want to add an example? Great! Make sure it:
1. Focuses on one or two related features
2. Includes a detailed README
3. Works with `--dry-run`
4. Follows existing style and structure

## 🆘 Getting Help

**Example not working?**
1. Try `--dry-run` first
2. Check `--log-level debug` output
3. Verify system facts with `mooncake facts`
4. Review the example's README

**Want to learn more?**
- Complete examples in order (01-10)
- Study real-world examples
- Check the main README
- Experiment with modifications

---

**Ready to start?** → Begin with [01-hello-world](01-hello-world/)!
