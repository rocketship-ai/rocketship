# Shell Scripting

Execute shell commands and scripts within Rocketship workflows for build processes, system operations, and command-line tool integration.

## Key Features

- **Cross-platform Support** - Automatically detects and uses `bash` or `sh`
- **Variable Substitution** - Access config variables and step state
- **Environment Integration** - Automatic environment variable injection
- **External Script Files** - Store scripts in separate `.sh` files
- **Output Capture** - Automatically captures stdout, stderr, exit codes, and duration

## Basic Usage

### Inline Scripts

```yaml
- name: "Build and test"
  plugin: script
  config:
    language: shell
    script: |
      echo "Building {{ .vars.project_name }} version {{ .vars.version }}"
      npm install
      npm test
      npm run build
```

### External Script Files

```yaml
- name: "Deploy application"
  plugin: script
  config:
    language: shell
    file: "scripts/deploy.sh"
    timeout: "60s"
```

## Variable Access

### Template Variables

```yaml
vars:
  project_name: "my-app"
  environment: "production"

steps:
  - name: "Deploy"
    plugin: script
    config:
      language: shell
      script: |
        # Config variables
        echo "Deploying {{ .vars.project_name }} to {{ .vars.environment }}"
        
        # State variables from previous steps
        echo "Build ID: {{ build_id }}"
        echo "Commit: {{ commit_hash }}"
```

### Environment Variables

Shell scripts automatically receive environment variables:

```bash
# Config variables as ROCKETSHIP_VAR_*
echo "Project: $ROCKETSHIP_VAR_PROJECT_NAME"
echo "Environment: $ROCKETSHIP_VAR_ENVIRONMENT"

# State variables as ROCKETSHIP_*
echo "Build ID: $ROCKETSHIP_BUILD_ID"

# Previous step results
echo "Last exit code: $ROCKETSHIP_EXIT_CODE"
echo "Last stdout: $ROCKETSHIP_STDOUT"
```

## Integration with HTTP Steps

```yaml
- name: "Get deployment info"
  plugin: http
  config:
    method: GET
    url: "https://api.example.com/deploy/latest"
  save:
    - json_path: ".deploy_id"
      as: "deploy_id"

- name: "Deploy application"
  plugin: script
  config:
    language: shell
    script: |
      echo "Deploying {{ deploy_id }}"
      kubectl apply -f deployment.yaml
      kubectl set image deployment/app app=myapp:{{ .vars.version }}
```

## Error Handling

```yaml
- name: "Robust deployment"
  plugin: script
  config:
    language: shell
    script: |
      set -euo pipefail  # Exit on error, undefined vars, pipe failures
      
      cleanup() {
        echo "Cleaning up..."
        docker stop temp-container 2>/dev/null || true
      }
      
      trap cleanup ERR EXIT
      
      # Your deployment commands here
      docker run -d --name temp-container myapp:{{ .vars.version }}
      
      # Wait for health check
      for i in {1..30}; do
        if curl -f http://localhost:8080/health; then
          echo "✅ Deployment successful"
          exit 0
        fi
        sleep 2
      done
      
      echo "❌ Health check failed"
      exit 1
```

## File Operations

```yaml
- name: "Prepare release"
  plugin: script
  config:
    language: shell
    script: |
      # Create release package
      mkdir -p release
      tar -czf "{{ .vars.project_name }}-{{ .vars.version }}.tar.gz" dist/
      
      # Generate metadata
      cat > release/metadata.json << EOF
      {
        "project": "{{ .vars.project_name }}",
        "version": "{{ .vars.version }}",
        "build_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      }
      EOF
```

## Running Examples

```bash
# Run comprehensive shell testing (includes external files)
rocketship run -af examples/shell-testing/rocketship.yaml

# Debug logging to see shell execution details
ROCKETSHIP_LOG=DEBUG rocketship run -af examples/shell-testing/rocketship.yaml
```

## Best Practices

- **Use `set -euo pipefail`** for safe error handling
- **Quote variables** to handle spaces: `"{{ .vars.project_name }}"`
- **Use environment variables for secrets**: `$ROCKETSHIP_VAR_API_TOKEN`
- **Provide clear output** with status indicators: `✅` `❌` `🔧`
- **Clean up resources** with trap functions on exit/error