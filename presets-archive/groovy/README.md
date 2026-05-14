# Groovy - Dynamic JVM Language

Powerful scripting and programming language for the JVM. Java-compatible with concise syntax, closures, and dynamic features.

## Quick Start
```yaml
- preset: groovy
```

## Features
- **Java compatible**: Use any Java library seamlessly
- **Concise syntax**: Less boilerplate than Java
- **Dynamic typing**: Optional static typing for flexibility
- **Closures**: First-class functions and functional programming
- **DSL creation**: Build domain-specific languages easily
- **Scripting**: Run Groovy as scripts or compile to bytecode

## Basic Usage
```bash
# Run Groovy script
groovy script.groovy

# Interactive console
groovysh

# Compile to class files
groovyc MyClass.groovy

# Run compiled class
java -cp .:/path/to/groovy-all.jar MyClass

# Run one-liner
groovy -e 'println "Hello, World!"'

# Check version
groovy --version
```

## Advanced Configuration
```yaml
- preset: groovy
  with:
    state: present
```

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Install or remove Groovy |

## Platform Support
- ✅ Linux (SDKMAN, package managers, binary download)
- ✅ macOS (Homebrew, SDKMAN, binary download)
- ✅ Windows (Scoop, Chocolatey, binary download)

## Configuration
- **GROOVY_HOME**: Installation directory
- **Classpath**: Access all Java libraries
- **Grape**: Dependency management (@Grab annotations)

## Real-World Examples

### Hello World Script
```groovy
#!/usr/bin/env groovy
// hello.groovy
println "Hello, World!"

// Variables (no type declaration needed)
def name = "Alice"
def age = 30
println "Name: $name, Age: $age"

// Lists and maps
def fruits = ['apple', 'banana', 'orange']
def person = [name: 'Bob', age: 25, city: 'NYC']

fruits.each { println it }
person.each { k, v -> println "$k: $v" }
```

### File Processing
```groovy
// Read file
new File('data.txt').eachLine { line ->
    println line.toUpperCase()
}

// Write file
new File('output.txt').withWriter { writer ->
    writer.writeLine('Hello from Groovy')
}

// Process CSV
new File('users.csv').splitEachLine(',') { fields ->
    println "Name: ${fields[0]}, Email: ${fields[1]}"
}

// JSON processing
@Grab('org.codehaus.groovy:groovy-json:3.0.19')
import groovy.json.*

def json = new JsonSlurper().parseText('{"name":"Alice","age":30}')
println json.name

def output = new JsonBuilder([name: 'Bob', age: 25])
println output.toPrettyString()
```

### REST API Client
```groovy
@Grab('org.codehaus.groovy:groovy-json:3.0.19')
import groovy.json.*

// GET request
def url = 'https://api.github.com/users/octocat'.toURL()
def json = new JsonSlurper().parse(url)
println "User: ${json.login}, Followers: ${json.followers}"

// POST request
def connection = new URL('https://httpbin.org/post').openConnection()
connection.setRequestMethod('POST')
connection.doOutput = true
connection.setRequestProperty('Content-Type', 'application/json')

def body = new JsonBuilder([name: 'Alice', age: 30])
connection.outputStream.withWriter { writer ->
    writer << body.toString()
}

def response = new JsonSlurper().parse(connection.inputStream)
println response
```

### Build Automation (Gradle)
```groovy
// build.gradle
plugins {
    id 'java'
    id 'application'
}

repositories {
    mavenCentral()
}

dependencies {
    implementation 'com.google.guava:guava:32.1.3-jre'
    testImplementation 'junit:junit:4.13.2'
}

application {
    mainClass = 'com.example.Main'
}

// Custom task
task copyDocs(type: Copy) {
    from 'docs'
    into "$buildDir/docs"
}

// Groovy DSL for configuration
task hello {
    doLast {
        println 'Hello from Gradle!'
    }
}
```

### Jenkins Pipeline
```groovy
// Jenkinsfile
pipeline {
    agent any

    stages {
        stage('Build') {
            steps {
                sh './gradlew clean build'
            }
        }

        stage('Test') {
            steps {
                sh './gradlew test'
            }
        }

        stage('Deploy') {
            when {
                branch 'main'
            }
            steps {
                sh './deploy.sh'
            }
        }
    }

    post {
        always {
            junit 'build/test-results/**/*.xml'
        }
        success {
            echo 'Build succeeded!'
        }
        failure {
            echo 'Build failed!'
        }
    }
}
```

### Database Access
```groovy
@Grab('org.postgresql:postgresql:42.7.1')
import groovy.sql.Sql

def sql = Sql.newInstance(
    'jdbc:postgresql://localhost:5432/mydb',
    'user',
    'password',
    'org.postgresql.Driver'
)

// Query
sql.eachRow('SELECT * FROM users') { row ->
    println "${row.name} - ${row.email}"
}

// Insert
sql.execute('''
    INSERT INTO users (name, email)
    VALUES (?, ?)
''', ['Alice', 'alice@example.com'])

// Transaction
sql.withTransaction {
    sql.execute('UPDATE users SET active = ? WHERE id = ?', [true, 1])
    sql.execute('INSERT INTO audit_log (action) VALUES (?)', ['user_updated'])
}

sql.close()
```

### Object-Oriented Groovy
```groovy
class Person {
    String name
    int age

    // Constructor
    Person(String name, int age) {
        this.name = name
        this.age = age
    }

    // Method
    def greet() {
        println "Hello, I'm $name and I'm $age years old"
    }
}

// Traits (mixins)
trait Swimmer {
    def swim() {
        println "$name is swimming"
    }
}

class Athlete extends Person implements Swimmer {
    String sport

    Athlete(String name, int age, String sport) {
        super(name, age)
        this.sport = sport
    }
}

def athlete = new Athlete('Bob', 25, 'Swimming')
athlete.greet()
athlete.swim()
```

## Agent Use
- Write automation scripts for build systems
- Create Jenkins pipelines for CI/CD
- Build Gradle plugins and tasks
- Process JSON/XML data in ETL pipelines
- Generate code from templates
- Implement DSLs for configuration

## Troubleshooting

### Command not found
```bash
# Check installation
which groovy
groovy --version

# Set GROOVY_HOME
export GROOVY_HOME=/usr/local/opt/groovy/libexec
export PATH=$GROOVY_HOME/bin:$PATH

# Verify
groovy -e 'println "OK"'
```

### Grape dependency errors
```bash
# Clear Grape cache
rm -rf ~/.groovy/grapes

# Use specific repository
@GrabResolver(name='central', root='https://repo1.maven.org/maven2/')
@Grab('group:artifact:version')

# Offline mode (use cached dependencies)
groovy --offline script.groovy
```

### Out of memory
```bash
# Increase heap size
export JAVA_OPTS="-Xmx2g"
groovy script.groovy

# Or in script
export GROOVY_OPTS="-Xmx2g"
```

### Compilation errors
```bash
# Enable static compilation
@groovy.transform.CompileStatic
class MyClass {
    // Type-checked at compile time
}

# Check syntax
groovyc --compile-static MyClass.groovy

# Use groovysh for debugging
groovysh
groovy:000> println "test"
```

## Uninstall
```yaml
- preset: groovy
  with:
    state: absent
```

## Resources
- Official docs: https://groovy-lang.org/documentation.html
- Getting started: https://groovy-lang.org/learn.html
- Groovy API: https://docs.groovy-lang.org/latest/html/api/
- Grape: https://groovy-lang.org/grape.html
- Search: "groovy tutorial", "groovy vs java", "groovy scripting"
