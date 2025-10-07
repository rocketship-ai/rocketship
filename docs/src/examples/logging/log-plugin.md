# Log Plugin

The log plugin allows you to add custom logging messages to your test suites for debugging, monitoring, and progress tracking. Log messages appear in the CLI output during test execution.

## Configuration

```yaml
plugin: log
config:
  message: "Your log message here"
```

## Basic Usage

```yaml
name: Basic Logging Example
tests:
  - name: Test with logging
    steps:
      - plugin: log
        config:
          message: "Starting test execution"

      - plugin: http
        config:
          url: "https://tryme.rocketship.sh/echo"
          method: "GET"

      - plugin: log
        config:
          message: "HTTP request completed"
```

## Variable Support

The log plugin supports all variable types including configuration variables, environment variables, and runtime variables:

```yaml
name: Logging with Variables
config:
  session: "test-session-123"
tests:
  - name: Variable logging example
    steps:
      - plugin: log
        config:
          message: "Starting test for session: {{ .vars.session }}"

      - plugin: http
        config:
          url: "https://tryme.rocketship.sh/echo"
          method: "GET"
        save:
          - key: "response_data"
            value: "{{ .response.json }}"
      
      - plugin: log
        config:
          message: "User agent: {{ .runtime.response_data.headers.User-Agent }}"
      
      - plugin: log
        config:
          message: "Test running on: {{ .env.HOSTNAME }}"
```

## Example Output

When running tests:

```bash
rocketship run -af examples/simple-log/rocketship.yaml
```

You'll see log messages in the output:
```
🚀 Starting user-service tests in staging environment
Running on user's machine at /home/user
Created test data with ID: test_1234567890, Status: active
⚠️  Warning: This is a simulated warning during testing
✅ Test completed successfully for user-service
```

## Use Cases

- **Progress Tracking**: Log milestones in long-running tests
- **Debug Information**: Output variable values and intermediate results
- **Test Documentation**: Add context about what each step is doing
- **Monitoring**: Track important events during test execution

Log messages always appear in the CLI output regardless of the logging level, making them perfect for providing real-time feedback during test execution.