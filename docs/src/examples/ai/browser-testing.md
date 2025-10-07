# Browser Plugin - AI-Powered Web Automation

Test web applications using AI-driven browser automation. The browser plugin uses natural language instructions to navigate websites, extract data, and validate interfaces.

## Prerequisites

```bash
# Install dependencies (automatic on first use)
pip install browser-use playwright
playwright install chromium

# Set API key (OpenAI or Anthropic)
export OPENAI_API_KEY=sk-your-key-here
# OR
export ANTHROPIC_API_KEY=sk-ant-your-key-here
```

## Basic Usage

```yaml
- name: "Check website content"
  plugin: browser
  config:
    task: "Navigate to https://example.com and extract the main heading"
    llm:
      provider: "openai"  # or "anthropic"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    headless: true
    timeout: "1m"
  save:
    - json_path: ".result"
      as: "page_content"
  assertions:
    - type: "json_path"
      path: ".success"
      expected: true
```

## Configuration

### Required Fields

```yaml
config:
  task: "Natural language instruction"  # What the browser should do
  llm:                                   # LLM provider configuration
    provider: "openai"                   # "openai" or "anthropic"
    model: "gpt-4o"                      # Model name
    config:
      OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

### Optional Fields

```yaml
config:
  headless: true               # Run without visible browser (default: true)
  timeout: "2m"                # Max execution time (default: 2m)
  use_vision: false            # Enable visual analysis (default: false)
  max_actions_per_step: 10     # Action limit per step (default: 10)
  allowed_domains:             # Restrict navigation (optional)
    - "example.com"
    - "api.example.com"
  viewport:                    # Custom browser size
    width: 1920
    height: 1080
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

## Save & Assert

### Extract Data

```yaml
- name: "Scrape product info"
  plugin: browser
  config:
    task: "Go to https://example.com/product and extract the price and title"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
  save:
    - json_path: ".result"
      as: "product_data"
    - json_path: ".actions_taken"
      as: "action_count"
```

### Assert Success

```yaml
assertions:
  - type: "json_path"
    path: ".success"
    expected: true
  - type: "json_path"
    path: ".actions_taken"
    exists: true
```

## Common Use Cases

### Web Application Testing

```yaml
- name: "Test login flow"
  plugin: browser
  config:
    task: |
      1. Go to https://app.example.com/login
      2. Enter email: test@example.com
      3. Enter password: testpass123
      4. Click login button
      5. Verify you see the dashboard
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    timeout: "3m"
```

### Data Extraction

```yaml
- name: "Scrape pricing table"
  plugin: browser
  config:
    task: "Navigate to https://example.com/pricing and extract all plan names and prices"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    use_vision: true  # Better for visual elements
  save:
    - json_path: ".result"
      as: "pricing_data"
```

### Form Submission

```yaml
- name: "Fill contact form"
  plugin: browser
  config:
    task: |
      Go to https://example.com/contact
      Fill in:
      - Name: Test User
      - Email: test@example.com
      - Message: Automated test message
      Click submit
      Verify success message appears
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    headless: false  # Watch it work
```

## Multi-Step Workflows

```yaml
tests:
  - name: "Complete purchase flow"
    steps:
      - name: "Browse products"
        plugin: browser
        config:
          task: "Go to https://shop.example.com and find the first product"
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
        save:
          - json_path: ".result"
            as: "product_url"

      - name: "Add to cart"
        plugin: browser
        config:
          task: "Go to {{ product_url }} and click add to cart"
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"

      - name: "Checkout"
        plugin: browser
        config:
          task: "Navigate to cart and proceed to checkout"
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

## Best Practices

**Clear Instructions**: Be specific about what the browser should do
```yaml
# ❌ Vague
task: "Check the website"

# ✅ Specific
task: "Navigate to https://example.com/products, click on the first product, and extract its price"
```

**Appropriate Timeouts**: Complex tasks need more time
```yaml
# Simple navigation: 1m
# Form filling: 2-3m
# Multi-page workflows: 5m+
timeout: "3m"
```

**Headless vs Headful**:
- Use `headless: true` for CI/CD and faster execution
- Use `headless: false` for debugging and watching the browser

**Vision Mode**: Enable for visual elements (charts, images)
```yaml
use_vision: true  # Better accuracy for visual content, slower execution
```

**Restrict Domains**: Prevent navigation to unexpected sites
```yaml
allowed_domains:
  - "example.com"
  - "app.example.com"
```

## Troubleshooting

**Browser won't start**: Install Playwright browsers
```bash
playwright install chromium
```

**Timeout errors**: Increase timeout or simplify task
```yaml
timeout: "5m"  # For complex workflows
```

**Actions not working**: Add debug logging
```yaml
- name: "Debug browser actions"
  plugin: log
  config:
    message: "Actions taken: {{ action_count }}"
```

**Navigation fails**: Check allowed_domains
```yaml
allowed_domains:
  - "*.example.com"  # Allow all subdomains
```

## Running Examples

```bash
# Run browser tests
rocketship run -af examples/browser-testing/rocketship.yaml

# With specific env file
rocketship run -af examples/browser-testing/rocketship.yaml \
  --env-file .env
```
