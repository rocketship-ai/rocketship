# Lifecycle Hooks

Run setup and teardown steps at suite and test levels.

## Suite Lifecycle

```yaml
name: "API Test Suite"
init:
  - name: "Get auth token"
    plugin: http
    config:
      method: POST
      url: "{{ .env.API_URL }}/auth/token"
    save:
      - json_path: ".token"
        as: "api_token"

tests:
  - name: "Test with token"
    steps:
      - plugin: http
        config:
          url: "{{ .env.API_URL }}/users"
          headers:
            Authorization: "Bearer {{ api_token }}"

cleanup:
  always:
    - name: "Cleanup resources"
      plugin: http
      config:
        method: DELETE
        url: "{{ .env.API_URL }}/test-data"

  on_failure:
    - name: "Collect logs"
      plugin: http
      config:
        method: GET
        url: "{{ .env.OPS_URL }}/logs?run={{ .run.id }}"
```

**Execution**: `init` → `tests` → `cleanup.on_failure` (if failed) → `cleanup.always`

## Test Lifecycle

```yaml
tests:
  - name: "User CRUD test"
    init:
      - name: "Create test user"
        plugin: http
        config:
          method: POST
          url: "{{ .env.API_URL }}/users"
        save:
          - json_path: ".id"
            as: "user_id"

    steps:
      - name: "Update user"
        plugin: http
        config:
          method: PATCH
          url: "{{ .env.API_URL }}/users/{{ user_id }}"

    cleanup:
      always:
        - name: "Delete user"
          plugin: http
          config:
            method: DELETE
            url: "{{ .env.API_URL }}/users/{{ user_id }}"
```

**Execution**: `init` → `steps` → `cleanup.on_failure` (if failed) → `cleanup.always`

## Variable Scoping

| Source            | Available In                               |
| ----------------- | ------------------------------------------ |
| Suite `init`      | All tests and suite cleanup                |
| Test `init/steps` | Remaining steps and cleanup in that test   |
| Cleanup saves     | Later cleanup steps in same cleanup block  |

## Best Practices

- **Always cleanup**: Use `cleanup.always` to prevent resource leaks
- **Unique resources**: Use `{{ .run.id }}` for unique resource names
- **Log on failure**: Use `cleanup.on_failure` to collect debugging info
- **Test isolation**: Each test should be independent

## See Also

- [Variables](variables.md) - Using saved variables from hooks
- [Retry Policies](retry-policies.md) - Adding retries to hook steps
