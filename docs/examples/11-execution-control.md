# Example 11: Shell Execution Control

Advanced execution control for shell commands with timeouts, retries, and custom result evaluation.

**Note:** These features are specific to shell commands. File and template operations don't support timeout, retries, env, cwd, changed_when, or failed_when fields.

## Timeouts

Prevent commands from running too long:

```yaml
- name: Command with timeout
  shell: ./long-running-script.sh
  timeout: 30s

- name: Build with timeout
  shell: make build
  timeout: 10m
  cwd: /opt/project
```

Timeout exit code is 124 (standard timeout exit code).

## Retries and Delays

Automatically retry failed commands:

```yaml
- name: Download file with retries
  shell: curl -O https://example.com/file.tar.gz
  retries: 5
  retry_delay: 10s

- name: Wait for service
  shell: nc -z localhost 8080
  retries: 10
  retry_delay: 2s
  failed_when: "result.rc != 0"
```

## Environment Variables

Set custom environment variables:

```yaml
- name: Build with custom environment
  shell: make build
  env:
    CC: gcc-11
    CFLAGS: "-O2 -Wall"
    MAKEFLAGS: "-j4"

- name: Run tests with env
  shell: npm test
  env:
    NODE_ENV: test
    DEBUG: "app:*"
```

### Template Variables in Env

```yaml
- vars:
    build_type: release
    num_cores: 4

- name: Compile with variables
  shell: cmake --build .
  env:
    BUILD_TYPE: "{{build_type}}"
    CMAKE_BUILD_PARALLEL_LEVEL: "{{num_cores}}"
```

## Working Directory

Change directory before execution:

```yaml
- name: Build in project directory
  shell: npm install && npm run build
  cwd: /opt/myproject

- name: Run tests from subdir
  shell: pytest tests/
  cwd: "{{project_root}}/backend"
```

## Custom Change Detection

Override whether a step reports as changed:

```yaml
- name: Git pull (only changed if updated)
  shell: git pull
  register: git_result
  changed_when: "'Already up to date' not in result.stdout"

- name: Restart if config changed
  shell: systemctl restart nginx
  become: true
  when: config.changed == true
```

### Always/Never Changed

```yaml
- name: Read-only command (never changed)
  shell: cat /etc/os-release
  changed_when: false

- name: Force changed status
  shell: echo "notify handler"
  changed_when: true
```

## Custom Failure Detection

Override when a command is considered failed:

```yaml
- name: Grep (0=found, 1=not found, 2+=error)
  shell: grep "pattern" file.txt
  failed_when: "result.rc >= 2"

- name: Command with acceptable exit codes
  shell: ./script.sh
  failed_when: "result.rc not in [0, 2, 3]"

- name: Check stderr for errors
  shell: ./noisy-command.sh
  failed_when: "'ERROR' in result.stderr or 'FATAL' in result.stderr"
```

## Privilege Escalation

Run as different users:

```yaml
- name: Run as root
  shell: systemctl restart nginx
  become: true

- name: Run as postgres user
  shell: psql -c "SELECT version()"
  become: true
  become_user: postgres

- name: Run as application user
  shell: ./manage.py migrate
  become: true
  become_user: appuser
  cwd: /opt/application
```

## Complete Example: Robust Deployment

```yaml
- name: Stop application
  shell: systemctl stop myapp
  become: true
  timeout: 30s

- name: Backup current version
  shell: |
    backup_file="/backup/myapp-$(date +%Y%m%d-%H%M%S).tar.gz"
    tar czf "$backup_file" /opt/myapp
    echo "Backed up to $backup_file"
  timeout: 5m
  register: backup_result

- name: Download new version
  shell: curl -o /tmp/myapp.tar.gz https://releases.example.com/myapp-{{version}}.tar.gz
  timeout: 10m
  retries: 3
  retry_delay: 30s

- name: Extract application
  shell: |
    rm -rf /opt/myapp
    tar xzf /tmp/myapp.tar.gz -C /opt
  become: true
  timeout: 2m

- name: Install dependencies
  shell: pip install -r requirements.txt
  cwd: /opt/myapp
  become: true
  become_user: appuser
  timeout: 5m
  env:
    PIP_INDEX_URL: "{{pip_mirror}}"

- name: Run database migrations
  shell: ./manage.py migrate
  cwd: /opt/myapp
  become: true
  become_user: appuser
  timeout: 10m
  register: migrate_result
  changed_when: "'No migrations to apply' not in result.stdout"
  failed_when: "result.rc != 0"

- name: Start application
  shell: systemctl start myapp
  become: true
  timeout: 30s

- name: Wait for application to be ready
  shell: curl -sf http://localhost:8080/health
  retries: 30
  retry_delay: 2s
  register: health_check
  failed_when: "result.rc != 0"

- name: Verify deployment
  shell: |
    version=$(curl -s http://localhost:8080/version)
    echo "Deployed version: $version"
    test "$version" = "{{expected_version}}"
  register: verify
  failed_when: "result.rc != 0"
```

## Real-World: Service Health Check

```yaml
- name: Check service dependencies
  shell: |
    services="postgresql redis nginx"
    for service in $services; do
      systemctl is-active $service || exit 1
    done
  retries: 5
  retry_delay: 10s
  timeout: 5s
  register: deps_check

- name: Start application service
  shell: systemctl start myapp
  become: true
  when: deps_check.rc == 0

- name: Wait for service to be ready
  shell: curl -sf http://localhost:8080/ready
  retries: 60
  retry_delay: 1s
  timeout: 5s
  register: ready_check
  failed_when: "result.rc != 0"
  changed_when: false  # Health check doesn't change anything

- name: Run smoke tests
  shell: ./run-smoke-tests.sh
  cwd: /opt/myapp/tests
  timeout: 2m
  env:
    TEST_URL: http://localhost:8080
    TEST_ENV: staging
  register: smoke_tests
  failed_when: "result.rc != 0 or 'FAIL' in result.stdout"
```

## See Also

- [Actions Reference](../guide/config/actions.md#common-fields) - Complete field documentation
- [Advanced Configuration](../guide/config/advanced.md#error-handling) - Error handling patterns
- [Example 07: Register](07-register.md) - Using command results
