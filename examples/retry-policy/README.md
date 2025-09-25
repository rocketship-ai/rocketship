# Retry Policy Test Suite

This example demonstrates the plugin-agnostic retry functionality in Rocketship. The retry feature allows you to configure Temporal activity retry policies for any step, regardless of the plugin type.

## Features Demonstrated

### 1. Plugin-Agnostic Retry Configuration
- Works with any plugin (`http`, `delay`, `log`, etc.)
- Configurable per-step retry policies
- Backward compatible (steps without retry use default single-attempt policy)

### 2. Retry Policy Options

The retry configuration supports all Temporal RetryPolicy options:

```yaml
retry:
  initial_interval: "1s"          # Initial retry interval
  maximum_interval: "10s"         # Maximum retry interval  
  maximum_attempts: 3             # Maximum number of attempts
  backoff_coefficient: 2.0        # Exponential backoff coefficient
  non_retryable_errors: []        # Error types that should not be retried
```

### 3. Test Scenarios

1. **HTTP Request with Retry Policy**: Tests HTTP requests with retry configuration (succeeds without needing retries)
2. **Delay Plugin with Retry Policy**: Shows retry works with non-HTTP plugins
3. **Multiple Plugin Types**: Different retry policies for different plugins
4. **Steps Without Retry Policy**: Demonstrates backward compatibility

## Running the Tests

```bash
# With the Minikube stack running
kubectl port-forward -n rocketship svc/rocketship-engine 7700:7700
rocketship run -af examples/retry-policy/rocketship.yaml

# Local auto mode
rocketship run -af examples/retry-policy/rocketship.yaml
```

## Implementation Details

### DSL Changes

The `Step` struct now includes an optional `Retry` field:

```go
type Step struct {
    Name       string                   `json:"name" yaml:"name"`
    Plugin     string                   `json:"plugin" yaml:"plugin"`
    Config     map[string]interface{}   `json:"config" yaml:"config"`
    Assertions []map[string]interface{} `json:"assertions" yaml:"assertions"`
    Save       []map[string]interface{} `json:"save" yaml:"save,omitempty"`
    Retry      *RetryPolicy             `json:"retry" yaml:"retry,omitempty"`
}

type RetryPolicy struct {
    InitialInterval    string   `json:"initial_interval" yaml:"initial_interval,omitempty"`
    MaximumInterval    string   `json:"maximum_interval" yaml:"maximum_interval,omitempty"`
    MaximumAttempts    int      `json:"maximum_attempts" yaml:"maximum_attempts,omitempty"`
    BackoffCoefficient float64  `json:"backoff_coefficient" yaml:"backoff_coefficient,omitempty"`
    NonRetryableErrors []string `json:"non_retryable_errors" yaml:"non_retryable_errors,omitempty"`
}
```

### Workflow Integration

Each step now creates its own activity options with the appropriate retry policy:

```go
// Create step-specific activity options with retry policy
ao := workflow.ActivityOptions{
    StartToCloseTimeout: time.Minute * 30,
    RetryPolicy:         buildRetryPolicy(step.Retry),
}
stepCtx := workflow.WithActivityOptions(ctx, ao)

// Execute the plugin activity with step-specific options
err := workflow.ExecuteActivity(stepCtx, step.Plugin, pluginParams).Get(stepCtx, &activityResp)
```

## Expected Behavior

When you run this test suite:

1. Steps with retry policies will attempt retries **only when the step fails** (assertion failures, network errors, etc.)
2. Steps without retry policies will use the default (single attempt)
3. All plugin types work with retry policies
4. Failed requests will be retried with exponential backoff
5. Debug logging will show retry attempts in action

!!! note "When Retries Occur"
    Retries only happen when a step **fails** - either due to assertion failures, network errors, or plugin execution errors. If all assertions pass, the step is successful and no retries occur. This test demonstrates retry **configuration** working correctly, not actual retry execution.

## Debug Logging

To see retry behavior in action, run with debug logging:

```bash
ROCKETSHIP_LOG=DEBUG rocketship run -af examples/retry-policy/rocketship.yaml
```

This will show:
- Temporal activity execution details
- Retry attempt logs
- Plugin execution flow
- Workflow state changes