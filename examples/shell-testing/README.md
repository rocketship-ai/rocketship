# Shell Script Plugin Example

This example demonstrates the shell script execution capabilities in Rocketship using the script plugin with `language: shell`.

## Features Demonstrated

### 1. Basic Shell Commands
- Simple echo commands
- Variable substitution from config (`{{ .vars.variable_name }}`)
- Environment information gathering

### 2. File Operations
- Creating temporary files
- Reading file contents
- File cleanup

### 3. Variable Integration
- **Template Variables**: Use `{{ variable_name }}` for state variables and `{{ .vars.variable_name }}` for config variables
- **Environment Variables**: Access state as `$ROCKETSHIP_VARIABLE_NAME` and config as `$ROCKETSHIP_VAR_VARIABLE_NAME`

### 4. Build Simulation
- Multi-step compilation process
- Build verification
- Status reporting

### 5. Complex Pipelines
- Multi-stage operations
- Directory management
- File generation and processing

## Automatic Result Saving

The shell executor automatically saves these results for each step:
- `stdout`: Standard output from the command
- `stderr`: Standard error output  
- `exit_code`: Exit code (0 for success, non-zero for failure)
- `duration`: Execution time

These are available in subsequent steps as environment variables:
- `$ROCKETSHIP_STDOUT`
- `$ROCKETSHIP_STDERR` 
- `$ROCKETSHIP_EXIT_CODE`
- `$ROCKETSHIP_DURATION`

## Usage

Run the example:
```bash
rocketship run -af examples/shell-testing/rocketship.yaml
```

## Configuration

```yaml
- name: "My shell step"
  plugin: script
  config:
    language: shell
    script: |
      echo "Hello from shell!"
      echo "Project: {{ .vars.project_name }}"
      
    # Alternative: external file
    # file: "./scripts/my-script.sh"
    
    # Optional timeout
    timeout: "30s"
```

## Error Handling

- Shell commands that exit with non-zero status will cause the step to fail
- stderr output is captured and included in error messages
- Use proper error handling in your shell scripts:

```bash
# Good: Check command success
if ! command_that_might_fail; then
    echo "Command failed!" >&2
    exit 1
fi

# Good: Use set -e for strict error handling
set -e
command1
command2  # Will fail the step if this fails
```

## Security Notes

- Shell commands execute in the current working directory where `rocketship` is run
- No sandboxing is applied - the shell has full access to the system
- Use appropriate caution with user input and file operations
- Suitable for trusted automation environments

## Use Cases

Perfect for:
- **Build Automation**: Running make, npm, cargo, etc.
- **Environment Setup**: Installing dependencies, configuring services
- **File Processing**: Data transformation, file generation
- **System Integration**: Calling system tools and utilities
- **CI/CD Workflows**: Test execution, deployment scripts
- **Agent Tooling**: Providing shell access for coding agents