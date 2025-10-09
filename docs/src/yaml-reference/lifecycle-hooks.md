# Lifecycle Hooks

Rocketship test suites can run deterministic setup and teardown hooks at both the suite and test levels. Hooks reuse the same step schema you already know (`name`, `plugin`, `config`, `assertions`, `save`, `retry`) and execute linearly in the order they are declared.

## Suite Lifecycle

```yaml
name: "Checkout Suite"
init:
  - name: "Boot ephemeral stack"
    plugin: script
    config: { cmd: "./env/up.sh" }
tests:
  - name: "Happy path checkout"
    steps: [...]
cleanup:
  always:
    - name: "Shutdown stack"
      plugin: script
      config: { cmd: "./env/down.sh" }
  on_failure:
    - name: "Collect logs"
      plugin: http
      config:
        method: GET
        url: "https://ops.example.com/logs?run={{ run_id }}"
```

Execution order:

1. `init` runs once before any tests. If a step fails, the suite is marked failed, tests are skipped, and cleanup runs.
2. Tests run (in parallel or sequentially, matching existing run behaviour).
3. `cleanup.always` runs once after every test completes. If any test or the suite init failed, `cleanup.on_failure` runs first.

Saved values from suite init steps are injected as runtime variables on every test. Reference them directly:

- `{{ api_token }}` – anywhere inside tests or cleanup steps.
- `{{ api_token }}` inside suite cleanup.

These values are read-only snapshots; cleanup can rely on them but should not expect later steps to consume data emitted in cleanup.

### Suite Cleanup Timeout

**Important**: Suite cleanup workflows have a **45-minute timeout**. If your cleanup takes longer than 45 minutes, it will be canceled and may leave resources in a partially cleaned-up state.

Best practices for long-running cleanup:

- **Break up long teardowns**: If you have a 30-minute environment teardown script, consider breaking it into smaller, independent steps that can fail gracefully
- **Use idempotent operations**: Design cleanup steps to be safely re-runnable in case of timeout
- **Log progress**: Add log steps between long-running operations so you can see how far cleanup progressed
- **Prioritize critical cleanup**: Put the most important cleanup steps first (e.g., delete databases before deleting logs)

Example with delay steps:

```yaml
cleanup:
  always:
    - name: "Delete database"
      plugin: supabase
      config: { action: execute_sql, query: "DROP DATABASE test_db" }
    - name: "Wait for DB deletion"
      plugin: delay
      config: { duration: "5m" }  # This counts toward the 45-minute limit
    - name: "Delete storage buckets"
      plugin: script
      config: { cmd: "./cleanup-storage.sh" }
```

**What happens on timeout**: The cleanup workflow is canceled, Rocketship logs a warning, and the server continues shutting down. The suite outcome (passed/failed) is not changed by cleanup timeouts.

## Test Lifecycle

```yaml
tests:
  - name: "Creates an order"
    init:
      - name: "Seed user"
        plugin: sql
        config: { file: "./sql/seed_user.sql" }
    steps:
      - name: "POST /orders"
        plugin: http
        config: { url: "{{ api_url }}/orders" }
        save:
          - json_path: ".id"
            as: "order_id"
    cleanup:
      always:
        - name: "Delete order"
          plugin: http
          config:
            method: DELETE
            url: "{{ api_url }}/orders/{{ order_id }}"
      on_failure:
        - name: "Snapshot order payload"
          plugin: http
          config:
            method: GET
            url: "{{ api_url }}/orders/{{ order_id }}"
```

For each test:

1. `init` runs first. Failure marks the test failed, skips the main `steps`, then moves straight into cleanup.
2. `steps` run with the same linear semantics as before.
3. `cleanup.always` runs every time. If the test failed (including its init phase), `cleanup.on_failure` runs beforehand.

Test-level hook saves behave exactly like other runtime values: reference them as `{{ name }}` within the same test (init, steps, cleanup).

## Referencing Saved Values

| Source | How to reference inside tests | Where available |
|--------|-------------------------------|-----------------|
| Suite `init` saved values | `{{ <name> }}` | All test init/steps/cleanup, suite cleanup |
| Test `init` or `steps` saved values | `{{ <name> }}` | Remaining steps and cleanups for that test |
| Cleanup saved values | `{{ <name> }}` | Later cleanup steps in the same cleanup block |

Additional notes:

- Saved values are injected as runtime variables, so you can use them in any templated string (URLs, headers, bodies, script vars, etc.).
- Test-level values never leak across tests. Each test gets its own state map.
- Suite cleanup runs with a disconnected Temporal context. Failures are logged but do not overwrite the original test/suite outcome.

Use lifecycle hooks to create deterministic, self-cleaning suites that still respect Rocketship’s linear execution model.
