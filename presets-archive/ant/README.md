# Apache Ant - Java Build Tool

Build automation tool for Java projects. XML-based build scripts with extensive task library.

## Quick Start
```yaml
- preset: ant
```

## Features
- **XML build files**: Declarative build configuration
- **Extensive tasks**: Compile, test, package, deploy
- **Cross-platform**: Runs on any Java-supported platform
- **Extensible**: Custom tasks in Java
- **Dependencies**: Target dependency management
- **Properties**: Build-time configuration via properties
- **Ant-contrib**: Additional tasks via extensions

## Basic Usage
```bash
# Run default target
ant

# Run specific target
ant compile

# Run multiple targets
ant clean compile test

# Use custom build file
ant -f custom-build.xml

# Set property
ant -Dversion=1.0.0 build
```

## Advanced Configuration
```yaml
- preset: ant
  with:
    state: present
  become: true
```

## Platform Support
- ✅ Linux (apt, dnf, yum, pacman, zypper, apk)
- ✅ macOS (Homebrew)
- ❌ Windows

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Automated deployment and configuration
- Infrastructure as code workflows
- CI/CD pipeline integration
- Development environment setup
- Production service management

## Uninstall
```yaml
- preset: ant
  with:
    state: absent
```

## Resources
- Search: "ant documentation", "ant tutorial"
