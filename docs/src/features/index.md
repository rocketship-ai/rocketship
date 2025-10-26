# Features

Rocketship provides powerful features for building robust, maintainable test suites.

## Core Features

### [Variables](variables.md)

Parameterize tests with four types of variables:

- **Built-in Variables** - `{{ .run.id }}` for unique identifiers per test run
- **Environment Variables** - `{{ .env.API_KEY }}` for secrets and environment-specific config
- **Config Variables** - `{{ .vars.timeout }}` for test parameters
- **Runtime Variables** - `{{ user_id }}` for values saved during execution

### [Lifecycle Hooks](lifecycle-hooks.md)

Setup and teardown at suite and test levels:

- **Suite-level** - Run init/cleanup once for entire test suite
- **Test-level** - Run init/cleanup for each individual test
- **Conditional cleanup** - `always` and `on_failure` hooks
- **Variable sharing** - Suite init values available to all tests

### [Retry Policies](retry-policies.md)

Automatic retries with configurable backoff:

- **Exponential backoff** - Increasing delays between retries
- **Linear backoff** - Constant delays between retries
- **Max attempts** - Limit retry attempts
- **Error filtering** - Skip retries for specific error types
- **All plugins** - Works with HTTP, SQL, browser, and all other plugins

## Feature Highlights

### Test Isolation

```yaml
tests:
  - name: "Test 1"
    init:
      - name: "Setup unique data"
        plugin: script
        config:
          script: |
            save("test_id", "{{ .run.id }}");
```

### Self-Cleaning Tests

```yaml
cleanup:
  always:
    - name: "Delete test data"
      plugin: http
      config:
        method: DELETE
        url: "{{ .vars.api_url }}/test-data/{{ test_id }}"
```

### Reliable Tests

```yaml
- name: "Call flaky API"
  plugin: http
  config:
    url: "{{ .vars.api_url }}/status"
  retry:
    maximum_attempts: 5
    initial_interval: "1s"
    backoff_coefficient: 2.0
```

## See Also

- [Plugins](../plugins/index.md) - Available testing plugins
- [Command Reference](../reference/rocketship.md) - CLI commands
