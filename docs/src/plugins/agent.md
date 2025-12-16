# Agent Plugin

AI-powered testing using Claude for browser control, verification, and multi-step workflows.

## Quick Start

```yaml
- name: "Verify login page"
  plugin: agent
  config:
    prompt: |
      Navigate to {{ .env.FRONTEND_URL }}/login and verify:
      - Login form is visible
      - Email and password fields exist
      - Submit button is present
```

## Prerequisites

```bash
# Set Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-your-key-here
```

## Configuration

### Required Fields

| Field    | Description                      | Example                                    |
| -------- | -------------------------------- | ------------------------------------------ |
| `prompt` | Natural language task for Claude | `"Click login button and verify redirect"` |

### Optional Fields

| Field       | Description               | Default     |
| ----------- | ------------------------- | ----------- |
| `max_turns` | Max agent loop iterations | unlimited   |
| `timeout`   | Max execution time        | unlimited   |

## Common Use Cases

### Login Flow

```yaml
- name: "Complete login"
  plugin: agent
  config:
    prompt: |
      Navigate to {{ .env.FRONTEND_URL }}/login and login with:
      - Email: {{ .env.TEST_EMAIL }}
      - Password: {{ .env.TEST_PASSWORD }}

      Verify you land on the dashboard page.
    timeout: "2m"
```

### Multi-Step Workflows

```yaml
- name: "Complete checkout"
  plugin: agent
  config:
    prompt: |
      Complete a checkout flow:
      1. Add product ID {{ product_id }} to cart
      2. Navigate to cart
      3. Click checkout
      4. Fill shipping form with test data
      5. Verify order summary shows {{ product_id }}
    max_turns: 15
    timeout: "3m"
  save:
    - json_path: ".result"
      as: "checkout_result"
```

### Data Extraction

```yaml
- name: "Extract pricing"
  plugin: agent
  config:
    prompt: |
      Go to /pricing and extract all plan names and prices.
      Return as JSON: [{"name": "...", "price": "..."}]
  save:
    - json_path: ".result"
      as: "pricing_plans"
```

### Combining with Database

```yaml
- name: "Get expected count from database"
  plugin: supabase
  config:
    operation: "select"
    table: "vehicles"
    select:
      columns: ["id"]
  save:
    - json_path: ". | length"
      as: "vehicle_count"

- name: "Verify UI matches database"
  plugin: agent
  config:
    prompt: |
      Navigate to /fleet and verify:
      - Map displays {{ vehicle_count }} vehicle markers
      - Vehicle cards show {{ vehicle_count }} vehicles
```

## Complete Example

```yaml
name: "Login Test"
tests:
  - name: "Agent-powered login flow"
    steps:
      - name: "Login and verify"
        plugin: agent
        config:
          prompt: |
            1. Navigate to {{ .env.FRONTEND_URL }}/login
            2. Fill email: {{ .env.TEST_EMAIL }}
            3. Fill password: {{ .env.TEST_PASSWORD }}
            4. Click login button
            5. Verify you're on the dashboard page
          max_turns: 10
          timeout: "2m"
```

## Best Practices

- **Be specific**: `"Click 'Add to Cart' for product 123"` not `"Add something"`
- **Set timeouts**: 30s (simple), 2m (multi-step), 5m (complex)
- **Use variables**: Pass dynamic data via `{{ variable_name }}`
- **Handle dynamic content**: `"Wait for spinner to disappear, then verify..."`
- **Clear prompts**: Break complex tasks into numbered steps

## Troubleshooting

| Issue             | Solution                                        |
| ----------------- | ----------------------------------------------- |
| Agent timeout     | Increase `timeout` or reduce task complexity    |
| Task fails        | Simplify prompt, add more specific instructions |
| Connection errors | Verify ANTHROPIC_API_KEY is set                 |

## See Also

- [Playwright Plugin](playwright.md) - Deterministic browser automation
- [Browser Use Plugin](browser-use.md) - Alternative AI browser automation
- [Variables](../features/variables.md) - Passing data to agent prompts
