# Write your own plugin

Rocketship uses a plugin system to extend its capabilities. Plugins are activities that can be executed as part of Temporal workflows. This document explains how to create your own custom plugins.

## Plugin Interface

All plugins must implement the `Plugin` interface defined in `plugin.go`:

```go
type Plugin interface {
    GetType() string
    Activity(ctx context.Context, p map[string]interface{}) (interface{}, error)
}
```

### Methods

- **GetType()**: Returns a unique string identifier for your plugin type
- **Activity()**: The main execution logic for your plugin, receives context and parameters

## Creating a Plugin

### 1. Create Plugin Directory

Create a new directory under `internal/plugins/` for your plugin:

```
internal/plugins/
├── yourplugin/
│   ├── yourplugin.go
│   └── types.go
```

### 2. Define Plugin Types

Create a `types.go` file to define your plugin's configuration structure:

```go
package yourplugin

type YourPlugin struct {
    Name   string           `json:"name" yaml:"name"`
    Plugin string           `json:"plugin" yaml:"plugin"`
    Config YourPluginConfig `json:"config" yaml:"config"`
}

type YourPluginConfig struct {
    // Define your plugin's configuration fields here
    SomeField string `json:"some_field" yaml:"some_field"`
    AnotherField int `json:"another_field" yaml:"another_field"`
}
```

### 3. Implement Plugin Logic

Create your main plugin file (e.g., `yourplugin.go`):

```go
package yourplugin

import (
    "context"
    "fmt"

    "go.temporal.io/sdk/activity"
)

func (yp *YourPlugin) GetType() string {
    return "yourplugin"
}

func (yp *YourPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
    logger := activity.GetLogger(ctx)

    // Parse configuration from parameters
    configData, ok := p["config"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid config format")
    }

    // Extract your configuration fields
    someField, ok := configData["some_field"].(string)
    if !ok {
        return nil, fmt.Errorf("some_field is required")
    }

    // Implement your plugin logic here
    logger.Info("Executing your plugin", "some_field", someField)

    // Return your result
    return map[string]interface{}{
        "result": "success",
        "data": someField,
    }, nil
}
```

### 4. Register Plugin

Add your plugin to the worker in `cmd/worker/main.go`:

```go
import (
    // ... other imports
    "github.com/rocketship-ai/rocketship/internal/plugins/yourplugin"
)

func main() {
    // ... existing code ...

    // Register your plugin
    plugins.RegisterWithTemporal(w, &yourplugin.YourPlugin{})

    // ... rest of the code ...
}
```

## Plugin Examples

### Simple Plugin (Delay)

The delay plugin is a minimal example:

```go
func (dp *DelayPlugin) GetType() string {
    return "delay"
}

func (dp *DelayPlugin) Activity(ctx context.Context, p map[string]interface{}) (interface{}, error) {
    // Dummy activity - actual delay is handled by workflow sleep
    return nil, nil
}
```

## Best Practices

### Error Handling

- Always validate input parameters
- Return descriptive error messages
- Use structured logging with context

### Configuration

- Define clear configuration structures in `types.go`
- Validate required fields
- Provide sensible defaults where appropriate

### State Management

- Access workflow state through the `state` parameter
- Save data back to state using the return value
- Handle variable replacement for dynamic values

### Logging

- Use the Temporal activity logger: `activity.GetLogger(ctx)`
- Log important operations and errors
- Include relevant context in log messages

### Testing

- Create comprehensive unit tests
- Test error conditions and edge cases
- Mock external dependencies

## Variable Replacement

Plugins can use variable replacement to access workflow state. Variables are referenced using the `{{ variable_name }}` syntax:

```yaml
config:
  url: "https://api.example.com/users/{{ user_id }}"
  headers:
    Authorization: "Bearer {{ auth_token }}"
```

The HTTP plugin provides a reference implementation of variable replacement.

## Plugin Parameters

Plugins receive parameters through the `p map[string]interface{}` parameter:

- `config`: Plugin-specific configuration
- `state`: Current workflow state (variables from previous steps)
- `assertions`: Test assertions (if applicable)
- `save`: Data extraction configuration (if applicable)

## Return Values

Plugins should return structured data that can be:

- Used by subsequent workflow steps
- Saved to workflow state
- Used for assertions and validations

Example return structure:

```go
return map[string]interface{}{
    "status": "success",
    "data": responseData,
    "saved": savedVariables,
}, nil
```

## Integration

Once your plugin is implemented and registered:

1. It will be available as an activity in Temporal workflows
2. The workflow interpreter can execute it as part of test scenarios
3. It can be configured through YAML test definitions

For an example, see the http plugin in `internal/plugins/http/`.
