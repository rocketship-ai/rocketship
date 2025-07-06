# Browser Plugin - AI-Powered Web Automation

The browser plugin enables AI-driven browser automation within your test workflows, using browser-use to intelligently navigate websites, extract data, and validate web interfaces. Use it to test web applications, scrape data, monitor website functionality, or automate complex browser interactions.

## Key Features Demonstrated

- **AI-Driven Navigation**: Autonomous web browsing with natural language instructions
- **Visual Processing**: Optional visual analysis for enhanced accuracy
- **Viewport Control**: Custom browser window and viewport dimensions
- **Session Management**: Persistent browser sessions across test steps
- **Multi-Domain Support**: Control which domains the agent can navigate
- **Headless/Headful Modes**: Run with or without visible browser window
- **LLM Flexibility**: Support for OpenAI and Anthropic models
- **Data Extraction**: Extract structured data from web pages

## Prerequisites

Before using the browser plugin, you need:

1. **Browser-use installed**: The plugin will attempt to install it automatically
2. **LLM API Key**: Either OpenAI or Anthropic API key
3. **Chrome/Chromium**: Installed on your system (handled by Playwright)

```bash
# For OpenAI
export OPENAI_API_KEY=sk-your-key-here

# For Anthropic
export ANTHROPIC_API_KEY=sk-ant-your-key-here
```

### Updating browser-use

To update browser-use to the latest version (e.g., 0.4.2), run:

```bash
# Update browser-use
pip install --upgrade browser-use

# Or install a specific version
pip install browser-use==0.4.2

# Verify the installation
python3 -c "import browser_use; print(f'browser-use version: {browser_use.__version__}')"
```

Note: The browser plugin uses the Python `browser-use` library installed on your system. Updates to browser-use may introduce new features or breaking changes - check the [browser-use releases](https://github.com/browser-use/browser-use/releases) for details.

## Basic Configuration

```yaml
plugin: browser
config:
  task: "Your browser task here" # Required: instruction for the AI agent
  llm: # Required: LLM configuration
    provider: "openai" # or "anthropic"
    model: "gpt-4o" # or other supported models
    config:
      OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
  headless: true # Optional: run without visible browser
  timeout: "2m" # Optional: execution timeout
```

## Simple Example

```yaml
name: "Basic Browser Test"

tests:
  - name: "Check website content"
    steps:
      - name: "Visit and analyze homepage"
        plugin: browser
        config:
          task: "Navigate to https://example.com and tell me what the main heading says"
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
          headless: true
          timeout: "1m"
        save:
          - json_path: ".result"
            as: "page_analysis"
          - json_path: ".success"
            as: "success"
        assertions:
          - type: "json_path"
            path: ".success"
            expected: true

      - name: "Log results"
        plugin: log
        config:
          message: "Page analysis: {{ page_analysis }}"
```

## Comprehensive Configuration Example

```yaml
name: "Browser Plugin Feature Demo"

vars:
  target_site: "https://docs.rocketship.sh"
  search_term: "installation"

tests:
  - name: "Complete browser automation workflow"
    steps:
      # Basic navigation with custom viewport
      - name: "Mobile viewport test"
        plugin: browser
        config:
          task: |
            Navigate to {{ .vars.target_site }} and describe:
            1. The main navigation menu
            2. How the layout appears on mobile
            3. Any responsive design elements
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
          executor_type: "python"
          headless: false
          timeout: "3m"
          max_steps: 5
          use_vision: true
          viewport:
            width: 375
            height: 667
          browser_type: "chromium"
        save:
          - json_path: ".result"
            as: "mobile_analysis"
          - json_path: ".success"
            as: "mobile_success"
          - json_path: ".session_id"
            as: "session_id"

      # Desktop viewport with domain restrictions
      - name: "Desktop search test"
        plugin: browser
        config:
          task: |
            1. Navigate to {{ .vars.target_site }}
            2. Find and use the search functionality
            3. Search for "{{ .vars.search_term }}"
            4. Tell me what results you found
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
          headless: false
          timeout: "2m"
          max_steps: 10
          use_vision: true
          viewport:
            width: 1920
            height: 1080
          allowed_domains:
            - "docs.rocketship.sh"
            - "rocketship.sh"
        save:
          - json_path: ".result"
            as: "search_results"
          - json_path: ".extracted_data"
            as: "extracted_info"

      # Data extraction example
      - name: "Extract structured data"
        plugin: browser
        config:
          task: |
            Navigate to https://jsonplaceholder.typicode.com/users
            Extract the first 3 users' information including:
            - Name
            - Email
            - Company name
            Return as structured JSON data
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
          headless: true
          timeout: "2m"
          use_vision: false # Text extraction doesn't need vision
        save:
          - json_path: ".result"
            as: "user_data"
          - json_path: ".success"
            as: "extraction_success"

      # Log comprehensive results
      - name: "Log all results"
        plugin: log
        config:
          message: |
            🌐 Browser Automation Results:
            
            📱 Mobile Analysis:
            {{ mobile_analysis }}
            
            🔍 Search Results:
            {{ search_results }}
            
            📊 Extracted Data:
            {{ user_data }}
            
            ✅ Success Status: Mobile={{ mobile_success }}, Extraction={{ extraction_success }}
```

## Configuration Options

### Required Fields

| Field  | Description                                      | Example                              |
|--------|--------------------------------------------------|--------------------------------------|
| `task` | Natural language instruction for the browser agent | `"Navigate to site and click login"` |
| `llm`  | LLM configuration object                         | See LLM Configuration section        |

### Optional Fields

| Field             | Type    | Default    | Description                                    |
|-------------------|---------|------------|------------------------------------------------|
| `executor_type`   | string  | `"python"` | Executor type (only python supported)          |
| `timeout`         | string  | `"5m"`     | Execution timeout (e.g., "30s", "2m", "1h")    |
| `max_steps`       | integer | 50         | Maximum browser automation steps               |
| `browser_type`    | string  | `"chromium"` | Browser to use: chromium, chrome, edge       |
| `headless`        | boolean | true       | Run without visible browser window             |
| `use_vision`      | boolean | true       | Enable visual processing for better accuracy   |
| `session_id`      | string  | -          | Browser session ID for persistence             |
| `save_screenshots`| boolean | false      | Save screenshots during execution              |
| `allowed_domains` | array   | []         | Restrict navigation to specific domains        |
| `viewport`        | object  | 1920x1080  | Browser viewport dimensions                    |

### LLM Configuration

The `llm` object configures which AI model to use:

```yaml
llm:
  provider: "openai" # or "anthropic"
  model: "gpt-4o" # Model name
  config:
    # Provider-specific API keys
    OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    # OR for Anthropic:
    # ANTHROPIC_API_KEY: "{{ .env.ANTHROPIC_API_KEY }}"
```

Supported models:
- OpenAI: `gpt-4o`, `gpt-4`, `gpt-3.5-turbo`
- Anthropic: `claude-3-opus`, `claude-3-sonnet`, `claude-3-haiku`

### Viewport Configuration

Control browser window and viewport dimensions:

```yaml
viewport:
  width: 1280  # Width in pixels
  height: 720  # Height in pixels
```

Common viewport sizes:
- Mobile: 375x667 (iPhone), 360x640 (Android)
- Tablet: 768x1024 (iPad)
- Desktop: 1920x1080, 1366x768, 1280x720

Note: In headless mode, only the viewport affects rendering. In headful mode, both the browser window and viewport are set to these dimensions.

## Response Structure

The browser plugin returns a JSON response with:

```json
{
  "success": true,
  "result": "AI agent's description of what it found/did",
  "session_id": "unique-session-identifier",
  "steps": [...], // Detailed automation steps
  "screenshots": [...], // If save_screenshots is true
  "extracted_data": {...}, // Any structured data extracted
  "error": "Error message if success is false"
}
```

## Save Operations

### Basic Data Extraction

```yaml
save:
  - json_path: ".result"
    as: "browser_output"
  - json_path: ".success"
    as: "execution_success"
```

### Complete Response Capture

```yaml
save:
  - json_path: ".result"
    as: "ai_analysis"
  - json_path: ".session_id"
    as: "browser_session"
  - json_path: ".extracted_data"
    as: "structured_data"
  - json_path: ".steps"
    as: "automation_steps"
  - json_path: ".screenshots"
    as: "screenshots_list"
```

### Optional Fields

```yaml
save:
  - json_path: ".result"
    as: "required_result"
  - json_path: ".optional_metadata"
    as: "extra_info"
    required: false # Won't fail if missing
```

## Assertions

Validate browser automation results:

```yaml
assertions:
  - type: "json_path"
    path: ".success"
    expected: true
```

## Template Variables

Use data from previous steps in browser tasks:

```yaml
# From previous HTTP responses
task: "Navigate to {{ api_base_url }} and login with {{ test_credentials }}"

# From configuration variables
task: "Go to {{ .vars.target_site }} and search for {{ .vars.search_query }}"

# Multi-line tasks with context
task: |
  Previous analysis found: {{ previous_result }}
  Now navigate to the contact page and:
  1. Fill out the form with this data
  2. Submit and capture the confirmation
```

## Use Cases

### Web Application Testing

```yaml
- name: "Test login flow"
  plugin: browser
  config:
    task: |
      1. Navigate to {{ app_url }}/login
      2. Enter username: testuser@example.com
      3. Enter password: testpass123
      4. Click login button
      5. Verify you see the dashboard
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    headless: false
    timeout: "2m"
```

### Data Scraping

```yaml
- name: "Extract product information"
  plugin: browser
  config:
    task: |
      Navigate to {{ product_url }}
      Extract:
      - Product name
      - Price
      - Availability
      - Description
      Return as JSON: {"name": "", "price": "", "available": true/false, "description": ""}
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    use_vision: true # Helps with complex layouts
```

### Visual Regression Testing

```yaml
- name: "Check page layout"
  plugin: browser
  config:
    task: |
      Navigate to {{ page_url }}
      Analyze the visual layout and report:
      1. Any broken elements
      2. Missing images
      3. Layout issues
      4. Text overflow problems
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    use_vision: true
    save_screenshots: true
    viewport:
      width: 1920
      height: 1080
```

### Multi-Step Form Testing

```yaml
- name: "Complete multi-page form"
  plugin: browser
  config:
    task: |
      1. Go to {{ form_url }}
      2. Page 1: Fill personal information (John Doe, john@example.com)
      3. Click Next
      4. Page 2: Select "Premium" plan
      5. Click Next
      6. Page 3: Enter payment details (use test card 4111111111111111)
      7. Submit form
      8. Capture confirmation number
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    max_steps: 20
    timeout: "5m"
```

### Responsive Design Testing

```yaml
- name: "Test responsive design"
  plugin: browser
  config:
    task: "Navigate to {{ site_url }} and describe how the navigation menu behaves"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    headless: false
    viewport:
      width: 375  # Mobile width
      height: 667
```

## Running Examples

```bash
# Run basic browser test
rocketship run -af examples/browser-testing/rocketship.yaml

# Run with OpenAI API key
OPENAI_API_KEY=your-key rocketship run -af examples/browser-testing/rocketship.yaml

# Run viewport comparison test
OPENAI_API_KEY=your-key rocketship run -af examples/browser-testing/viewport-test.yaml

# Run in debug mode to see browser interactions
OPENAI_API_KEY=your-key ROCKETSHIP_LOG=DEBUG rocketship run -af examples/browser-testing/rocketship.yaml
```

## Best Practices

### 1. Clear, Specific Instructions

```yaml
# Good: Specific, step-by-step instructions
task: |
  1. Navigate to https://example.com/products
  2. Click on "Electronics" category
  3. Find the first laptop listing
  4. Extract the price and product name

# Avoid: Vague instructions
task: "Go to the website and find laptop prices"
```

### 2. Use Appropriate Timeouts

```yaml
# Quick navigation tasks
timeout: "30s"

# Complex multi-step processes
timeout: "5m"

# Data extraction from large pages
timeout: "2m"
```

### 3. Choose Headless Mode Wisely

```yaml
# Use headless for:
# - CI/CD pipelines
# - Data extraction
# - High-volume testing
headless: true

# Use headful for:
# - Debugging
# - Visual testing
# - Development
headless: false
```

### 4. Optimize Vision Usage

```yaml
# Enable for visual tasks
use_vision: true # Layout testing, visual elements

# Disable for text-only tasks
use_vision: false # Form filling, text extraction
```

### 5. Control Navigation Scope

```yaml
# Restrict to specific domains for security
allowed_domains:
  - "myapp.com"
  - "api.myapp.com"
  - "cdn.myapp.com"
```

### 6. Handle Dynamic Content

```yaml
task: |
  1. Navigate to {{ dynamic_url }}
  2. Wait for the loading spinner to disappear
  3. Wait for the content section to be visible
  4. Then extract the data
max_steps: 10 # Allow enough steps for waiting
```

## Troubleshooting

### Common Issues

**"Browser automation failed: Python executor not available"**
- The plugin will attempt to install browser-use automatically
- Ensure Python 3.8+ is installed on your system
- Check logs for specific installation errors

**"Failed to launch browser"**
- Ensure Chrome/Chromium is installed
- For headless mode on servers, install: `apt-get install chromium-browser`
- Check if another browser instance is using the same profile

**"Viewport not changing"**
- Viewport settings only work with headless=false for window size
- The viewport always affects page rendering
- Check that viewport values are integers, not strings

**"Navigation blocked"**
- Check `allowed_domains` configuration
- Some sites block automation - try with `use_vision: true`
- Increase `max_steps` for complex navigation

**"Empty or unclear results"**
- Make browser task instructions more specific
- Enable vision with `use_vision: true` for better accuracy
- Check if the page loaded completely before extraction
- Increase timeout for slow-loading pages

**"Session not persisting"**
- Save and reuse `session_id` between steps
- Browser sessions are cleared after each test by default
- Use the same `browser_type` across steps

### Debug Mode

Run with debug logging to see detailed browser interactions:

```bash
ROCKETSHIP_LOG=DEBUG rocketship run -af your-test.yaml
```

This will show:
- Browser launch commands
- Page navigation details
- AI agent decisions
- Screenshot captures (if enabled)

The browser plugin enables sophisticated web testing scenarios that combine the flexibility of AI-driven automation with the reliability of structured test frameworks, making it ideal for testing modern web applications with dynamic content and complex interactions.