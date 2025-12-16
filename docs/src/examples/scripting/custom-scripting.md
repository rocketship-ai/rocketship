# Custom Scripting - JavaScript Integration

Execute custom JavaScript code within your test workflows for complex data processing, validation, and business logic.

## Quick Start

```yaml
- name: "Process data"
  plugin: "script"
  config:
    language: "javascript"
    script: |
      // Access state from previous steps
      let userName = state.user_name;

      // Process and save data
      let processedName = userName.toUpperCase();
      save("processed_name", processedName);

      // Validate data
      assert(userName.length > 0, "User name must not be empty");
```

## Configuration Modes

### Inline Scripts
```yaml
plugin: "script"
config:
  language: "javascript"
  script: |
    // Your JavaScript code here
```

### External Files
```yaml
plugin: "script"
config:
  language: "javascript"
  file: "scripts/validate-and-process.js"
```

## Built-in Functions

### `save(key, value)`
Save data to test state for use in subsequent steps:

```javascript
// Save simple values
save("user_count", "42");
save("status", "active");

// Save complex data as JSON
const profile = { name: "John", age: 30 };
save("user_profile", JSON.stringify(profile));
```

### `assert(condition, message)`
Validate data and fail the test if conditions aren't met:

```javascript
// Basic assertions
assert(state.user_id, "User ID must be present");
assert(state.score > 0, "Score must be positive");

// Complex validations
assert(state.email.includes("@"), "Email must be valid");
```

## Accessing Data

### Configuration Variables
Access variables defined in the `vars` section:

```javascript
let apiUrl = vars.api_url;
let timeout = vars.timeout;
```

### Test State
Access data saved from previous HTTP or script steps:

```javascript
// From HTTP save operations
let userId = state.user_id;
let userName = state.user_name;

// Convert types (state values are strings)
let score = parseInt(state.user_score);
let price = parseFloat(state.price);
let isActive = state.active === "true";
```

## Complete Integration Example

```yaml
name: "HTTP ↔ Script Integration"

vars:
  api_url: "https://api.example.com"

tests:
  - name: "Data Processing Pipeline"
    steps:
      # 1. HTTP: Fetch data
      - name: "Get user data"
        plugin: "http"
        config:
          method: "GET"
          url: "{{ .vars.api_url }}/users/123"
        save:
          - json_path: ".id"
            as: "user_id"
          - json_path: ".name"
            as: "user_name"
          - json_path: ".age"
            as: "user_age"

      # 2. Script: Process data
      - name: "Process user data"
        plugin: "script"
        config:
          language: "javascript"
          script: |
            // Access HTTP response data
            let name = state.user_name;
            let age = parseInt(state.user_age);

            // Calculate derived values
            let category = age >= 18 ? "adult" : "minor";
            let nameUpper = name.toUpperCase();

            // Save for next steps
            save("user_category", category);
            save("display_name", nameUpper);

      # 3. HTTP: Use processed data
      - name: "Update user category"
        plugin: "http"
        config:
          method: "PATCH"
          url: "{{ .vars.api_url }}/users/{{ user_id }}"
          body: |
            {
              "category": "{{ user_category }}",
              "display_name": "{{ display_name }}"
            }
        assertions:
          - type: status_code
            expected: 200
```

## External JavaScript Files

For complex logic, use external files:

```javascript
// scripts/validate-and-process.js

// Validate required data
assert(state.animal_name, "Animal name is required");
assert(state.animal_species, "Animal species is required");

// Business logic
let animalCategory = "unknown";
const domesticAnimals = ["dog", "cat", "horse"];
const wildAnimals = ["lion", "tiger", "elephant"];

if (domesticAnimals.some(a => state.animal_name.toLowerCase().includes(a))) {
    animalCategory = "domestic";
} else if (wildAnimals.some(a => state.animal_name.toLowerCase().includes(a))) {
    animalCategory = "wild";
}

// Calculate scores
let score = state.animal_name.length + state.animal_species.length;
if (animalCategory === "wild") score += 10;

// Save results
save("animal_category", animalCategory);
save("animal_score", score.toString());
```

## Common Use Cases

### Data Transformation
```javascript
// Transform API response
let rawData = JSON.parse(state.api_response);
let transformed = rawData.map(item => ({
    id: item.identifier,
    name: item.display_name.toUpperCase(),
    active: item.status === "enabled"
}));
save("transformed_data", JSON.stringify(transformed));
```

### Complex Validation
```javascript
// Business rule validation
let order = JSON.parse(state.order_details);
assert(
    order.items.length > 0 && order.total > 0,
    "Order must have items and positive total"
);

// Conditional validation
if (order.total > 1000) {
    assert(order.approval_required, "High-value orders require approval");
}
```

### Dynamic Test Data
```javascript
// Generate test data
const suffix = Date.now();
save("test_email", `test-${suffix}@example.com`);
save("test_password", `Pass${suffix}!`);
save("test_user", `user_${suffix}`);
```

### Response Analysis
```javascript
// Performance categorization
let responseTime = parseInt(state.response_time_ms);
save("perf_category",
     responseTime < 100 ? "fast" :
     responseTime < 500 ? "medium" : "slow");

assert(responseTime < 2000, "Response must be under 2 seconds");
```

## Best Practices

### 1. Keep Scripts Focused
Use scripts for data processing, not HTTP operations:

```javascript
// ✅ Good: Data processing
let cleaned = state.raw_data.trim().toUpperCase();
save("clean_data", cleaned);

// ❌ Avoid: Use HTTP plugin instead
// Don't make HTTP requests from scripts
```

### 2. External Files for Complex Logic
```yaml
# Simple: inline
- plugin: "script"
  config:
    script: 'save("doubled", (parseInt(state.value) * 2).toString());'

# Complex: external file
- plugin: "script"
  config:
    file: "scripts/complex-analysis.js"
```

### 3. Clear Error Messages
```javascript
// ✅ Good: Descriptive
assert(state.user_id, "User ID required for profile operations");
assert(state.email.includes("@"), "Invalid email format");

// ❌ Poor: Vague
assert(state.user_id, "Missing data");
```

### 4. Type Conversions
State values are always strings:

```javascript
// Convert when needed
let count = parseInt(state.item_count);
let price = parseFloat(state.price);
let isActive = state.active === "true";

// Save with explicit conversion
save("total", (price * count).toString());
```

## Running the Example

```bash
rocketship run -af examples/custom-scripting/rocketship.yaml
```

See the [full example](https://github.com/rocketship-ai/rocketship/blob/main/examples/custom-scripting/rocketship.yaml) for comprehensive script integration patterns.
