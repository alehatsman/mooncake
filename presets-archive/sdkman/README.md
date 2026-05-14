# SDKMAN! - Software Development Kit Manager

Manage parallel versions of multiple JVM SDKs. Install, switch, and manage Java, Kotlin, Scala, Groovy, Gradle, Maven, and 70+ other SDK tools.

## Quick Start
```yaml
- preset: sdkman
```

## Features
- **Multi-SDK**: Manage 70+ SDKs (Java, Kotlin, Scala, Gradle, Maven, Groovy, Micronaut, Quarkus)
- **Version management**: Install and switch between SDK versions per shell
- **Parallel versions**: Multiple versions installed simultaneously
- **Offline mode**: Work without network connectivity
- **Lightweight**: Pure bash, no dependencies
- **Cross-platform**: Linux, macOS, WSL, Cygwin, Solaris

## Basic Usage
```bash
# List available SDKs
sdk list

# List Java versions
sdk list java

# Install latest stable Java
sdk install java

# Install specific version
sdk install java 17.0.9-tem
sdk install gradle 8.5

# Use version for current shell
sdk use java 17.0.9-tem

# Set default version
sdk default java 17.0.9-tem

# Show current versions
sdk current
sdk current java

# Update SDK database
sdk update

# Upgrade installed SDKs
sdk upgrade
```

## Java Management
```bash
# List Java distributions
sdk list java

# Available distributions:
# - Temurin (Eclipse)
# - Oracle OpenJDK
# - Amazon Corretto
# - GraalVM
# - Liberica
# - Zulu
# - SapMachine

# Install specific distribution
sdk install java 17.0.9-tem    # Temurin
sdk install java 17.0.9-oracle # Oracle
sdk install java 17.0.9-amzn   # Corretto
sdk install java 21.0.1-graal  # GraalVM

# Switch Java version
sdk use java 17.0.9-tem

# Set default
sdk default java 17.0.9-tem

# Uninstall
sdk uninstall java 17.0.9-tem
```

## Build Tools
```bash
# Gradle
sdk install gradle 8.5
sdk use gradle 8.5
gradle --version

# Maven
sdk install maven 3.9.6
sdk use maven 3.9.6
mvn --version

# Ant
sdk install ant 1.10.14
sdk use ant 1.10.14

# SBT (Scala Build Tool)
sdk install sbt 1.9.8
sdk use sbt 1.9.8
```

## Frameworks
```bash
# Micronaut
sdk install micronaut 4.2.3
mn create-app myapp

# Quarkus
sdk install quarkus 3.6.4
quarkus create app org.acme:myapp

# Spring Boot
sdk install springboot 3.2.1
spring init --dependencies=web myapp

# Vert.x
sdk install vertx 4.5.1
vertx create myapp
```

## Language Runtimes
```bash
# Kotlin
sdk install kotlin 1.9.22
kotlinc -version

# Groovy
sdk install groovy 4.0.17
groovy --version

# Scala
sdk install scala 3.3.1
scala -version
```

## Version Management

### Per-Shell Version
```bash
# Terminal 1
sdk use java 17.0.9-tem
java -version  # Shows Java 17

# Terminal 2
sdk use java 21.0.1-graal
java -version  # Shows Java 21
```

### Project-Specific Version (.sdkmanrc)
```bash
# Create .sdkmanrc
cd myproject
sdk env init

# Contents:
java=17.0.9-tem
gradle=8.5
kotlin=1.9.22

# Auto-activate on cd
sdk env

# Add to shell profile
cd() { builtin cd "$@" && sdk env; }
```

### Default Version
```bash
# Set system default
sdk default java 17.0.9-tem

# Check current defaults
sdk current

# Output:
# Using java version 17.0.9-tem
# Using gradle version 8.5
```

## Configuration

### Location
- **Installation**: `~/.sdkman/`
- **Candidates**: `~/.sdkman/candidates/`
- **Archives**: `~/.sdkman/archives/`
- **Config**: `~/.sdkman/etc/config`

### Config Options
```bash
# Edit config
vim ~/.sdkman/etc/config

# Common settings:
sdkman_auto_answer=true           # Auto-answer prompts
sdkman_auto_selfupdate=true       # Auto-update SDKMAN
sdkman_insecure_ssl=false         # SSL verification
sdkman_curl_connect_timeout=7     # Connection timeout
sdkman_curl_max_time=10           # Max download time
sdkman_colour_enable=true         # Color output
sdkman_auto_env=true              # Auto-switch via .sdkmanrc
```

### Shell Integration
```bash
# Already added during installation to:
# ~/.bashrc, ~/.zshrc, ~/.bash_profile

# Manual initialization
source "$HOME/.sdkman/bin/sdkman-init.sh"
```

## Offline Mode
```bash
# Download for offline use
sdk install java 17.0.9-tem

# Enable offline mode
sdk offline enable

# Use offline
sdk list java      # Shows only installed
sdk use java 17.0.9-tem

# Disable offline
sdk offline disable
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Install SDKMAN
  run: |
    curl -s "https://get.sdkman.io" | bash
    source "$HOME/.sdkman/bin/sdkman-init.sh"

- name: Install Java and Gradle
  run: |
    source "$HOME/.sdkman/bin/sdkman-init.sh"
    sdk install java 17.0.9-tem
    sdk install gradle 8.5
    sdk default java 17.0.9-tem
    sdk default gradle 8.5

- name: Build
  run: |
    source "$HOME/.sdkman/bin/sdkman-init.sh"
    gradle build
```

### GitLab CI
```yaml
before_script:
  - curl -s "https://get.sdkman.io" | bash
  - source "$HOME/.sdkman/bin/sdkman-init.sh"
  - sdk install java 17.0.9-tem
  - sdk default java 17.0.9-tem

build:
  script:
    - source "$HOME/.sdkman/bin/sdkman-init.sh"
    - ./gradlew build
```

### Docker
```dockerfile
# Install SDKMAN
RUN curl -s "https://get.sdkman.io" | bash && \
    bash -c "source $HOME/.sdkman/bin/sdkman-init.sh && \
    sdk install java 17.0.9-tem && \
    sdk install gradle 8.5 && \
    sdk default java 17.0.9-tem && \
    sdk default gradle 8.5"

# Use in RUN commands
RUN bash -c "source $HOME/.sdkman/bin/sdkman-init.sh && gradle build"
```

## Advanced Usage

### Flush Caches
```bash
# Clear download cache
sdk flush archives

# Clear tmp files
sdk flush tmp

# Reinstall
sdk flush
```

### Self-Update
```bash
# Update SDKMAN itself
sdk selfupdate

# Force update
sdk selfupdate force
```

### Broadcast Messages
```bash
# Show latest news
sdk broadcast

# Output:
# Latest SDK news and updates
```

### Help
```bash
# General help
sdk help

# Command help
sdk help install
sdk help use
```

## Real-World Examples

### Multi-Project Developer
```yaml
# Project 1: Legacy Java 11 + Gradle 7
- name: Setup project 1
  shell: |
    source ~/.sdkman/bin/sdkman-init.sh
    cd /workspace/legacy-app
    echo "java=11.0.21-tem" > .sdkmanrc
    echo "gradle=7.6" >> .sdkmanrc
    sdk env

# Project 2: Modern Java 21 + Gradle 8
- name: Setup project 2
  shell: |
    source ~/.sdkman/bin/sdkman-init.sh
    cd /workspace/modern-app
    echo "java=21.0.1-graal" > .sdkmanrc
    echo "gradle=8.5" >> .sdkmanrc
    sdk env
```

### Testing Multiple Versions
```bash
# Test against Java 11, 17, 21
for version in 11.0.21-tem 17.0.9-tem 21.0.1-graal; do
  sdk use java $version
  ./gradlew test
done
```

### Framework Setup
```yaml
- name: Install development stack
  shell: |
    source ~/.sdkman/bin/sdkman-init.sh
    sdk install java 17.0.9-tem
    sdk install gradle 8.5
    sdk install kotlin 1.9.22
    sdk install micronaut 4.2.3
    sdk default java 17.0.9-tem

- name: Create Micronaut app
  shell: |
    source ~/.sdkman/bin/sdkman-init.sh
    mn create-app com.example.myapp --features=graalvm
```

## Troubleshooting

### Command Not Found
```bash
# Reinitialize
source "$HOME/.sdkman/bin/sdkman-init.sh"

# Check installation
ls -la ~/.sdkman/

# Reinstall
curl -s "https://get.sdkman.io" | bash
```

### Version Not Switching
```bash
# Check current
sdk current java

# Force default
sdk default java 17.0.9-tem

# Check PATH
echo $PATH | grep sdkman

# Reload shell
exec $SHELL
```

### Slow Downloads
```bash
# Adjust timeouts
vim ~/.sdkman/etc/config

# Increase:
sdkman_curl_connect_timeout=15
sdkman_curl_max_time=30
```

## Comparison with Alternatives
| Feature | SDKMAN | jenv | asdf | Homebrew |
|---------|--------|------|------|----------|
| JVM SDKs | 70+ | Java only | Via plugins | Limited |
| Version switching | Yes | Yes | Yes | No |
| .sdkmanrc | Yes | .java-version | .tool-versions | No |
| Offline mode | Yes | No | No | No |
| Install speed | Fast | Manual | Moderate | Moderate |

## Best Practices
- Use `.sdkmanrc` for project consistency
- Set sensible defaults with `sdk default`
- Enable auto-env for automatic switching
- Keep SDKMAN updated (`sdk selfupdate`)
- Use offline mode for unreliable networks
- Document SDK versions in project README
- Test against multiple Java versions in CI

## Advanced Configuration

### Mooncake Integration
```yaml
- preset: sdkman

- name: Configure project environment
  template:
    content: |
      java={{ java_version }}
      gradle={{ gradle_version }}
      kotlin={{ kotlin_version }}
    dest: "{{ project_dir }}/.sdkmanrc"
    mode: "0644"

- name: Install SDKs
  shell: |
    source ~/.sdkman/bin/sdkman-init.sh
    cd {{ project_dir }}
    sdk env install
```

### Team Standardization
```yaml
- name: Enforce team SDK versions
  shell: |
    source ~/.sdkman/bin/sdkman-init.sh
    sdk install java 17.0.9-tem
    sdk install gradle 8.5
    sdk default java 17.0.9-tem
    sdk default gradle 8.5
  become: false
```

## Platform Support
- ✅ Linux (bash)
- ✅ macOS (bash, zsh)
- ✅ Windows (WSL, Cygwin, Git Bash)
- ✅ Solaris
- ❌ Windows native (use WSL)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove tool |

## Agent Use
- Automated development environment setup
- CI/CD Java version management
- Multi-version testing automation
- Project environment standardization
- SDK version compliance checking
- Development toolchain provisioning

## Uninstall
```yaml
- preset: sdkman
  with:
    state: absent
```

Manual removal:
```bash
rm -rf ~/.sdkman
# Remove from ~/.bashrc, ~/.zshrc
```

## Resources
- Website: https://sdkman.io/
- Documentation: https://sdkman.io/usage
- SDKs: https://sdkman.io/sdks
- GitHub: https://github.com/sdkman/sdkman-cli
- Search: "sdkman java versions", "sdkman gradle", "sdkman .sdkmanrc"
