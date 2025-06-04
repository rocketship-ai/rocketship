# Agent Plugin - AI-Powered Testing

The agent plugin integrates Claude Code into your test workflows, enabling AI-powered analysis, data processing, and intelligent test validation. Use it to analyze API responses, generate test data, validate complex business logic, or create intelligent test assertions.

## Key Features Demonstrated

- **AI Analysis**: Analyze API responses, logs, and test data with Claude
- **Multiple Output Formats**: Support for JSON, text, and streaming formats
- **Session Management**: Continue conversations across test steps
- **Template Integration**: Use previous test results in AI prompts
- **Metadata Extraction**: Access cost, duration, and session information
- **Flexible Timeouts**: Configure execution timeouts for different use cases
- **System Prompts**: Customize AI behavior with custom instructions

## Prerequisites

Before using the agent plugin, you need:

1. **Claude Code CLI installed**: Install via `npm install -g @anthropic-ai/claude-code` or use Homebrew
2. **Anthropic API Key**: Set the `ANTHROPIC_API_KEY` environment variable
3. **Authentication**: Run `claude login` to authenticate with Claude Code

```bash
# Install Claude Code CLI
npm install -g @anthropic-ai/claude-code
# OR
brew install claude

# Set your API key
export ANTHROPIC_API_KEY=sk-ant-your-key-here

# Login to Claude Code
claude login
```

## Basic Configuration

```yaml
plugin: agent
config:
  agent: "claude-code" # Only supported agent type
  prompt: "Your prompt here" # Required: instruction for Claude
  mode: "single" # Optional: single, continue, resume
  output_format: "json" # Optional: json, text, streaming-json
  timeout: "30s" # Optional: execution timeout
```

## Simple Example

```yaml
name: "Basic Agent Analysis"
tests:
  - name: "Analyze API response"
    steps:
      - name: "Get user data"
        plugin: http
        config:
          method: GET
          url: "https://jsonplaceholder.typicode.com/users/1"
        save:
          - json_path: ".name"
            as: "user_name"
          - json_path: ".email"
            as: "user_email"

      - name: "AI analysis of user data"
        plugin: agent
        config:
          agent: "claude-code"
          prompt: |
            Analyze this user data:
            Name: {{ user_name }}
            Email: {{ user_email }}

            Is this a valid user profile? Respond with JSON:
            {"valid": true/false, "issues": ["list", "of", "issues"]}
          output_format: "json"
        save:
          - json_path: ".response"
            as: "validation_result"

      - name: "Log validation result"
        plugin: log
        config:
          message: "User validation: {{ validation_result }}"
```

## Comprehensive Configuration Example

```yaml
name: "Agent Plugin Feature Demo"
vars:
  api_endpoint: "https://jsonplaceholder.typicode.com/posts/1"
  system_instructions: "You are a technical analyst. Be concise and precise."

tests:
  - name: "Complete agent workflow"
    steps:
      # Get test data
      - name: "Fetch API data"
        plugin: http
        config:
          method: GET
          url: "{{ .vars.api_endpoint }}"
        save:
          - json_path: ".title"
            as: "post_title"
          - json_path: ".body"
            as: "post_body"

      # JSON output with system prompt
      - name: "Structured analysis"
        plugin: agent
        config:
          agent: "claude-code"
          prompt: |
            Analyze this content:
            Title: {{ post_title }}
            Body: {{ post_body }}

            Provide JSON analysis: {
              "sentiment": "positive/negative/neutral",
              "topics": ["array", "of", "topics"],
              "word_count": number,
              "summary": "brief summary"
            }
          mode: "single"
          output_format: "json"
          system_prompt: "{{ .vars.system_instructions }}"
          timeout: "45s"
          save_full_response: true
        save:
          - json_path: ".response"
            as: "structured_analysis"
          - json_path: ".cost"
            as: "analysis_cost"
          - json_path: ".session_id"
            as: "session_id"
        assertions:
          - type: "json_path"
            path: ".success"
            expected: true

      # Text output with multi-turn
      - name: "Detailed analysis"
        plugin: agent
        config:
          agent: "claude-code"
          prompt: "Expand on the analysis: {{ structured_analysis }}"
          mode: "single"
          output_format: "text"
          max_turns: 2
          timeout: "60s"
        save:
          - json_path: ".response"
            as: "detailed_analysis"

      # Optional field extraction
      - name: "Extract key insights"
        plugin: agent
        config:
          agent: "claude-code"
          prompt: "List the top 3 insights from: {{ detailed_analysis }}"
          output_format: "text"
        save:
          - json_path: ".response"
            as: "key_insights"
          - json_path: ".nonexistent_field"
            as: "optional_data"
            required: false

      # Final summary
      - name: "Log comprehensive results"
        plugin: log
        config:
          message: |
            🤖 AI Analysis Complete:
            📊 Structured: {{ structured_analysis }}
            💰 Cost: ${{ analysis_cost }}
            📝 Detailed: {{ detailed_analysis }}
            🎯 Insights: {{ key_insights }}
            ❓ Optional: {{ optional_data }}
```

## Configuration Options

### Required Fields

| Field    | Description                                 | Example                                   |
| -------- | ------------------------------------------- | ----------------------------------------- |
| `agent`  | Agent type (only "claude-code" supported)   | `"claude-code"`                           |
| `prompt` | Instruction for Claude (supports templates) | `"Analyze this data: {{ api_response }}"` |

### Optional Fields

| Field                | Type    | Default    | Description                                     |
| -------------------- | ------- | ---------- | ----------------------------------------------- |
| `mode`               | string  | `"single"` | Execution mode: `single`, `continue`, `resume`  |
| `output_format`      | string  | `"json"`   | Output format: `json`, `text`, `streaming-json` |
| `timeout`            | string  | `"30s"`    | Execution timeout (e.g., "30s", "2m")           |
| `system_prompt`      | string  | -          | Custom system instructions                      |
| `max_turns`          | integer | 1          | Maximum conversation turns                      |
| `session_id`         | string  | -          | Session ID for resume mode                      |
| `continue_recent`    | boolean | false      | Continue most recent conversation               |
| `save_full_response` | boolean | true       | Save complete response to context               |

## Output Formats

### JSON Format

Returns structured metadata with the Claude response:

```yaml
output_format: "json"
```

Response structure:

```json
{
  "type": "result",
  "subtype": "success",
  "result": "Claude's response here",
  "session_id": "unique-session-id",
  "cost_usd": 0.003,
  "duration_ms": 1234,
  "num_turns": 1,
  "is_error": false
}
```

### Text Format

Returns Claude's response as plain text:

```yaml
output_format: "text"
```

Best for simple analysis or when you want Claude's raw response.

## Execution Modes

### Single Mode (Default)

Execute a one-time prompt:

```yaml
mode: "single"
prompt: "Analyze this data: {{ api_response }}"
```

### Continue Mode

Continue the most recent conversation:

```yaml
mode: "continue"
continue_recent: true
prompt: "Now summarize our discussion"
```

### Resume Mode

Resume a specific conversation session:

```yaml
mode: "resume"
session_id: "{{ previous_session_id }}"
prompt: "Continue from where we left off"
```

## Save Operations

### Basic Response Extraction

```yaml
save:
  - json_path: ".response"
    as: "ai_analysis"
```

### Metadata Extraction

```yaml
save:
  - json_path: ".response"
    as: "analysis_content"
  - json_path: ".cost"
    as: "execution_cost"
  - json_path: ".duration"
    as: "execution_time"
  - json_path: ".session_id"
    as: "session_id"
  - json_path: ".success"
    as: "success_status"
```

### Optional Fields

```yaml
save:
  - json_path: ".response"
    as: "required_analysis"
  - json_path: ".optional_metadata"
    as: "optional_data"
    required: false # Won't fail if field doesn't exist
```

## Assertions

Validate agent execution results:

```yaml
assertions:
  - type: "json_path"
    path: ".success"
    expected: true
  - type: "json_path"
    path: ".exit_code"
    expected: 0
  - type: "json_path"
    path: ".cost"
    expected: 0 # Cost should be 0 for testing
```

## Template Variables

Use data from previous steps in agent prompts:

```yaml
# From HTTP responses
prompt: "Analyze this API data: {{ api_response }}"

# From configuration
prompt: "Process {{ .vars.user_data }} according to {{ .vars.business_rules }}"

# From previous agent results
prompt: "Based on {{ previous_analysis }}, what are the next steps?"

# Multi-line prompts with templates
prompt: |
  Previous Analysis: {{ step1_result }}
  New Data: {{ step2_data }}

  Compare these results and identify:
  1. Key differences
  2. Trending patterns
  3. Recommendations
```

## Use Cases

### API Response Analysis

```yaml
- name: "Analyze API health"
  plugin: agent
  config:
    agent: "claude-code"
    prompt: |
      API Response Analysis:
      Status: {{ response_status }}
      Time: {{ response_time }}ms
      Size: {{ response_size }} bytes

      Rate this API's health (1-10) and explain issues.
    output_format: "json"
```

### Test Data Validation

```yaml
- name: "Validate business rules"
  plugin: agent
  config:
    agent: "claude-code"
    prompt: |
      User Data: {{ user_profile }}
      Business Rules: {{ .vars.validation_rules }}

      Does this user profile comply with business rules?
      Return: {"compliant": boolean, "violations": []}
```

### Intelligent Assertions

```yaml
- name: "Smart content validation"
  plugin: agent
  config:
    agent: "claude-code"
    prompt: |
      Content: {{ page_content }}

      Check for:
      - Appropriate language
      - Complete information
      - Professional tone

      Score 1-10 with reasoning.
```

### Log Analysis

```yaml
- name: "Analyze error logs"
  plugin: agent
  config:
    agent: "claude-code"
    prompt: |
      Error Logs: {{ error_logs }}

      Categorize errors and suggest fixes:
      {"categories": [], "critical_count": 0, "suggestions": []}
```

## Running Examples

```bash
# Run basic agent example
rocketship run -af examples/agent-testing/rocketship.yaml

# Run with API key environment variable
ANTHROPIC_API_KEY=your-key rocketship run -af examples/agent-testing/rocketship.yaml

# Run comprehensive test suite
rocketship run -af examples/agent-testing/comprehensive-test.yaml
```

## Best Practices

### 1. Clear, Specific Prompts

```yaml
# Good: Specific instructions
prompt: |
  Analyze this JSON response for data quality:
  {{ api_response }}

  Check: completeness, format validity, business logic
  Return: {"score": 1-10, "issues": ["specific", "problems"]}

# Avoid: Vague prompts
prompt: "Check this data: {{ api_response }}"
```

### 2. Use Appropriate Output Formats

```yaml
# Use JSON for structured data extraction
output_format: "json"
prompt: "Return analysis as: {\"score\": number, \"issues\": []}"

# Use text for explanations and summaries
output_format: "text"
prompt: "Explain the security implications of this configuration"
```

### 3. Set Reasonable Timeouts

```yaml
# Quick analysis
timeout: "15s"

# Complex analysis
timeout: "60s"

# Multi-turn conversations
timeout: "120s"
```

### 4. Handle Optional Data

```yaml
save:
  - json_path: ".response"
    as: "analysis"
  - json_path: ".metadata.extra"
    as: "extra_info"
    required: false # Won't fail test if missing
```

### 5. Use System Prompts for Consistency

```yaml
vars:
  analyst_prompt: "You are a security analyst. Be thorough and highlight risks."

config:
  system_prompt: "{{ .vars.analyst_prompt }}"
  prompt: "Analyze this configuration: {{ config_data }}"
```

## Troubleshooting

### Common Issues

**"claude command not found"**

- Install Claude Code CLI: `npm install -g @anthropic-ai/claude-code`
- Verify installation: `which claude`

**"ANTHROPIC_API_KEY environment variable is required"**

- Set your API key: `export ANTHROPIC_API_KEY=sk-ant-your-key`
- Get an API key from https://console.anthropic.com/

**"Authentication required"**

- Login to Claude Code: `claude login`

**Empty responses**

- Check your prompt is clear and specific
- Verify template variables are being substituted correctly
- Increase timeout for complex analysis

**JSON parsing errors**

- Use `output_format: "text"` for debugging
- Check Claude's actual response format
- Ensure prompts request valid JSON structure

The agent plugin enables powerful AI-driven testing workflows, making your tests more intelligent and capable of handling complex validation scenarios that would be difficult to implement with traditional assertion methods.
