# Java Preset

Install OpenJDK Java Development Kit with automatic JAVA_HOME configuration.

## Quick Start

```yaml
- preset: java
  with:
    version: "21"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `state` | string | `present` | `present` or `absent` |
| `version` | string | `21` | Java version (11, 17, 21) |
| `set_java_home` | bool | `true` | Set JAVA_HOME env variable |

## Usage

### Latest LTS (Java 21)
```yaml
- preset: java
```

### Specific Version
```yaml
- preset: java
  with:
    version: "17"
```

### Without JAVA_HOME
```yaml
- preset: java
  with:
    set_java_home: false
```

## Verify Installation

```bash
# Check version
java -version
javac -version

# Check JAVA_HOME
echo $JAVA_HOME

# Compile and run
echo 'public class Hello { public static void main(String[] args) { System.out.println("Hello!"); }}' > Hello.java
javac Hello.java
java Hello
```

## Common Operations

```bash
# Compile Java file
javac MyProgram.java

# Run Java program
java MyProgram

# Create JAR file
jar cvf myapp.jar *.class

# Run JAR file
java -jar myapp.jar

# Check classpath
echo $CLASSPATH

# Run with custom classpath
java -cp lib/*:. MyProgram
```

## Maven

```bash
# Install Maven
brew install maven  # macOS
sudo apt install maven  # Ubuntu

# Create project
mvn archetype:generate -DgroupId=com.example -DartifactId=myapp

# Build
mvn clean package

# Run
mvn exec:java -Dexec.mainClass="com.example.App"
```

## Gradle

```bash
# Install Gradle
brew install gradle  # macOS
sudo apt install gradle  # Ubuntu

# Create project
gradle init

# Build
gradle build

# Run
gradle run
```

## Environment Variables

After installation, these are set:
- `JAVA_HOME` - JDK installation directory
- `PATH` - includes `$JAVA_HOME/bin`

Restart terminal or run:
```bash
source ~/.bashrc  # or ~/.zshrc
```

## Multiple Java Versions

```bash
# macOS - switch versions
export JAVA_HOME=$(/usr/libexec/java_home -v 17)

# Linux - alternatives system
sudo update-alternatives --config java
```

## Uninstall

```yaml
- preset: java
  with:
    state: absent
```
