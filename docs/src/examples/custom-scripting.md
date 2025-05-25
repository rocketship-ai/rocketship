# Custom Scripting - JavaScript Integration

This example demonstrates how to use custom scripting in Rocketship test suites. The script plugin allows you to execute custom JavaScript code within your test workflows, enabling complex data processing, validation, and business logic that goes beyond simple HTTP assertions.

## Key Features Demonstrated

- **Inline JavaScript**: Execute scripts directly in your YAML test files
- **External JavaScript Files**: Reference external `.js` files for complex logic
- **State Integration**: Access and modify test state between HTTP and script steps
- **Configuration Variables**: Access config variables from script code
- **Bidirectional Data Flow**: Pass data from HTTP → Script → HTTP seamlessly
- **Built-in Functions**: Use `save()` and `assert()` functions
- **Business Logic**: Implement complex data validation and transformation

## Script Plugin Configuration

The script plugin supports two execution modes:

### Inline Scripts

```yaml
- name: "Process data with inline JavaScript"
  plugin: "script"
  config:
    language: "javascript"
    script: |
      // Access config variables
      let apiUrl = vars.api_url;
      
      // Access state from previous steps
      let userName = state.user_name;
      
      // Process and save data
      let processedName = state.user_name.toUpperCase();
      save("processed_name", processedName);
      
      // Validate data
      assert(state.user_name, "User name must be present");
```

### External JavaScript Files

```yaml
- name: "Process data with external file"
  plugin: "script"
  config:
    language: "javascript"
    file: "examples/custom-scripting/validate-and-process.js"
```

## Complete Integration Example

The example demonstrates a complete HTTP ↔ Script integration workflow:

```yaml
name: "Custom Scripting Demo - HTTP↔Script State Integration"
vars:
  api_url: "https://tryme.rocketship.sh"
  max_retries: 3
  user_name: "test_user"

tests:
  - name: "Complete HTTP and Script State Integration"
    steps:
      # 1. HTTP: Create initial data
      - name: "HTTP Step 1 - Create Animal Data"
        plugin: "http"
        config:
          method: "POST"
          url: "{{ .vars.api_url }}/animals"
          body: |
            {
              "name": "African Elephant",
              "species": "Loxodonta africana",
              "habitat": "Savanna",
              "weight_kg": 6000,
              "conservation_status": "Endangered"
            }
        save:
          - json_path: ".id"
            as: "animal_id"
          - json_path: ".name"
            as: "animal_name"

      # 2. Script: Process HTTP data
      - name: "Script Step 1 - Initial Processing"
        plugin: "script"
        config:
          language: "javascript"
          script: |
            // Access config variables and HTTP data
            let apiUrl = vars.api_url;
            let userName = vars.user_name;
            let animalName = state.animal_name;
            let animalId = state.animal_id;
            
            // Process user and config data
            let processedUserName = vars.user_name.toUpperCase();
            let animalWeight = parseInt(state.animal_weight);
            let weightCategory = animalWeight > 1000 ? "large" : "medium";
            
            // Save processed results for next steps
            save("processed_user_name", processedUserName);
            save("weight_category", weightCategory);

      # 3. Script: External file processing
      - name: "Script Step 2 - External File Processing"
        plugin: "script"
        config:
          language: "javascript"
          file: "examples/custom-scripting/validate-and-process.js"

      # 4. HTTP: Use script data
      - name: "HTTP Step 2 - Create Assessment"
        plugin: "http"
        config:
          method: "POST"
          url: "{{ .vars.api_url }}/animals/assessments"
          headers:
            X-Processed-By: "{{ processed_user_name }}"
          body: |
            {
              "animal_id": "{{ animal_id }}",
              "category": "{{ animal_category }}",
              "score": {{ animal_score }},
              "weight_category": "{{ weight_category }}"
            }
```

## Built-in Functions

### `save(key, value)`

Save data to the test state for use in subsequent steps:

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
assert(Array.isArray(JSON.parse(state.items)), "Items must be an array");
```

## State and Variable Access

### Configuration Variables

Access variables defined in the `vars` section:

```javascript
// Simple variables
let apiUrl = vars.api_url;
let timeout = vars.timeout;

// Nested variables
let authToken = vars.auth.token;
let dbHost = vars.database.host;
```

### Test State

Access data saved from previous HTTP or script steps:

```javascript
// Data from HTTP responses
let userId = state.user_id;        // From save: json_path: ".id"
let userName = state.user_name;    // From save: json_path: ".name"

// Data from previous scripts
let processed = state.processed_flag;  // From save("processed_flag", "true")
let score = parseInt(state.user_score); // Convert saved strings to numbers
```

## External JavaScript Files

For complex logic, use external JavaScript files:

```javascript
// validate-and-process.js

// Validate required data
if (!state.animal_name || !state.animal_species) {
    assert(false, "Missing required animal data");
}

// Complex business logic
let animalCategory = "unknown";
const domesticAnimals = ["dog", "cat", "horse"];
const wildAnimals = ["lion", "tiger", "elephant", "bear"];

if (domesticAnimals.some(animal => state.animal_name.toLowerCase().includes(animal))) {
    animalCategory = "domestic";
} else if (wildAnimals.some(animal => state.animal_name.toLowerCase().includes(animal))) {
    animalCategory = "wild";
} else {
    animalCategory = "exotic";
}

// Calculate scores
let animalScore = state.animal_name.length + state.animal_species.length;
if (animalCategory === "wild") animalScore += 10;

// Generate recommendations
let recommendations = [];
if (animalCategory === "domestic") {
    recommendations.push("suitable_for_families");
} else if (animalCategory === "wild") {
    recommendations.push("observe_from_distance");
}

// Save results for HTTP steps
save("animal_category", animalCategory);
save("animal_score", animalScore.toString());
save("recommendations_count", recommendations.length.toString());

// Save individual recommendations for template access
recommendations.forEach((rec, index) => {
    save(`recommendation_${index + 1}`, rec);
});
```

## Running the Example

```bash
# Run the complete custom scripting example
rocketship run -af examples/custom-scripting/rocketship.yaml
```

## Understanding the Data Flow

The example demonstrates a complete data processing pipeline:

1. **HTTP Step 1**: Create animal data via API, save ID and attributes
2. **Script Step 1**: Process config variables and HTTP data, create derived values
3. **Script Step 2**: External file performs complex business logic and categorization
4. **HTTP Step 2**: Use script-processed data to create a comprehensive assessment
5. **Script Step 3**: Final validation ensures all data flows worked correctly

Each step builds on the previous ones, showing:

- **HTTP → Script**: Pass API response data to JavaScript for processing
- **Script → Script**: Share state between inline and external scripts
- **Script → HTTP**: Use processed data in API requests
- **Config Integration**: Mix configuration variables with runtime processing

## Use Cases for Custom Scripting

### Data Transformation

```javascript
// Transform API responses
let rawData = JSON.parse(state.api_response);
let transformedData = rawData.map(item => ({
    id: item.identifier,
    name: item.display_name.toUpperCase(),
    active: item.status === "enabled"
}));
save("transformed_data", JSON.stringify(transformedData));
```

### Complex Validations

```javascript
// Business rule validation
let orderData = JSON.parse(state.order_details);
let isValidOrder = orderData.items.length > 0 && 
                   orderData.total > 0 && 
                   orderData.customer_id;

assert(isValidOrder, "Order must have items, positive total, and customer ID");

// Multi-step validation logic
if (orderData.total > 1000) {
    assert(orderData.approval_required, "High-value orders require approval");
}
```

### Dynamic Test Data Generation

```javascript
// Generate test data based on conditions
let testUsers = [];
for (let i = 0; i < vars.user_count; i++) {
    testUsers.push({
        id: `user_${i}`,
        email: `test${i}@example.com`,
        role: i % 2 === 0 ? "admin" : "user"
    });
}
save("test_users", JSON.stringify(testUsers));
```

### API Response Analysis

```javascript
// Analyze API performance and content
let responseTime = parseInt(state.response_time_ms);
let responseSize = state.response_body.length;

save("performance_category", 
     responseTime < 100 ? "fast" : 
     responseTime < 500 ? "medium" : "slow");

assert(responseTime < 2000, "Response time must be under 2 seconds");
assert(responseSize > 0, "Response must not be empty");
```

## Best Practices

### 1. Keep Scripts Focused

Use scripts for data processing and validation, not for replacing HTTP operations:

```javascript
// Good: Data processing
let processedData = state.raw_data.toUpperCase().trim();
save("clean_data", processedData);

// Avoid: HTTP operations (use http plugin instead)
// Don't try to make HTTP requests from scripts
```

### 2. Use External Files for Complex Logic

Move complex business logic to external files:

```yaml
# Simple processing: inline
- plugin: "script"
  config:
    script: 'save("doubled", (parseInt(state.value) * 2).toString());'

# Complex processing: external file
- plugin: "script"
  config:
    file: "scripts/complex-analysis.js"
```

### 3. Clear Error Messages

Provide helpful assertion messages:

```javascript
// Good: Descriptive messages
assert(state.user_id, "User ID is required for profile operations");
assert(state.email.includes("@"), "Email format validation failed");

// Poor: Vague messages
assert(state.user_id, "Missing data");
```

### 4. Type Conversions

Remember that saved state is always strings:

```javascript
// Convert types when needed
let count = parseInt(state.item_count);
let price = parseFloat(state.price);
let isActive = state.active === "true";

// Save with explicit string conversion
save("calculated_total", (price * count).toString());
```

The custom scripting plugin enables powerful data processing and validation workflows while maintaining the simplicity and clarity of Rocketship's declarative test approach.