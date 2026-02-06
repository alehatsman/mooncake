# Example 11: Execution Control

This example demonstrates advanced execution control features in Mooncake.

## Features Demonstrated

- **Timeouts**: Prevent commands from running too long
- **Retries**: Automatically retry failed commands
- **Retry Delays**: Wait between retry attempts
- **Environment Variables**: Set custom environment for commands
- **Working Directory**: Execute commands in specific directories
- **Changed When**: Custom logic to determine if a step made changes
- **Failed When**: Custom logic to determine if a step failed
- **Become User**: Run commands as different users (see note below)

## Running the Example

```bash
# Run all examples
mooncake run --config config.yml

# Preview what will run
mooncake run --config config.yml --dry-run

# With debug logging
mooncake run --config config.yml --log-level debug
```

## What Each Example Shows

### Example 1: Basic Timeout
Shows how to set a timeout to prevent commands from hanging indefinitely.

### Example 2: Retry with Delay
Demonstrates automatic retry of failed commands with configurable delay between attempts.

### Example 3: Environment Variables
Shows how to set custom environment variables, including template variable expansion.

### Example 4: Working Directory
Demonstrates changing the working directory before executing a command.

### Example 5: Custom Change Detection
Shows how to mark a command as "unchanged" even though it runs.

### Example 6: Git-Style Change Detection
Demonstrates detecting changes based on command output (common pattern with git).

### Example 7: Custom Failure Detection (grep)
Shows how to handle commands where certain non-zero exit codes are acceptable.

### Example 8: Acceptable Exit Codes
Demonstrates accepting multiple exit codes as success.

### Example 9: Combined Features
Shows using timeout and retry together for robust command execution.

### Example 10: Full Featured
Demonstrates using multiple execution control features together.

## Real-World Applications

These features are essential for production deployments:

- **Timeouts**: Prevent CI/CD pipelines from hanging
- **Retries**: Handle flaky network requests, service startups
- **Environment Variables**: Configure build tools, set API keys
- **Working Directory**: Build projects, run tests in correct locations
- **changed_when**: Accurate change reporting, trigger handlers correctly
- **failed_when**: Handle tools with non-standard exit codes

## Note on become_user

The `become_user` feature (running as different users) is not demonstrated in this example as it requires:

- Root privileges
- Sudo password
- Specific users to exist on the system

To use `become_user` in your own configs:

```yaml
- name: Run as postgres user
  shell: psql -c "SELECT version()"
  become: true
  become_user: postgres
  # Requires: mooncake run --config config.yml --sudo-pass <password>
```

## See Also

- [Execution Control Documentation](../../docs/examples/11-execution-control.md)
- [Actions Reference](../../docs/guide/config/actions.md#common-fields)
- [Register Example](../07-register/) - Capturing command output
