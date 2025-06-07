# Rocketship MCP Server Usage Examples

This document shows practical examples of using the Rocketship MCP server with AI coding agents.

## Agent Conversations Examples

### 1. Generate Complete Test Suite

**Agent Prompt:**
```
I need to create a comprehensive test suite for my Node.js API. I have user management endpoints, authentication, and a PostgreSQL database. Can you generate the complete Rocketship test structure?
```

**MCP Tool Call:**
```json
{
  "tool": "scan_and_generate_test_suite",
  "arguments": {
    "project_root": ".",
    "environments": ["staging", "prod"],
    "codebase_analysis": {
      "api_endpoints": [
        {"method": "GET", "path": "/api/v1/users"},
        {"method": "POST", "path": "/api/v1/users"},
        {"method": "POST", "path": "/api/v1/auth/login"}
      ],
      "database_schemas": [
        {"table": "users", "columns": ["id", "email", "name"]}
      ],
      "service_configs": [
        {"name": "postgres", "type": "database"}
      ]
    }
  }
}
```

**Generated Structure:**
```
.rocketship/
├── staging-vars.yaml
├── prod-vars.yaml
├── api-tests/
│   └── rocketship.yaml
├── database-tests/
│   └── rocketship.yaml
└── integration-tests/
    └── rocketship.yaml
```

### 2. Generate Specific Test from Prompt

**Agent Prompt:**
```
Create a test that validates user registration API including error cases for duplicate emails and invalid passwords
```

**MCP Tool Call:**
```json
{
  "tool": "generate_test_from_prompt", 
  "arguments": {
    "prompt": "Create a test that validates user registration API including error cases for duplicate emails and invalid passwords",
    "test_type": "api",
    "environment": "staging",
    "context": {
      "base_url": "https://api-staging.myapp.com",
      "auth_type": "bearer_token"
    }
  }
}
```

**Generated Test:**
```yaml
name: User Registration API Tests
description: "Generated from prompt: Create a test that validates user registration API including error cases for duplicate emails and invalid passwords"
vars:
  base_url: "https://api-staging.myapp.com"
  timeout: 30
  auth_token: "{{ .vars.auth.bearer_token }}"
tests:
  - name: Test User Registration API
    steps:
      - name: HTTP request
        plugin: http
        config:
          method: GET
          url: "{{ .vars.base_url }}/endpoint"
          headers:
            Authorization: "Bearer {{ .vars.auth_token }}"
        assertions:
          - type: status_code
            expected: 200
```

### 3. Validate and Fix Test Files

**Agent Prompt:**
```
I created a Rocketship test but it's not working. Can you validate it and tell me what's wrong?
```

**MCP Tool Call:**
```json
{
  "tool": "validate_test_file",
  "arguments": {
    "file_path": ".rocketship/api-tests/rocketship.yaml"
  }
}
```

**Response Example:**
```
❌ Test file validation failed!

📁 File: .rocketship/api-tests/rocketship.yaml

Errors:
• Invalid plugin name 'httpx' - should be 'http'
• Missing required field 'url' in HTTP config
• Assertion type 'status' should be 'status_code'

Suggestions:
• Verify plugin name is correct and supported
• Add missing required fields to configuration
• Review assertion syntax and expected values
```

### 4. Run Tests with Analysis

**Agent Prompt:**
```
Run my user API tests and analyze any failures
```

**MCP Tool Call:**
```json
{
  "tool": "run_and_analyze_tests",
  "arguments": {
    "file_path": ".rocketship/api-tests/rocketship.yaml",
    "environment": "staging",
    "var_file": ".rocketship/staging-vars.yaml"
  }
}
```

**Success Response:**
```
✅ Test execution successful!

📁 File: .rocketship/api-tests/rocketship.yaml
🌍 Environment: staging
⏱️ Exit code: 0

Output summary:
Starting Rocketship test execution...
✅ Test 1: API Health Check - PASSED
✅ Test 2: User Registration - PASSED
✅ Test 3: User Authentication - PASSED
All tests completed successfully!

Suggestions:
• All tests passed successfully!
```

**Failure Response:**
```
❌ Test execution failed!

📁 File: .rocketship/api-tests/rocketship.yaml
🌍 Environment: staging
⏱️ Exit code: 1

Failure Analysis:
Authentication failed - check API keys and tokens; Request timed out - service may be slow or overloaded

Output:
❌ Test 2: User Registration - FAILED
   HTTP 401: Unauthorized
   Request to POST /api/v1/users failed
❌ Test 3: User Authentication - TIMEOUT
   Request timed out after 30 seconds

Suggestions:
• Verify API keys and authentication tokens
• Check if credentials have expired
• Increase timeout values in test configuration
• Check if the service is experiencing high load
```

### 5. Git Diff Analysis for Test Maintenance

**Agent Prompt:**
```
I just made changes to my API endpoints. Can you analyze what tests need to be updated?
```

**MCP Tool Call:**
```json
{
  "tool": "analyze_git_diff",
  "arguments": {
    "base_branch": "main",
    "feature_branch": "feature/new-user-endpoints"
  }
}
```

**Response:**
```
📊 Git Diff Analysis Complete!

🔀 Comparing: main...feature/new-user-endpoints
📁 Repository: .
🟡 Confidence: medium

File Changes:
• Modified: 3 files
  - src/routes/users.js
  - src/controllers/userController.js
  - src/models/User.js
• Added: 1 files
  - src/routes/admin.js

Test Suggestions:
• Add integration tests for new API endpoints: src/routes/admin.js
• Update existing tests for modified endpoints: src/routes/users.js, src/controllers/userController.js

💡 These suggestions are based on file change patterns. Human review is recommended before implementing changes.
```

## Environment Variables Setup

For the generated tests to work, set these environment variables:

```bash
# Staging Environment
export DB_HOST_STAGING="staging-db.example.com"
export DB_USER_STAGING="staging_user"
export DB_PASSWORD_STAGING="staging_password"
export DB_NAME_STAGING="staging_db"
export API_KEY_STAGING="sk-staging-api-key-123"
export BEARER_TOKEN_STAGING="staging-bearer-token-456"

# Production Environment  
export DB_HOST_PROD="prod-db.example.com"
export DB_USER_PROD="prod_user"
export DB_PASSWORD_PROD="prod_password"
export DB_NAME_PROD="prod_db"
export API_KEY_PROD="sk-prod-api-key-789"
export BEARER_TOKEN_PROD="prod-bearer-token-012"
```

## Complete Workflow Example

**Agent Conversation:**
```
Human: I'm building a new microservice with user management and need comprehensive tests.

Agent: I'll help you create a comprehensive test suite for your microservice. Let me analyze your codebase and generate the appropriate test structure.

*Uses scan_and_generate_test_suite with codebase analysis*

✅ I've generated a complete test suite structure with:
- API integration tests for your user endpoints  
- Database tests for your user schema
- Environment configurations for staging and prod
- Integration tests for end-to-end workflows

Human: Great! Now I need a specific test for user password validation with multiple error cases.

Agent: I'll create a detailed test for password validation scenarios.

*Uses generate_test_from_prompt*

✅ Created comprehensive password validation test with:
- Valid password scenarios
- Too short password errors
- Missing special characters
- Common password detection

Human: Perfect! Let me validate this test before running it.

Agent: *Uses validate_test_file*

✅ Test validation passed! The file is ready to run.

Human: Run the tests and let me know if anything fails.

Agent: *Uses run_and_analyze_tests*

✅ All password validation tests passed! Your API correctly handles:
- Strong password acceptance
- Weak password rejection with proper error messages
- Rate limiting for repeated attempts

Human: I just updated my user API to add email verification. What tests should I add?

Agent: *Uses analyze_git_diff*

📊 Detected changes to user registration flow. I recommend:
- Add tests for email verification endpoint
- Test email verification token validation
- Update existing registration tests to check verification status
- Add timeout tests for verification tokens

Should I generate these tests for you?
```

## Best Practices for Using the MCP Server

### 1. Provide Rich Codebase Context

The more context you provide about your codebase, the better the generated tests:

```json
{
  "api_endpoints": [
    {
      "method": "POST",
      "path": "/api/v1/users",
      "description": "Create user with email verification",
      "auth_required": false,
      "request_body": {"email": "string", "password": "string"},
      "responses": {"201": "user created", "400": "validation error"}
    }
  ]
}
```

### 2. Use Environment-Specific Variables

Always use the generated environment variable files:

```bash
# Run staging tests
rocketship run -af .rocketship/api-tests/rocketship.yaml --var-file .rocketship/staging-vars.yaml

# Run production tests
rocketship run -af .rocketship/api-tests/rocketship.yaml --var-file .rocketship/prod-vars.yaml
```

### 3. Regular Git Diff Analysis

Use git diff analysis as part of your development workflow:

```bash
# Before merging a PR
git checkout feature-branch
# Agent analyzes: "What tests need updating for this feature?"
```

### 4. Iterative Test Refinement

1. Generate initial tests from prompts
2. Validate and fix any issues
3. Run tests and analyze failures
4. Refine based on real API behavior
5. Add edge cases and error scenarios

## Integration with CI/CD

The generated tests work seamlessly with CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Run Rocketship Tests
  run: |
    rocketship run -af .rocketship/api-tests/rocketship.yaml \
      --var-file .rocketship/staging-vars.yaml
```

## Troubleshooting

### Common Issues

1. **"rocketship command not found"**
   - Ensure Rocketship CLI is installed
   - Check PATH configuration

2. **Environment variable errors**
   - Verify all required environment variables are set
   - Check variable file syntax

3. **Test validation failures**
   - Use the validate_test_file tool
   - Check Rocketship schema documentation

4. **Git diff analysis errors**
   - Ensure you're in a git repository
   - Check branch names exist

The Rocketship MCP server makes test creation and maintenance significantly easier by leveraging AI to understand your codebase and generate appropriate test configurations automatically.