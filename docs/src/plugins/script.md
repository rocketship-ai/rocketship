# Script Plugin

Execute custom JavaScript or shell scripts for data processing, validation, and system operations.

## Quick Start

```yaml
- name: "Process data"
  plugin: script
  config:
    language: javascript
    script: |
      let userName = state.user_name.toUpperCase();
      save("display_name", userName);
      assert(userName.length > 0, "Name required");
```

## Configuration

| Field | Description | Example |
|-------|-------------|---------|
| `language` | Script language | `javascript`, `shell` |
| `script` | Inline script content | See examples below |
| `file` | Path to external script file | `./scripts/process.js` |
| `timeout` | Execution timeout | `30s` (default: 2m) |

Note: Must provide either `script` or `file`, not both.

## JavaScript

### Built-in Functions

```javascript
// Save data for later steps
save("key", "value");

// Validate data (fails test if false)
assert(condition, "Error message");
```

### Accessing Data

```javascript
// Config variables
let apiUrl = vars.api_url;

// Runtime variables (from previous steps)
let userId = state.user_id;

// Type conversions (state values are strings)
let count = parseInt(state.count);
let price = parseFloat(state.price);
let isActive = state.active === "true";
```

### Common Patterns

```yaml
# Data transformation
- name: "Process response"
  plugin: script
  config:
    language: javascript
    script: |
      let name = state.user_name.toUpperCase();
      let age = parseInt(state.user_age);
      save("display_name", name);
      save("is_adult", (age >= 18).toString());

# Validation
- name: "Validate order"
  plugin: script
  config:
    language: javascript
    script: |
      let total = parseFloat(state.order_total);
      assert(total > 0, "Total must be positive");

# Generate test data
- name: "Create unique data"
  plugin: script
  config:
    language: javascript
    script: |
      const suffix = Date.now();
      save("test_email", `test-${suffix}@example.com`);
```

## Shell

### Common Patterns

```yaml
# Build operations
- name: "Build app"
  plugin: script
  config:
    language: shell
    script: |
      set -euo pipefail
      npm install
      npm run build

# File operations
- name: "Create package"
  plugin: script
  config:
    language: shell
    script: |
      tar -czf "{{ .vars.project }}-{{ .vars.version }}.tar.gz" dist/

# Docker operations
- name: "Deploy container"
  plugin: script
  config:
    language: shell
    timeout: "5m"
    script: |
      set -euo pipefail
      docker build -t myapp:{{ .vars.version }} .
      docker run -d myapp:{{ .vars.version }}
```

### Variable Access

Shell scripts receive variables as environment variables:

```bash
# Config variables as ROCKETSHIP_VAR_*
echo "Project: $ROCKETSHIP_VAR_PROJECT_NAME"

# State variables as ROCKETSHIP_*
echo "Build ID: $ROCKETSHIP_BUILD_ID"
```

Or use template syntax:

```yaml
script: |
  echo "Building {{ .vars.project }} version {{ .vars.version }}"
```

## Best Practices

### JavaScript
- **Keep scripts focused**: Use for data processing, not HTTP operations
- **External files for complexity**: Use `file` for scripts > 20 lines
- **Clear error messages**: Write descriptive assertion messages
- **Type conversions**: Remember state values are always strings

### Shell
- **Use `set -euo pipefail`**: Exit on errors, undefined vars, pipe failures
- **Quote variables**: Handle spaces with `"{{ .vars.name }}"`
- **Clean up**: Use trap functions for cleanup on exit/error

## See Also

- [Variables](../features/variables.md) - Accessing config and runtime variables
- [HTTP Plugin](http.md) - Making API calls
- [SQL Plugin](sql.md) - Database operations
