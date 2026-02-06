# jenv - Java Version Manager

Manage multiple Java versions. Switch JDK per project without changing JAVA_HOME manually.

## Quick Start
```yaml
- preset: jenv
```

## Basic Usage
```bash
# Add Java installation
jenv add /Library/Java/JavaVirtualMachines/openjdk-17.jdk/Contents/Home
jenv add /Library/Java/JavaVirtualMachines/openjdk-11.jdk/Contents/Home

# List versions
jenv versions

# Set version
jenv global 17.0      # System-wide
jenv local 11.0       # Project-specific (.java-version)
jenv shell 21.0       # Current shell

# Show current
jenv version
```

## Shell Integration
```bash
# Bash (~/.bashrc)
export PATH="$HOME/.jenv/bin:$PATH"
eval "$(jenv init -)"

# Zsh (~/.zshrc)
export PATH="$HOME/.jenv/bin:$PATH"
eval "$(jenv init -)"

# Fish (~/.config/fish/config.fish)
set -x PATH $HOME/.jenv/bin $PATH
status --is-interactive; and jenv init - | source
```

## Adding Java Versions
```bash
# macOS - Add system JDKs
jenv add /Library/Java/JavaVirtualMachines/openjdk-17.jdk/Contents/Home
jenv add /Library/Java/JavaVirtualMachines/openjdk-11.jdk/Contents/Home
jenv add /Library/Java/JavaVirtualMachines/openjdk-21.jdk/Contents/Home

# macOS - Add Homebrew JDKs
jenv add /opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
jenv add /opt/homebrew/opt/openjdk@11/libexec/openjdk.jdk/Contents/Home

# Linux - Add Java installations
jenv add /usr/lib/jvm/java-17-openjdk-amd64
jenv add /usr/lib/jvm/java-11-openjdk-amd64

# Verify
jenv versions
```

## Version Selection
```bash
# Global (default)
jenv global 17.0
cat ~/.jenv/version

# Local (project)
jenv local 11.0
cat .java-version

# Shell (session)
jenv shell 21.0
echo $JENV_VERSION

# Check current
jenv version
java -version
```

## .java-version File
```bash
# Create manually
echo "17.0" > .java-version

# Or use jenv
jenv local 17.0

# Auto-switch on cd
# jenv automatically detects .java-version
cd myproject  # Switches to version in .java-version
```

## Plugins
```bash
# Enable export plugin (sets JAVA_HOME)
jenv enable-plugin export

# Enable maven plugin
jenv enable-plugin maven

# Enable gradle plugin
jenv enable-plugin gradle

# Enable ant plugin
jenv enable-plugin ant

# Enable sbt plugin
jenv enable-plugin sbt

# List plugins
jenv plugins

# Disable plugin
jenv disable-plugin maven
```

## JAVA_HOME Management
```bash
# Enable export plugin (REQUIRED for JAVA_HOME)
jenv enable-plugin export

# Verify JAVA_HOME
echo $JAVA_HOME

# JAVA_HOME will now auto-update when switching versions
jenv global 17.0 && echo $JAVA_HOME
jenv global 11.0 && echo $JAVA_HOME
```

## Project Workflows
```bash
# New Spring Boot project
cd myproject
jenv local 17.0
java -version
mvn spring-boot:run

# Clone and setup
git clone repo
cd repo
jenv local  # Uses .java-version
mvn clean install

# Multiple projects
cd project-a && java -version  # Uses Java 11
cd project-b && java -version  # Uses Java 17
```

## Maven Integration
```bash
# Enable maven plugin
jenv enable-plugin maven

# Maven uses jenv's Java
mvn --version

# Run with specific version
jenv local 17.0
mvn clean package

# Multi-version testing
for v in 11.0 17.0 21.0; do
  jenv local $v
  mvn test
done
```

## Gradle Integration
```bash
# Enable gradle plugin
jenv enable-plugin gradle

# Gradle uses jenv's Java
gradle --version

# Build with specific version
jenv local 17.0
./gradlew build

# gradle.properties (alternative)
org.gradle.java.home=/Users/you/.jenv/versions/17.0
```

## Common Commands
```bash
# List versions
jenv versions
jenv versions --bare  # Names only

# Show paths
jenv root             # ~/.jenv
jenv prefix           # Current version path

# Version info
jenv version          # Active version with source
jenv version-name     # Active version number only

# Remove version
jenv remove 11.0

# Refresh versions
jenv refresh-versions
```

## CI/CD Integration
```bash
# GitHub Actions
- name: Setup Java
  uses: actions/setup-java@v3
  with:
    java-version: '17'
    distribution: 'temurin'

# Or with jenv
- name: Setup jenv
  run: |
    git clone https://github.com/jenv/jenv.git ~/.jenv
    echo 'export PATH="$HOME/.jenv/bin:$PATH"' >> $GITHUB_ENV
    echo 'eval "$(jenv init -)"' >> $GITHUB_ENV
    jenv add /usr/lib/jvm/java-17-openjdk-amd64
    jenv global 17.0

# GitLab CI
image: openjdk:17-jdk

before_script:
  - java -version
  - ./mvnw --version

# Docker
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y openjdk-17-jdk git curl
RUN git clone https://github.com/jenv/jenv.git ~/.jenv
ENV PATH="/root/.jenv/bin:$PATH"
RUN eval "$(jenv init -)" && \
    jenv add /usr/lib/jvm/java-17-openjdk-amd64 && \
    jenv global 17.0
```

## Finding Java Installations
```bash
# macOS - List installed JDKs
/usr/libexec/java_home -V

# macOS - Find specific version
/usr/libexec/java_home -v 17

# Linux - Find Java installations
update-alternatives --list java
ls /usr/lib/jvm/

# Add all macOS JDKs
for jdk in /Library/Java/JavaVirtualMachines/*/Contents/Home; do
  jenv add "$jdk"
done

# Add all Linux JDKs
for jdk in /usr/lib/jvm/java-*-openjdk-amd64; do
  jenv add "$jdk"
done
```

## Multiple JDK Distributions
```bash
# Oracle JDK
jenv add /Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home

# OpenJDK
jenv add /Library/Java/JavaVirtualMachines/openjdk-17.jdk/Contents/Home

# Temurin (AdoptOpenJDK)
jenv add /Library/Java/JavaVirtualMachines/temurin-17.jdk/Contents/Home

# Azul Zulu
jenv add /Library/Java/JavaVirtualMachines/zulu-17.jdk/Contents/Home

# GraalVM
jenv add /Library/Java/JavaVirtualMachines/graalvm-ce-java17/Contents/Home
```

## Environment Setup
```bash
# Full setup script (macOS)
#!/bin/bash
# Install jenv
git clone https://github.com/jenv/jenv.git ~/.jenv

# Add to shell
echo 'export PATH="$HOME/.jenv/bin:$PATH"' >> ~/.zshrc
echo 'eval "$(jenv init -)"' >> ~/.zshrc

# Enable plugins
jenv enable-plugin export
jenv enable-plugin maven
jenv enable-plugin gradle

# Add Java versions
for jdk in /Library/Java/JavaVirtualMachines/*/Contents/Home; do
  jenv add "$jdk"
done

# Set default
jenv global 17.0
```

## Troubleshooting
```bash
# Java version not changing
jenv version  # Check which version and source
which java    # Check java path

# JAVA_HOME not set
jenv enable-plugin export
source ~/.zshrc

# Maven/Gradle using wrong Java
jenv enable-plugin maven
jenv enable-plugin gradle
jenv rehash

# Version not found
jenv versions       # List installed
jenv add /path/to/jdk  # Add missing version

# Shims not working
jenv rehash
```

## Advanced Usage
```bash
# Use specific Java for command
jenv exec 11.0 java MyApp

# Rehash shims
jenv rehash

# Doctor command
jenv doctor

# Configuration
jenv root          # ~/.jenv
jenv prefix 17.0   # Path to version

# Unset version
jenv local --unset
jenv shell --unset
```

## Comparison
| Feature | jenv | SDKMAN! | jabba | asdf |
|---------|------|---------|-------|------|
| Platform | *nix | All | All | All |
| Auto-install | No | Yes | Yes | Yes |
| JAVA_HOME | Plugin | Auto | Auto | Plugin |
| Build tools | Plugins | Yes | No | Plugins |
| Simplicity | Simple | Feature-rich | Simple | Universal |

## Best Practices
- **Enable export plugin** for JAVA_HOME support
- **Use .java-version** for project consistency
- **Commit .java-version** to git
- **Enable build tool plugins** (maven, gradle)
- **Add all JDK versions** upfront
- **Set sensible global default** (LTS version)
- **Test on multiple versions** before release

## Tips
- Lightweight shim-based solution
- No sudo required
- Per-project version control
- Build tool integration
- Works with all JDK distributions
- Shell completion support
- Automatic version switching

## Agent Use
- Automated Java version management
- CI/CD pipeline setup
- Multi-version testing
- Development environment setup
- Team version consistency
- Container image builds

## Uninstall
```yaml
- preset: jenv
  with:
    state: absent
```

## Resources
- GitHub: https://github.com/jenv/jenv
- Docs: https://www.jenv.be/
- Search: "jenv add java", "jenv local"
