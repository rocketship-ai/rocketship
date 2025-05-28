# Handlebars Escaping

When your APIs or databases use handlebars syntax (`{{ }}`) for their own templating, you need a way to include literal handlebars in your test data without Rocketship trying to process them as variables. Rocketship provides unlimited-level handlebars escaping using backslashes.

## Basic Escaping Syntax

Use backslashes (`\`) before handlebars to escape them:

```yaml
# Normal variable processing
"message": "Hello {{ user_name }}"           # Processes as variable

# Escaped handlebars (literal)
"template": "Use \\{{ user_id }} in API"    # Outputs: Use {{ user_id }} in API
```

## Unlimited Escape Levels

Rocketship supports unlimited levels of backslash escaping using a mathematical algorithm:

- **Odd number of backslashes**: Produces literal handlebars
- **Even number of backslashes**: Processes variables with remaining backslashes

### Examples

```yaml
# 1 backslash (odd) → literal handlebars
"example1": "Template: \\{{ user_id }}"
# Output: Template: {{ user_id }}

# 2 backslashes (even) → backslash + processed variable  
"example2": "Path: \\\\{{ .vars.api_path }}"
# Output: Path: \staging/api

# 3 backslashes (odd) → backslash + literal handlebars
"example3": "Docs: \\\\\\{{ variable_name }}"
# Output: Docs: \{{ variable_name }}

# 4 backslashes (even) → double backslash + processed variable
"example4": "Config: \\\\\\\\{{ .vars.environment }}"
# Output: Config: \\staging
```

## JSON Context Considerations

When using escaped handlebars in JSON, remember that JSON has its own escaping rules. In YAML files, you may need to double backslashes:

```yaml
# In YAML block literal (recommended)
body: |-
  {
    "instructions": "Use \\{{ user_id }} in your requests",
    "template_guide": "Syntax: \\{{ variable_name }} for literals"
  }

# In YAML quoted strings (requires more escaping)
body: |
  {
    "instructions": "Use \\\\{{ user_id }} in your requests"
  }
```

## Complete Working Example

Here's a real example from the config-variables test suite:

```yaml
name: "Handlebars Escaping Demo"
version: "v1.0.0"

vars:
  base_url: "https://tryme.rocketship.sh"
  environment: "staging"

tests:
  - name: "Config Variables Demo"
    steps:
      - name: "Demo handlebars escaping in JSON body"
        plugin: "http"
        config:
          method: "POST"
          url: "{{ .vars.base_url }}/books"
          headers:
            "Content-Type": "application/json"
          body: |-
            {
              "title": "Handlebars Escaping Demo",
              "author": "{{ .vars.environment }}",
              "description": "Normal var: {{ .vars.environment }}, Escaped: \\{{ user_id }}",
              "api_docs": "Use \\{{ user_id }} in your requests",
              "template_guide": "Syntax: \\{{ variable_name }} for literals",
              "mixed_example": "Real: {{ .vars.environment }}, Literal: \\{{ placeholder }}"
            }
        assertions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: ".description"
            expected: "Normal var: staging, Escaped: {{ user_id }}"
          - type: "json_path"
            path: ".api_docs"
            expected: "Use {{ user_id }} in your requests"
```

## SQL Context Example

Handlebars escaping also works in SQL statements:

```yaml
- name: "Demo handlebars escaping in SQL"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .vars.postgres_dsn }}"
    commands:
      - |-
        INSERT INTO users (name, instructions) 
        VALUES ('{{ .vars.test_user_name }}', 'Use \\{{ user_token }} for auth');
  assertions:
    - type: column_value
      query_index: 0
      row_index: 0
      column: "instructions"
      expected: "Use {{ user_token }} for auth"
```

## Common Use Cases

### 1. API Documentation
When testing APIs that return documentation containing template syntax:

```yaml
"help_text": "Use \\{{ user_id }} and \\{{ api_key }} in your requests"
```

### 2. Template Systems
When testing systems that process their own templates:

```yaml
"mustache_template": "Hello \\{{ name }}, welcome to \\{{ site_name }}"
"handlebars_template": "{{#each items}}\\{{ this.name }}{{/each}}"
```

### 3. Configuration Examples
When testing APIs that return configuration examples:

```yaml
"config_example": "api_url: \\{{ environment.api_url }}"
"yaml_template": "name: \\{{ project.name }}\\nversion: \\{{ project.version }}"
```

### 4. Code Generation
When testing code generation APIs:

```yaml
"javascript_template": "const userId = \\{{ user.id }};"
"go_template": "UserID: \\{{ .User.ID }}"
```

## How It Works

Rocketship's escaping algorithm:

1. **Counts consecutive backslashes** before `{{ }}`
2. **Determines behavior** based on count:
   - Odd count: Treat as literal handlebars
   - Even count: Process as variable
3. **Calculates remaining backslashes**: `count / 2` (integer division)
4. **Outputs result** with appropriate backslashes

This mathematical approach enables unlimited nesting levels, giving you complete control over handlebars rendering.

## Best Practices

### 1. Use Block Literals for JSON
Use YAML block literal syntax (`|-`) for cleaner JSON:

```yaml
body: |-
  {
    "template": "Use \\{{ variable }} here"
  }
```

### 2. Test Your Escaping
Always verify your escaping works by checking the actual output:

```yaml
assertions:
  - type: "json_path"
    path: ".template"
    expected: "Use {{ variable }} here"  # Verify literal handlebars
```

### 3. Document Your Intent
Add comments to clarify when you're using escaping:

```yaml
# This should output literal {{ user_id }}, not process as variable
"instructions": "Use \\{{ user_id }} in your API calls"
```

### 4. Start Simple
Begin with single backslash escaping and only use multiple levels if needed:

```yaml
# Usually sufficient for most cases
"example": "Template: \\{{ variable }}"
```

## Running the Examples

Test the handlebars escaping functionality:

```bash
# Run config-variables example (includes escaping demo)
rocketship run -af examples/config-variables/rocketship.yaml

# Run SQL escaping example  
rocketship run -af examples/sql-testing/rocketship.yaml
```

The examples demonstrate real-world usage of handlebars escaping in both HTTP and SQL contexts.