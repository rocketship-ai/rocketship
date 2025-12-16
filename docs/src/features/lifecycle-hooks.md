# Lifecycle Hooks

Lifecycle hooks let you **automatically run setup and cleanup tasks** before and after your tests. This is perfect for:
- Creating test data before tests run
- Cleaning up test data after tests complete
- Setting up authentication or connections
- Collecting logs or debugging info when tests fail

**Two levels of hooks:**
- **Suite-level**: Run once for all tests in a file
- **Test-level**: Run for each individual test

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

**Execution order:**
1. `init` steps run first (setup for all tests)
2. All `tests` run
3. If any test failed, `cleanup.on_failure` steps run
4. `cleanup.always` steps always run (cleanup guaranteed)

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

**Execution order:**
1. Test `init` steps run first (setup for this test)
2. Test `steps` run
3. If the test failed, `cleanup.on_failure` steps run
4. `cleanup.always` steps always run (cleanup guaranteed)

## Where Variables Are Available

When you save a variable, where can you use it?

| Where Variable is Saved        | Can Use It In                                          |
| ------------------------------ | ------------------------------------------------------ |
| Suite `init`                   | All tests and suite cleanup                            |
| Test `init` or `steps`         | Remaining steps in that test and the test's cleanup    |
| `cleanup.always` or `on_failure` | Later cleanup steps in the same cleanup block         |

## Best Practices

- **Always cleanup**: Use `cleanup.always` to prevent resource leaks
- **Unique resources**: Use `{{ .run.id }}` for unique resource names
- **Log on failure**: Use `cleanup.on_failure` to collect debugging info
- **Test isolation**: Each test should be independent

## See Also

- [Variables](variables.md) - Using saved variables from hooks
- [Retry Policies](retry-policies.md) - Adding retries to hook steps
