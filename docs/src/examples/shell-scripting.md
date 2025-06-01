# Shell Scripting

The shell script plugin enables execution of shell commands and scripts within Rocketship workflows. It provides a powerful way to integrate build processes, system operations, and command-line tools into your testing pipelines.

## Key Features

- **Cross-platform Shell Support** - Automatically detects and uses `bash` or `sh`
- **Variable Substitution** - Access config variables and step state in shell scripts
- **Environment Integration** - Automatic environment variable injection
- **Working Directory** - Executes in the directory where Rocketship is run
- **Output Capture** - Automatically captures stdout, stderr, exit codes, and execution duration
- **No Security Restrictions** - Full access to host system capabilities

## Basic Configuration

### Inline Shell Scripts

```yaml
- name: "Simple shell command"
  plugin: script
  config:
    language: shell
    script: |
      echo "Hello from shell!"
      echo "Current directory: $(pwd)"
      echo "User: $(whoami)"
```

### Multi-line Shell Scripts

```yaml
- name: "Build and test project"
  plugin: script
  config:
    language: shell
    script: |
      echo "Starting build process..."
      
      # Clean previous build
      rm -rf build/
      mkdir -p build/
      
      # Compile project
      make clean
      make build
      
      # Run tests
      make test
      
      echo "Build completed successfully!"
```

## Variable Substitution

Shell scripts support two types of variable substitution:

### Config Variables (`{{ .vars.variable_name }}`)

Access variables from the `vars` section:

```yaml
vars:
  project_name: "rocketship"
  build_target: "production"
  version: "1.0.0"

steps:
  - name: "Deploy with variables"
    plugin: script
    config:
      language: shell
      script: |
        echo "Deploying {{ .vars.project_name }} version {{ .vars.version }}"
        echo "Target environment: {{ .vars.build_target }}"
        
        # Create release package
        tar -czf "{{ .vars.project_name }}-{{ .vars.version }}.tar.gz" build/
        
        # Deploy to target
        ./deploy.sh --target={{ .vars.build_target }} --version={{ .vars.version }}
```

### State Variables (`{{ variable_name }}`)

Access data saved from previous steps:

```yaml
- name: "Get build info"
  plugin: http
  config:
    method: GET
    url: "https://api.example.com/builds/latest"
  save:
    - json_path: ".build_id"
      as: "build_id"
    - json_path: ".commit_hash"
      as: "commit_hash"

- name: "Deploy specific build"
  plugin: script
  config:
    language: shell
    script: |
      echo "Deploying build {{ build_id }}"
      echo "Commit: {{ commit_hash }}"
      
      # Download build artifact
      wget "https://artifacts.example.com/{{ build_id }}/app.tar.gz"
      
      # Extract and deploy
      tar -xzf app.tar.gz
      ./deploy.sh --build={{ build_id }} --commit={{ commit_hash }}
```

## Environment Variables

Shell scripts automatically receive environment variables with `ROCKETSHIP_` prefixes:

### State Variables as Environment

State from previous steps is available as `ROCKETSHIP_*` environment variables:

```yaml
- name: "Save database info"
  plugin: http
  config:
    method: GET
    url: "https://api.example.com/database/status"
  save:
    - json_path: ".host"
      as: "db_host"
    - json_path: ".port"
      as: "db_port"

- name: "Connect to database"
  plugin: script
  config:
    language: shell
    script: |
      echo "Database host: $ROCKETSHIP_DB_HOST"
      echo "Database port: $ROCKETSHIP_DB_PORT"
      
      # Connect using environment variables
      psql -h "$ROCKETSHIP_DB_HOST" -p "$ROCKETSHIP_DB_PORT" -c "SELECT version();"
```

### Config Variables as Environment

Config variables are available as `ROCKETSHIP_VAR_*` environment variables:

```yaml
vars:
  api_token: "secret-token-123"
  environment: "staging"

steps:
  - name: "Deploy with auth"
    plugin: script
    config:
      language: shell
      script: |
        echo "Deploying to: $ROCKETSHIP_VAR_ENVIRONMENT"
        
        # Use token for authentication
        curl -H "Authorization: Bearer $ROCKETSHIP_VAR_API_TOKEN" \
             -X POST \
             "https://api.example.com/deploy"
```

## Output Capture and Validation

Shell execution results are automatically captured and can be accessed in subsequent steps:

```yaml
- name: "Run build command"
  plugin: script
  config:
    language: shell
    script: |
      echo "Starting build..."
      make build 2>&1
      exit_code=$?
      echo "Build finished with exit code: $exit_code"
      exit $exit_code

- name: "Check build results"
  plugin: script
  config:
    language: shell
    script: |
      echo "Previous step exit code: $ROCKETSHIP_EXIT_CODE"
      echo "Previous step stdout contained:"
      echo "$ROCKETSHIP_STDOUT" | head -5
      
      # Validate build succeeded
      if [ "$ROCKETSHIP_EXIT_CODE" = "0" ]; then
        echo "✅ Build successful"
      else
        echo "❌ Build failed"
        exit 1
      fi
```

Available captured variables:
- `ROCKETSHIP_EXIT_CODE` - Exit code from previous shell step
- `ROCKETSHIP_STDOUT` - Standard output from previous shell step  
- `ROCKETSHIP_STDERR` - Standard error from previous shell step
- `ROCKETSHIP_DURATION` - Execution duration from previous shell step

## Complete Integration Example

Here's a comprehensive example showing shell integration with HTTP steps:

```yaml
name: "CI/CD Pipeline with Shell Integration"
description: "Complete build, test, and deployment pipeline"

vars:
  project_name: "my-api"
  environment: "staging"
  version: "1.2.0"
  docker_registry: "registry.example.com"

tests:
  - name: "Complete CI/CD Pipeline"
    steps:
      # 1. Prepare build environment
      - name: "Setup build environment"
        plugin: script
        config:
          language: shell
          script: |
            echo "Setting up build environment for {{ .vars.project_name }}"
            
            # Clean workspace
            rm -rf build/ dist/ node_modules/
            
            # Install dependencies
            npm install
            
            # Verify environment
            node --version
            npm --version
            echo "✅ Environment ready"

      # 2. Run tests
      - name: "Run test suite"
        plugin: script
        config:
          language: shell
          script: |
            echo "Running tests for {{ .vars.project_name }}"
            
            # Run linting
            npm run lint
            
            # Run unit tests
            npm test
            
            # Generate coverage report
            npm run test:coverage
            
            echo "✅ All tests passed"

      # 3. Build application
      - name: "Build application"
        plugin: script
        config:
          language: shell
          script: |
            echo "Building {{ .vars.project_name }} version {{ .vars.version }}"
            
            # Build for production
            NODE_ENV=production npm run build
            
            # Create distribution package
            tar -czf "{{ .vars.project_name }}-{{ .vars.version }}.tar.gz" dist/
            
            # Calculate checksum
            sha256sum "{{ .vars.project_name }}-{{ .vars.version }}.tar.gz" > checksum.txt
            
            echo "✅ Build completed"

      # 4. Build Docker image
      - name: "Build Docker image"
        plugin: script
        config:
          language: shell
          script: |
            echo "Building Docker image..."
            
            # Build image
            docker build -t {{ .vars.docker_registry }}/{{ .vars.project_name }}:{{ .vars.version }} .
            docker build -t {{ .vars.docker_registry }}/{{ .vars.project_name }}:latest .
            
            # Test image
            docker run --rm {{ .vars.docker_registry }}/{{ .vars.project_name }}:{{ .vars.version }} --version
            
            echo "✅ Docker image built and tested"

      # 5. Upload artifacts (HTTP step using shell results)
      - name: "Upload build artifacts"
        plugin: http
        config:
          method: POST
          url: "https://artifacts.example.com/{{ .vars.project_name }}/{{ .vars.version }}"
          headers:
            Authorization: "Bearer {{ .vars.upload_token }}"
            X-Environment: "{{ .vars.environment }}"
          files:
            - field: "artifact"
              path: "{{ .vars.project_name }}-{{ .vars.version }}.tar.gz"
            - field: "checksum"
              path: "checksum.txt"
        save:
          - json_path: ".artifact_id"
            as: "artifact_id"
          - json_path: ".download_url"
            as: "download_url"

      # 6. Deploy to staging
      - name: "Deploy to staging"
        plugin: script
        config:
          language: shell
          script: |
            echo "Deploying {{ .vars.project_name }} to {{ .vars.environment }}"
            echo "Artifact ID: {{ artifact_id }}"
            echo "Download URL: {{ download_url }}"
            
            # Pull latest image
            docker pull {{ .vars.docker_registry }}/{{ .vars.project_name }}:{{ .vars.version }}
            
            # Update deployment
            kubectl set image deployment/{{ .vars.project_name }} \
              app={{ .vars.docker_registry }}/{{ .vars.project_name }}:{{ .vars.version }} \
              --namespace={{ .vars.environment }}
            
            # Wait for rollout
            kubectl rollout status deployment/{{ .vars.project_name }} \
              --namespace={{ .vars.environment }} \
              --timeout=300s
            
            echo "✅ Deployment completed"

      # 7. Health check (HTTP step)
      - name: "Verify deployment health"
        plugin: http
        config:
          method: GET
          url: "https://{{ .vars.project_name }}-{{ .vars.environment }}.example.com/health"
          timeout: 30s
        assertions:
          - json_path: ".status"
            expected: "healthy"
          - json_path: ".version"
            expected: "{{ .vars.version }}"

      # 8. Run smoke tests
      - name: "Run smoke tests"
        plugin: script
        config:
          language: shell
          script: |
            echo "Running smoke tests against {{ .vars.environment }}"
            
            # Run API tests
            newman run smoke-tests.json \
              --env-var "base_url=https://{{ .vars.project_name }}-{{ .vars.environment }}.example.com" \
              --env-var "version={{ .vars.version }}"
            
            # Check critical endpoints
            curl -f "https://{{ .vars.project_name }}-{{ .vars.environment }}.example.com/api/users" > /dev/null
            curl -f "https://{{ .vars.project_name }}-{{ .vars.environment }}.example.com/api/status" > /dev/null
            
            echo "✅ Smoke tests passed"

      # 9. Notify deployment success
      - name: "Send deployment notification"
        plugin: http
        config:
          method: POST
          url: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
          body: |
            {
              "text": "🚀 Deployment Successful",
              "attachments": [
                {
                  "color": "good",
                  "fields": [
                    {"title": "Project", "value": "{{ .vars.project_name }}", "short": true},
                    {"title": "Version", "value": "{{ .vars.version }}", "short": true},
                    {"title": "Environment", "value": "{{ .vars.environment }}", "short": true},
                    {"title": "Artifact", "value": "{{ artifact_id }}", "short": true}
                  ]
                }
              ]
            }
```

## File Operations

Shell scripts excel at file and directory operations:

```yaml
- name: "Prepare release artifacts"
  plugin: script
  config:
    language: shell
    script: |
      echo "Preparing release artifacts..."
      
      # Create release directory structure
      mkdir -p release/{binaries,docs,config}
      
      # Copy binaries
      cp build/bin/* release/binaries/
      chmod +x release/binaries/*
      
      # Generate documentation
      markdown-to-pdf README.md release/docs/README.pdf
      cp -r docs/* release/docs/
      
      # Package configuration
      tar -czf release/config/default-config.tar.gz config/
      
      # Create release archive
      cd release
      zip -r "../{{ .vars.project_name }}-{{ .vars.version }}-release.zip" .
      cd ..
      
      # Generate release notes
      echo "## Release {{ .vars.version }}" > RELEASE_NOTES.md
      echo "Generated on: $(date)" >> RELEASE_NOTES.md
      echo "Artifacts:" >> RELEASE_NOTES.md
      find release -type f -exec basename {} \; | sed 's/^/- /' >> RELEASE_NOTES.md
      
      echo "✅ Release artifacts prepared"
```

## System Integration

Integrate with system tools and services:

```yaml
- name: "System health check"
  plugin: script
  config:
    language: shell
    script: |
      echo "Performing system health check..."
      
      # Check disk space
      df -h | grep -E '(Filesystem|/dev/)' 
      
      # Check memory usage
      free -h
      
      # Check running services
      systemctl status docker nginx postgresql
      
      # Check network connectivity
      ping -c 3 google.com
      
      # Check SSL certificates
      openssl s_client -connect api.example.com:443 < /dev/null 2>/dev/null | \
        openssl x509 -noout -dates
      
      echo "✅ System health check completed"
```

## Error Handling

Shell scripts support comprehensive error handling:

```yaml
- name: "Robust deployment script"
  plugin: script
  config:
    language: shell
    script: |
      set -euo pipefail  # Exit on error, undefined variables, pipe failures
      
      echo "Starting deployment with error handling..."
      
      # Function for cleanup on failure
      cleanup() {
        echo "❌ Deployment failed, performing cleanup..."
        docker stop {{ .vars.project_name }}-new 2>/dev/null || true
        docker rm {{ .vars.project_name }}-new 2>/dev/null || true
        echo "Cleanup completed"
      }
      
      # Set trap for cleanup on error
      trap cleanup ERR
      
      # Start new container
      echo "Starting new container..."
      docker run -d --name {{ .vars.project_name }}-new \
        -p 8080:80 \
        {{ .vars.docker_registry }}/{{ .vars.project_name }}:{{ .vars.version }}
      
      # Wait for container to be ready
      echo "Waiting for container to be ready..."
      for i in {1..30}; do
        if curl -f http://localhost:8080/health > /dev/null 2>&1; then
          echo "✅ Container is ready"
          break
        fi
        echo "Waiting... ($i/30)"
        sleep 2
      done
      
      # Verify health
      if ! curl -f http://localhost:8080/health > /dev/null 2>&1; then
        echo "❌ Health check failed"
        exit 1
      fi
      
      # Switch traffic
      echo "Switching traffic..."
      docker stop {{ .vars.project_name }}-old 2>/dev/null || true
      docker rename {{ .vars.project_name }}-new {{ .vars.project_name }}-current
      
      echo "✅ Deployment completed successfully"
```

## Running Shell Script Examples

```bash
# Run basic shell commands
rocketship run -af examples/shell-basic/rocketship.yaml

# Run comprehensive shell testing suite
rocketship run -af examples/shell-testing/rocketship.yaml

# Run with debug logging to see shell execution details
ROCKETSHIP_LOG=DEBUG rocketship run -af examples/shell-testing/rocketship.yaml
```

## Best Practices

### 1. Use Appropriate Exit Codes

```bash
# Good: Explicit exit codes
if [ ! -f "required-file.txt" ]; then
  echo "❌ Required file missing"
  exit 1
fi

# Good: Let commands fail naturally
make test  # Will exit with make's exit code
```

### 2. Quote Variables Properly

```bash
# Good: Quoted variables handle spaces
echo "Project: '{{ .vars.project_name }}'"
cp "{{ source_file }}" "{{ destination_path }}"

# Poor: Unquoted variables can break with spaces
echo Project: {{ .vars.project_name }}
```

### 3. Use Shell Safety Features

```bash
# Recommended shell script header
set -euo pipefail
# -e: Exit on any command failure
# -u: Exit on undefined variable usage  
# -o pipefail: Exit on any pipe command failure
```

### 4. Provide Clear Output

```bash
# Good: Clear, actionable output
echo "🔨 Building Docker image..."
echo "📦 Packaging artifacts..."
echo "✅ Deployment completed successfully"
echo "🌐 Application available at: https://app.example.com"

# Include timing information
start_time=$(date +%s)
# ... operations ...
end_time=$(date +%s)
echo "⏱️  Total execution time: $((end_time - start_time)) seconds"
```

### 5. Handle Secrets Securely

```bash
# Good: Use environment variables for secrets
curl -H "Authorization: Bearer $ROCKETSHIP_VAR_API_TOKEN" \
     https://api.example.com/deploy

# Poor: Don't expose secrets in command output
echo "Using token: $SECRET_TOKEN"  # This appears in logs!
```

The shell scripting plugin provides unlimited flexibility for integrating command-line tools, build processes, and system operations into your Rocketship testing workflows, making it perfect for CI/CD pipelines and infrastructure automation.