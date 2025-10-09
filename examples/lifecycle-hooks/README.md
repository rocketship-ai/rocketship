# Lifecycle Hooks Examples

This directory contains comprehensive examples demonstrating Rocketship's lifecycle hooks feature with Supabase integration.

## What are Lifecycle Hooks?

Lifecycle hooks allow you to run setup and teardown steps at both the **suite** and **test** levels, ensuring deterministic execution and proper resource cleanup.

### Suite-Level Hooks

- **`init`**: Runs once before all tests. Saved values are available in all tests as `{{ variable_name }}`
- **`cleanup.always`**: Runs once after all tests complete
- **`cleanup.on_failure`**: Runs before `cleanup.always` if any test or suite init failed

### Test-Level Hooks

- **`init`**: Runs before each test's main steps. Saved values are available in that test as `{{ variable_name }}`
- **`cleanup.always`**: Runs after each test completes
- **`cleanup.on_failure`**: Runs before `cleanup.always` if the test or its init failed

## Examples

### 1. Suite-Level Hooks (`suite-level-hooks.yaml`)

Demonstrates:
- Creating shared resources in suite `init` (database records, auth users)
- Using suite-saved values across multiple tests via `{{ shared_company_id }}`, `{{ shared_auth_token }}`
- Cleaning up shared resources in suite `cleanup.always`
- Debugging with suite `cleanup.on_failure`

**Run it:**
```bash
rocketship run -af examples/lifecycle-hooks/suite-level-hooks.yaml --env-file examples/lifecycle-hooks/.env
```

### 2. Test-Level Hooks (`test-level-hooks.yaml`)

Demonstrates:
- Creating test-specific resources in test `init`
- Using test-saved values in test steps via `{{ test_company_id }}`, `{{ test_user_id }}`
- Cleaning up test resources in test `cleanup.always`
- Capturing failure state with test `cleanup.on_failure`
- Test isolation (each test has independent resources)

**Run it:**
```bash
rocketship run -af examples/lifecycle-hooks/test-level-hooks.yaml --env-file examples/lifecycle-hooks/.env
```

### 3. Combined Hooks (`combined-hooks.yaml`)

Demonstrates:
- Both suite and test-level hooks working together
- Suite `init` creates shared infrastructure (storage bucket, admin user)
- Test `init` creates test-specific resources
- Test steps use both suite and test variables
- Test `cleanup` removes test resources
- Suite `cleanup` removes shared infrastructure

**Run it:**
```bash
rocketship run -af examples/lifecycle-hooks/combined-hooks.yaml --env-file examples/lifecycle-hooks/.env
```

## Variable Scoping

| Source | How to reference | Where available |
|--------|-----------------|-----------------|
| Suite `init` saved values | `{{ <name> }}` | All test init/steps/cleanup, suite cleanup |
| Test `init` or `steps` saved values | `{{ <name> }}` | Remaining steps and cleanups for that test |
| Cleanup saved values | `{{ <name> }}` | Later cleanup steps in the same cleanup block |

## Key Concepts

1. **Deterministic Execution**: All hooks run linearly in the order declared
2. **Failure Handling**: If init fails, main steps are skipped but cleanup still runs
3. **Cleanup Guarantees**: Cleanup always runs (with disconnected Temporal context)
4. **Value Propagation**: Suite init values are injected into every test's runtime state
5. **Test Isolation**: Test-level values never leak across tests

## Prerequisites

- Supabase project with the `companies` table (see `examples/supabase-testing/`)
- Environment variables set in `.env`:
  - `SUPABASE_ANON_KEY`
  - `SUPABASE_SERVICE_KEY`

## CI Integration

These examples are automatically tested in CI to ensure lifecycle hooks work correctly with Supabase operations.
