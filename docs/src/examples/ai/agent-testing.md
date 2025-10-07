# Agent Plugin - AI-Powered Test Analysis

Use AI to analyze test data, validate complex responses, and make intelligent assertions. The agent plugin executes LLM prompts within your test workflow.

## Prerequisites

```bash
# Set API key (OpenAI or Anthropic)
export OPENAI_API_KEY=sk-your-key-here
# OR
export ANTHROPIC_API_KEY=sk-ant-your-key-here
```

## Basic Usage

```yaml
- name: "Analyze API response"
  plugin: agent
  config:
    prompt: "Is this response valid? {{ api_response }}"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    output_format: "text"
  save:
    - json_path: ".result"
      as: "analysis"
```

## Configuration

### Required Fields

```yaml
config:
  prompt: "Your question or instruction"  # Required: what to ask the LLM
  llm:                                     # Required: LLM configuration
    provider: "openai"                     # "openai" or "anthropic"
    model: "gpt-4o"                        # Model name
    config:
      OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

### Optional Fields

```yaml
config:
  output_format: "json"        # "json" or "text" (default: "json")
  system_prompt: "..."         # System instructions for the LLM
  timeout: "1m"                # Max execution time (default: 1m)
  mode: "single"               # "single", "continue", or "resume"
```

## LLM Providers

### OpenAI

```yaml
llm:
  provider: "openai"
  model: "gpt-4o"  # or "gpt-4", "gpt-3.5-turbo"
  config:
    OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

### Anthropic

```yaml
llm:
  provider: "anthropic"
  model: "claude-3-5-sonnet-20241022"  # or other Claude models
  config:
    ANTHROPIC_API_KEY: "{{ .env.ANTHROPIC_API_KEY }}"
```

## Output Formats

### JSON Format

Request structured output:

```yaml
- name: "Validate user data"
  plugin: agent
  config:
    prompt: |
      Analyze this user data: {{ user_json }}
      Return JSON with:
      {
        "valid": boolean,
        "issues": [list of issues],
        "confidence": number (0-100)
      }
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    output_format: "json"
  save:
    - json_path: ".result.valid"
      as: "is_valid"
    - json_path: ".result.confidence"
      as: "confidence_score"
  assertions:
    - type: "json_path"
      path: ".result.valid"
      expected: true
```

### Text Format

Simple yes/no or descriptive answers:

```yaml
- name: "Check response quality"
  plugin: agent
  config:
    prompt: "Does this API response contain complete user information? {{ response }}"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    output_format: "text"
  save:
    - json_path: ".result"
      as: "answer"
```

## Execution Modes

### Single Mode (Default)

One-off prompt execution:

```yaml
mode: "single"  # Default, can be omitted
```

### Continue Mode

Multi-turn conversation with previous context:

```yaml
- name: "Initial analysis"
  plugin: agent
  config:
    prompt: "Analyze this data: {{ data }}"
    mode: "single"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"

- name: "Follow-up question"
  plugin: agent
  config:
    prompt: "Based on your previous analysis, what are the top 3 issues?"
    mode: "continue"  # Continues previous conversation
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

### Resume Mode

Pick up from a specific conversation:

```yaml
- name: "Resume analysis"
  plugin: agent
  config:
    prompt: "Continue from where we left off"
    mode: "resume"
    conversation_id: "{{ previous_conversation_id }}"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

## Common Use Cases

### API Response Validation

```yaml
- name: "Fetch user data"
  plugin: http
  config:
    url: "{{ .env.API_BASE_URL }}/users/123"
  save:
    - json_path: "."
      as: "user_data"

- name: "Validate completeness"
  plugin: agent
  config:
    prompt: |
      Check if this user data is complete: {{ user_data }}
      Required fields: id, email, name, created_at
      Return JSON: {"valid": true/false, "missing_fields": []}
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    output_format: "json"
  assertions:
    - type: "json_path"
      path: ".result.valid"
      expected: true
```

### Intelligent Test Data Generation

```yaml
- name: "Generate test data"
  plugin: agent
  config:
    prompt: |
      Generate 5 realistic user profiles in JSON format.
      Each should have: name, email, age, city
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    output_format: "json"
  save:
    - json_path: ".result"
      as: "test_users"
```

### Log Analysis

```yaml
- name: "Analyze error logs"
  plugin: agent
  config:
    prompt: |
      Analyze these error logs: {{ error_logs }}
      Identify patterns and root causes.
      Return JSON: {
        "error_count": number,
        "patterns": [strings],
        "root_cause": string,
        "severity": "low|medium|high"
      }
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    output_format: "json"
```

### Complex Assertions

```yaml
- name: "Intelligent validation"
  plugin: agent
  config:
    prompt: |
      Compare expected vs actual responses:
      Expected: {{ expected_response }}
      Actual: {{ actual_response }}

      Are they semantically equivalent, ignoring minor formatting differences?
      Return JSON: {"equivalent": true/false, "differences": []}
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    output_format: "json"
  assertions:
    - type: "json_path"
      path: ".result.equivalent"
      expected: true
```

## Best Practices

**Clear, Specific Prompts**: Tell the LLM exactly what you want
```yaml
# ❌ Vague
prompt: "Check this data"

# ✅ Specific
prompt: "Validate that this user data contains all required fields (id, email, name) and email is valid format"
```

**Structured Output**: Use JSON for reliable parsing
```yaml
output_format: "json"  # Easier to assert on
# vs
output_format: "text"  # Harder to parse
```

**System Prompts**: Set consistent behavior
```yaml
system_prompt: "You are a test validation assistant. Always return concise, structured JSON responses."
```

**Handle Optional Data**: Use `required: false` for optional saves
```yaml
save:
  - json_path: ".result.optional_field"
    as: "optional_value"
    required: false
```

## Troubleshooting

**API key errors**: Check environment variables
```bash
echo $OPENAI_API_KEY
echo $ANTHROPIC_API_KEY
```

**Timeout errors**: Increase timeout for complex prompts
```yaml
timeout: "2m"  # For complex analysis
```

**JSON parsing fails**: Ensure output_format is "json" and prompt requests JSON
```yaml
output_format: "json"
prompt: "Return JSON with structure: {\"field\": \"value\"}"
```

## Running Examples

```bash
# Run agent tests
rocketship run -af examples/agent-testing/rocketship.yaml

# With specific env file
rocketship run -af examples/agent-testing/rocketship.yaml \
  --env-file .env
```
