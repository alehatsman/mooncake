# Invalid Config Examples - Error Message Reference

This directory contains examples of invalid configurations to demonstrate the validation error messages.

## Implemented Error Messages

### 1. **Multiple Actions** (`multiple-actions.yml`)
**Error:** Step has both `shell` and `file` actions
**Message:**
```
Step has multiple actions. Only ONE action is allowed per step.
Choose either: shell, template, file, include, include_vars, or vars
```

### 2. **No Action** (`no-action.yml`)
**Error:** Step has no action defined
**Message:**
```
Step must have exactly one action (shell, template, file,
include, include_vars, or vars)
```

### 3. **Invalid File Mode** (`invalid-file-mode.yml`)
**Error:** File mode not in octal format (e.g., "644" instead of "0644")
**Message:**
```
Invalid file mode. Must be in octal format: '0' followed by 3 octal digits
(e.g., '0644', '0755')
```

### 4. **Invalid File State** (`invalid-file-state.yml`)
**Error:** File state is not one of the allowed values
**Message:**
```
Invalid file state. Must be one of: 'file', 'directory', or 'absent'
```

### 5. **Type Errors**
**Context-specific messages for different type mismatches:**

- String expected, got number:
  ```
  Expected text value, but got a number.
  Wrap it in quotes (e.g., "123" instead of 123)
  ```

- Boolean expected:
  ```
  Expected true or false (boolean), not text or number
  ```

- Object expected:
  ```
  Expected an object with fields (like 'path:', 'state:'),
  but got a simple value
  ```

- Array expected:
  ```
  Expected a list of items, but got a single value
  ```

### 6. **Missing Required Fields** (`template-missing-fields.yml`)
**Context-aware messages for specific fields:**

- Missing `src` in template:
  ```
  Missing required field 'src'. Template needs a source file path
  (e.g., src: ./template.j2)
  ```

- Missing `dest` in template:
  ```
  Missing required field 'dest'. Template needs a destination path
  (e.g., dest: /etc/config.conf)
  ```

- Missing `path` in file:
  ```
  Missing required field 'path'. File action needs a file or directory path
  (e.g., path: /tmp/myfile)
  ```

### 7. **Unknown Fields with Typo Suggestions** (`common-typos.yml`)
**Smart suggestions for common mistakes:**

- `command` → "Did you mean 'shell'?"
- `source` → "Did you mean 'src'?"
- `destination` → "Did you mean 'dest'?"
- `condition` → "Did you mean 'when'?"
- `sudo` → "Did you mean 'become'?"
- `tag` → "Did you mean 'tags'?"
- `variable` → "Did you mean 'vars'?"
- And many more...

**Note:** Due to YAML parsing limitations, unknown fields are detected only if they cause the step to have no valid action.

### 8. **String Length Validation**
- **Too short:**
  ```
  Value is too short. Must be at least X characters
  ```

- **Too long:**
  ```
  Value is too long. Must be at most X characters
  ```

### 9. **Number Range Validation**
- **Too small:**
  ```
  Value is too small. Must be at least X
  ```

- **Too large:**
  ```
  Value is too large. Must be at most X
  ```

### 10. **Format Validation**
**Context-specific for different formats:**

- Email: "Invalid email format. Must be like: user@example.com"
- URL: "Invalid URL format. Must be like: https://example.com/path"
- IPv4: "Invalid IPv4 address. Must be like: 192.168.1.1"
- Date/Time: "Invalid date/time format. Check syntax"

### 11. **List Validation**
- **Too few items:** "List must have at least the minimum number of items"
- **Too many items:** "List has too many items. Reduce the number of items"
- **Duplicates:** "List contains duplicate items. Each item must be unique"

## Error Output Format

All errors are displayed with:
- **File path**
- **Line number**
- **User-friendly message**
- **YAML context** (the actual line from the file)
- **Step name** (if applicable)

Example:
```
Error: /path/to/config.yml

  Line 21: Invalid file mode. Must be in octal format: '0' followed by 3 octal digits (e.g., '0644', '0755')
    mode: "777"
    (in step: Create config file)

Found 1 error(s)
```

## Coverage

The error messages cover:
- ✓ Pattern validation (regex patterns)
- ✓ Enum validation (allowed values)
- ✓ Type validation (string, number, boolean, object, array)
- ✓ Required fields
- ✓ Additional/unknown properties
- ✓ OneOf constraints (mutually exclusive fields)
- ✓ String length constraints
- ✓ Number range constraints
- ✓ Format validation (email, URL, IP, dates)
- ✓ Array constraints (min/max items, uniqueness)

## How It Works

1. **JSON Schema validation** catches structural errors
2. **Error type detection** identifies the validation keyword
3. **Context analysis** examines the field path and error details
4. **Message generation** creates user-friendly, actionable messages
5. **Formatting** adds YAML context and step names for clarity
