# Clojure - Functional Programming Language

Dynamic functional programming language that runs on the JVM with a focus on immutability and concurrency.

## Quick Start
```yaml
- preset: clojure
```

## Features
- **Functional**: First-class functions, immutable data structures
- **JVM integration**: Full access to Java libraries and ecosystem
- **REPL-driven**: Interactive development workflow
- **Lisp syntax**: Powerful macro system for metaprogramming
- **Concurrency**: Software transactional memory, agents, atoms
- **Modern**: Active development and growing ecosystem

## Basic Usage
```bash
# Start REPL
clj

# Run Clojure file
clj -M script.clj

# Run with main function
clj -M -m myapp.core

# Evaluate expression
clj -e '(println "Hello, World!")'

# Install dependencies and run
clj -M:run
```

## Project Structure

### deps.edn
```clojure
{:deps {org.clojure/clojure {:mvn/version "1.11.1"}
        compojure/compojure {:mvn/version "1.7.0"}
        ring/ring-core {:mvn/version "1.9.6"}}

 :aliases {:run {:main-opts ["-m" "myapp.core"]}
           :test {:extra-paths ["test"]
                  :extra-deps {lambdaisland/kaocha {:mvn/version "1.77.1236"}}}}}
```

### Example Code
```clojure
;; src/myapp/core.clj
(ns myapp.core
  (:require [compojure.core :refer [defroutes GET POST]]
            [ring.adapter.jetty :refer [run-jetty]]))

(defn handler [request]
  {:status 200
   :headers {"Content-Type" "text/plain"}
   :body "Hello from Clojure!"})

(defroutes app-routes
  (GET "/" [] handler)
  (GET "/api/status" [] {:status 200 :body "OK"}))

(defn -main [& args]
  (run-jetty app-routes {:port 3000}))
```

## REPL-Driven Development
```clojure
;; Start REPL, then evaluate expressions

;; Define function
(defn greet [name]
  (str "Hello, " name "!"))

;; Test it
(greet "World")
;; => "Hello, World!"

;; Work with data
(def users [{:name "Alice" :age 30}
            {:name "Bob" :age 25}])

(filter #(> (:age %) 26) users)
;; => ({:name "Alice" :age 30})

;; Map operations
(map :name users)
;; => ("Alice" "Bob")
```

## Real-World Examples

### Web Application Deployment
```yaml
- name: Install Clojure
  preset: clojure

- name: Clone application
  shell: git clone https://github.com/user/clojure-app.git /app

- name: Run tests
  shell: clj -X:test
  cwd: /app

- name: Build uberjar
  shell: clj -T:build uber
  cwd: /app

- name: Run application
  shell: java -jar target/app-standalone.jar
  cwd: /app
```

### REPL Development
```yaml
- name: Start REPL with dependencies
  shell: clj -M:dev:test
  cwd: /project
```

### Data Processing Script
```clojure
#!/usr/bin/env clj

(require '[clojure.data.json :as json]
         '[clojure.java.io :as io])

(defn process-file [file]
  (with-open [reader (io/reader file)]
    (->> (line-seq reader)
         (map json/read-str)
         (filter #(> (get % "value") 100))
         (map #(select-keys % ["id" "value"]))
         doall)))

(defn -main [file]
  (let [results (process-file file)]
    (println (json/write-str results))))

(-main (first *command-line-args*))
```

## Tools and Build

### Leiningen (alternative)
```bash
# Create new project
lein new app myapp

# Run project
lein run

# Build uberjar
lein uberjar

# Run tests
lein test
```

## Platform Support
- ✅ Linux (package managers, manual)
- ✅ macOS (Homebrew)
- ✅ Windows (installer)

## Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| state | string | present | Whether to install (present) or remove (absent) |

## Agent Use
- Build and deploy Clojure applications
- Run data processing scripts
- Execute functional transformations
- Integrate with JVM ecosystem
- Develop interactive applications with REPL


## Advanced Configuration
```yaml
# Use with Mooncake preset system
- name: Install clojure
  preset: clojure

- name: Use clojure in automation
  shell: |
    # Custom configuration here
    echo "clojure configured"
```
## Uninstall
```yaml
- preset: clojure
  with:
    state: absent
```

## Resources
- Official site: https://clojure.org
- Getting started: https://clojure.org/guides/getting_started
- Clojure docs: https://clojuredocs.org
- GitHub: https://github.com/clojure/clojure
- Search: "clojure tutorial", "clojure web development", "clojure examples"
