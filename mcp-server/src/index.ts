#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";

// Knowledge base for Rocketship patterns and examples
const ROCKETSHIP_KNOWLEDGE = {
  api_testing: {
    description:
      "API testing patterns focus on testing HTTP endpoints with proper authentication, status codes, and response validation",
    examples: [
      {
        name: "Basic API Health Check",
        code: `tests:
  - name: "Health Check"
    steps:
      - name: "Check API health"
        plugin: http
        config:
          method: GET
          url: "{{.vars.base_url}}/health"
          timeout: 5s
        assertions:
          - type: status_code
            expected: 200
          - type: response_json
            path: "$.status"
            expected: "healthy"`,
      },
      {
        name: "Authenticated API Request with Response Validation",
        code: `tests:
  - name: "Get User Profile"
    steps:
      - name: "Fetch user data"
        plugin: http
        config:
          method: GET
          url: "{{.vars.base_url}}/api/users/{{.vars.user_id}}"
          headers:
            Authorization: "Bearer {{.vars.auth_token}}"
            Content-Type: "application/json"
        assertions:
          - type: status_code
            expected: 200
          - type: response_json
            path: "$.data.email"
            expected: "{{.vars.expected_email}}"
        save:
          - name: user_name
            from: response_json
            path: "$.data.name"`,
      },
    ],
    best_practices: [
      "Always use variables for URLs and authentication tokens",
      "Include timeout configurations for all HTTP requests",
      "Use response_json assertions for validating API responses",
      "Save important values for use in subsequent steps",
    ],
  },

  step_chaining: {
    description:
      "Step chaining allows you to use data from one step in subsequent steps, essential for testing complex workflows",
    examples: [
      {
        name: "Create and Verify Resource",
        code: `tests:
  - name: "Create and Verify User"
    steps:
      - name: "Create user"
        plugin: http
        config:
          method: POST
          url: "{{.vars.base_url}}/api/users"
          body: |
            {
              "email": "test-{{.run.timestamp}}@example.com",
              "name": "Test User"
            }
        assertions:
          - type: status_code
            expected: 201
        save:
          - name: created_user_id
            from: response_json
            path: "$.data.id"
          - name: created_email
            from: response_json
            path: "$.data.email"
            
      - name: "Verify user exists"
        plugin: http
        config:
          method: GET
          url: "{{.vars.base_url}}/api/users/{{.steps.create_user.created_user_id}}"
        assertions:
          - type: status_code
            expected: 200
          - type: response_json
            path: "$.data.email"
            expected: "{{.steps.create_user.created_email}}"`,
      },
    ],
    best_practices: [
      "Use meaningful names for saved variables",
      "Reference saved variables with {{.steps.<step_name>.<variable_name>}}",
      "Always validate that created resources can be retrieved",
    ],
  },

  assertions: {
    description:
      "Assertions validate that your application behaves correctly. Rocketship supports multiple assertion types",
    examples: [
      {
        name: "Common Assertion Types",
        code: `assertions:
  # Status code validation
  - type: status_code
    expected: 200
    
  # JSON response validation
  - type: response_json
    path: "$.data.status"
    expected: "active"
    
  # Response contains text
  - type: response_body
    contains: "success"
    
  # Response time check
  - type: response_time
    max_ms: 2000
    
  # Header validation
  - type: response_header
    name: "Content-Type"
    expected: "application/json"
    
  # JSON array length
  - type: response_json
    path: "$.data.items"
    length: 5
    
  # JSON exists check
  - type: response_json_exists
    path: "$.data.id"
    
  # JSON type validation
  - type: response_json
    path: "$.data.count"
    type: "number"`,
      },
    ],
    best_practices: [
      "Use specific assertions rather than just checking status codes",
      "Validate both structure and content of responses",
      "Include response time assertions for performance testing",
      "Use JSONPath for precise JSON validation",
    ],
  },

  customer_journeys: {
    description:
      "E2E customer journey tests validate complete user workflows across multiple services and steps",
    examples: [
      {
        name: "E-Commerce Purchase Journey",
        code: `name: "E-Commerce Customer Purchase Journey"
description: "Test complete purchase flow from product search to order confirmation"
version: "v1.0.0"

tests:
  - name: "Complete Purchase Flow"
    steps:
      # Step 1: Search for product
      - name: "Search for product"
        plugin: http
        config:
          method: GET
          url: "{{.vars.base_url}}/api/products/search?q=laptop"
        assertions:
          - type: status_code
            expected: 200
          - type: response_json
            path: "$.results"
            min_length: 1
        save:
          - name: product_id
            from: response_json
            path: "$.results[0].id"
          - name: product_price
            from: response_json
            path: "$.results[0].price"
            
      # Step 2: Add to cart
      - name: "Add product to cart"
        plugin: http
        config:
          method: POST
          url: "{{.vars.base_url}}/api/cart/items"
          headers:
            Authorization: "Bearer {{.vars.customer_token}}"
          body: |
            {
              "product_id": "{{.steps.search_for_product.product_id}}",
              "quantity": 1
            }
        assertions:
          - type: status_code
            expected: 201
        save:
          - name: cart_id
            from: response_json
            path: "$.cart_id"
            
      # Step 3: Checkout
      - name: "Create checkout session"
        plugin: http
        config:
          method: POST
          url: "{{.vars.base_url}}/api/checkout"
          headers:
            Authorization: "Bearer {{.vars.customer_token}}"
          body: |
            {
              "cart_id": "{{.steps.add_product_to_cart.cart_id}}",
              "payment_method": "test_card"
            }
        assertions:
          - type: status_code
            expected: 200
        save:
          - name: order_id
            from: response_json
            path: "$.order_id"
            
      # Step 4: Verify order
      - name: "Verify order created"
        plugin: http
        config:
          method: GET
          url: "{{.vars.base_url}}/api/orders/{{.steps.create_checkout_session.order_id}}"
          headers:
            Authorization: "Bearer {{.vars.customer_token}}"
        assertions:
          - type: status_code
            expected: 200
          - type: response_json
            path: "$.status"
            expected: "confirmed"
          - type: response_json
            path: "$.total"
            expected: "{{.steps.search_for_product.product_price}}"`,
      },
    ],
    best_practices: [
      "Test complete user journeys, not just individual endpoints",
      "Validate data consistency across multiple steps",
      "Use realistic test data that mimics actual user behavior",
      "Include cleanup steps to reset test data",
      "Consider edge cases and error scenarios in journeys",
    ],
  },

  environments: {
    description:
      "Environment configuration allows tests to run across different stages with appropriate settings",
    examples: [
      {
        name: "Multi-Environment Configuration",
        code: `# In your main test file:
version: "v1.0.0"
name: "Multi-Environment Tests"

vars:
  base_url: "{{.env.BASE_URL}}"
  api_key: "{{.env.API_KEY}}"
  db_connection: "{{.env.DATABASE_URL}}"

# In staging-vars.yaml:
BASE_URL: "https://api-staging.example.com"
API_KEY: "staging-key-12345"
DATABASE_URL: "postgresql://user:pass@staging-db:5432/myapp"

# In production-vars.yaml:
BASE_URL: "https://api.example.com"
API_KEY: "{{.secrets.PROD_API_KEY}}"  # Reference from secrets
DATABASE_URL: "{{.secrets.PROD_DB_URL}}"`,
      },
    ],
    best_practices: [
      "Never hardcode environment-specific values",
      "Use separate variable files for each environment",
      "Store sensitive data in secure secret management",
      "Include environment validation in your tests",
    ],
  },

  plugins: {
    description:
      "Rocketship plugins extend testing capabilities beyond HTTP requests",
    examples: [
      {
        name: "SQL Plugin for Database Testing",
        code: `steps:
  - name: "Verify database state"
    plugin: sql
    config:
      driver: postgres
      dsn: "{{.vars.db_connection}}"
      commands:
        - "SELECT COUNT(*) as user_count FROM users WHERE status = 'active';"
    assertions:
      - type: sql_result
        query_index: 0
        row_index: 0
        column: "user_count"
        expected: 5`,
      },
      {
        name: "Browser Plugin for UI Testing",
        code: `steps:
  - name: "Test login flow"
    plugin: browser
    config:
      url: "{{.vars.app_url}}/login"
      actions:
        - type: fill
          selector: "#email"
          value: "{{.vars.test_email}}"
        - type: fill
          selector: "#password"
          value: "{{.vars.test_password}}"
        - type: click
          selector: "#login-button"
        - type: wait_for_navigation
    assertions:
      - type: url
        contains: "/dashboard"
      - type: element_visible
        selector: ".welcome-message"`,
      },
      {
        name: "Script Plugin for Custom Logic",
        code: `steps:
  - name: "Custom validation"
    plugin: script
    config:
      language: javascript
      inline: |
        // Access previous step data
        const responseData = context.steps.previous_step.response;
        
        // Custom validation logic
        if (responseData.items.length < 10) {
          throw new Error("Expected at least 10 items");
        }
        
        // Set variables for next steps
        context.setVariable("item_count", responseData.items.length);
        context.setVariable("first_item_id", responseData.items[0].id);`,
      },
    ],
    best_practices: [
      "Choose the right plugin for your testing needs",
      "Configure plugins with appropriate timeouts and retry logic",
      "Use variables for plugin configuration values",
      "Test plugin functionality separately before complex workflows",
      "Check plugin-specific documentation for advanced features"
    ]
  },
};

// Tool descriptions that emphasize the assistant nature
const TOOL_DESCRIPTIONS = {
  get_rocketship_examples: `Provides examples and best practices for specific Rocketship features. This helps you understand how to write tests properly.

YOU (the coding agent) will create the actual test files based on these examples.`,

  suggest_test_structure: `Suggests a test structure with helpful comments and TODOs for the given scenario. Returns a template that you can fill in.

YOU (the coding agent) will implement the actual test logic based on this structure.`,

  get_assertion_patterns: `Provides assertion examples for different testing scenarios. Shows you what assertions are available and when to use them.

YOU (the coding agent) will choose and implement the appropriate assertions.`,

  get_plugin_config: `Provides configuration examples for Rocketship plugins (http, sql, browser, etc). Shows available options and usage patterns.

YOU (the coding agent) will configure the plugins based on your specific needs.`,

  validate_and_suggest: `Reviews your Rocketship YAML and suggests improvements. Helps ensure your tests follow best practices.

YOU (the coding agent) will decide which suggestions to implement.`,

  get_cli_commands: `Provides Rocketship CLI command examples and usage patterns. Helps you understand how to run and manage tests.

YOU (the coding agent) will execute these commands as needed.`,
};

export class RocketshipMCPServer {
  private server: Server;

  constructor() {
    this.server = new Server(
      {
        name: "rocketship-mcp",
        version: "0.2.0",
      },
      {
        capabilities: {
          tools: {},
        },
      }
    );

    this.setupHandlers();
  }

  private setupHandlers() {
    this.server.setRequestHandler(ListToolsRequestSchema, async () => {
      return {
        tools: [
          {
            name: "get_rocketship_examples",
            description: TOOL_DESCRIPTIONS.get_rocketship_examples,
            inputSchema: {
              type: "object",
              properties: {
                feature: {
                  type: "string",
                  enum: [
                    "api_testing",
                    "step_chaining",
                    "assertions",
                    "plugins",
                    "environments",
                    "customer_journeys",
                  ],
                  description: "The Rocketship feature to get examples for",
                },
                context: {
                  type: "string",
                  description:
                    "Optional context about what you are trying to test",
                },
              },
              required: ["feature"],
            },
          },
          {
            name: "suggest_test_structure",
            description: TOOL_DESCRIPTIONS.suggest_test_structure,
            inputSchema: {
              type: "object",
              properties: {
                test_name: {
                  type: "string",
                  description: "Name of the test suite",
                },
                test_type: {
                  type: "string",
                  enum: ["api", "browser", "sql", "integration", "e2e"],
                  description: "Type of test to create",
                },
                description: {
                  type: "string",
                  description: "Description of what needs to be tested",
                },
                customer_journey: {
                  type: "string",
                  description:
                    "Description of the customer journey being tested",
                },
              },
              required: ["test_name", "test_type", "description"],
            },
          },
          {
            name: "get_assertion_patterns",
            description: TOOL_DESCRIPTIONS.get_assertion_patterns,
            inputSchema: {
              type: "object",
              properties: {
                response_type: {
                  type: "string",
                  enum: [
                    "json",
                    "xml",
                    "text",
                    "status",
                    "headers",
                    "sql",
                    "browser",
                  ],
                  description: "Type of response to validate",
                },
                test_scenario: {
                  type: "string",
                  description: "What you are trying to validate",
                },
              },
              required: ["response_type", "test_scenario"],
            },
          },
          {
            name: "get_plugin_config",
            description: TOOL_DESCRIPTIONS.get_plugin_config,
            inputSchema: {
              type: "object",
              properties: {
                plugin: {
                  type: "string",
                  enum: [
                    "http",
                    "sql",
                    "browser",
                    "agent",
                    "supabase",
                    "delay",
                    "script",
                    "log",
                  ],
                  description: "The Rocketship plugin to get configuration for",
                },
                use_case: {
                  type: "string",
                  description: "What you want to use the plugin for",
                },
              },
              required: ["plugin", "use_case"],
            },
          },
          {
            name: "validate_and_suggest",
            description: TOOL_DESCRIPTIONS.validate_and_suggest,
            inputSchema: {
              type: "object",
              properties: {
                yaml_content: {
                  type: "string",
                  description: "The Rocketship YAML content to validate",
                },
                improvement_focus: {
                  type: "string",
                  enum: [
                    "performance",
                    "assertions",
                    "structure",
                    "coverage",
                    "best_practices",
                  ],
                  description: "Area to focus improvements on",
                },
              },
              required: ["yaml_content"],
            },
          },
          {
            name: "get_cli_commands",
            description: TOOL_DESCRIPTIONS.get_cli_commands,
            inputSchema: {
              type: "object",
              properties: {
                command: {
                  type: "string",
                  enum: ["run", "validate", "start", "stop", "general"],
                  description: "The CLI command to get help for",
                },
                context: {
                  type: "string",
                  description: "Optional context about what you want to do",
                },
              },
              required: ["command"],
            },
          },
        ],
      };
    });

    this.server.setRequestHandler(CallToolRequestSchema, async (request) => {
      const { name, arguments: args } = request.params;

      try {
        switch (name) {
          case "get_rocketship_examples":
            return this.handleGetExamples(args);
          case "suggest_test_structure":
            return this.handleSuggestStructure(args);
          case "get_assertion_patterns":
            return this.handleGetAssertions(args);
          case "get_plugin_config":
            return this.handleGetPluginConfig(args);
          case "validate_and_suggest":
            return this.handleValidateAndSuggest(args);
          case "get_cli_commands":
            return this.handleGetCLICommands(args);
          default:
            throw new Error(`Unknown tool: ${name}`);
        }
      } catch (error) {
        return {
          content: [
            {
              type: "text",
              text: `Error: ${
                error instanceof Error ? error.message : String(error)
              }`,
            },
          ],
        };
      }
    });
  }

  private async handleGetExamples(args: any) {
    const { feature, context } = args;
    const knowledge =
      ROCKETSHIP_KNOWLEDGE[feature as keyof typeof ROCKETSHIP_KNOWLEDGE];

    if (!knowledge) {
      return {
        content: [
          {
            type: "text",
            text: `Unknown feature: ${feature}`,
          },
        ],
      };
    }

    let response = `# Rocketship ${feature
      .replace("_", " ")
      .replace(/\b\w/g, (l: string) => l.toUpperCase())} Examples\n\n`;
    response += `${knowledge.description}\n\n`;

    if (context) {
      response += `Context: ${context}\n\n`;
    }

    response += `## Examples:\n\n`;
    for (const example of knowledge.examples) {
      response += `### ${example.name}\n\n`;
      response += "```yaml\n" + example.code + "\n```\n\n";
    }

    if (knowledge.best_practices) {
      response += `## Best Practices:\n\n`;
      for (const practice of knowledge.best_practices) {
        response += `- ${practice}\n`;
      }
    }

    response += `\n## Next Steps:\n`;
    response += `1. Create your test file based on these examples\n`;
    response += `2. Customize the examples to match your specific API/application\n`;
    response += `3. Run 'rocketship validate <your-file>.yaml' to check syntax\n`;
    response += `4. Execute with 'rocketship run -af <your-file>.yaml'\n`;

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private async handleSuggestStructure(args: any) {
    const { test_name, test_type, description, customer_journey } = args;

    let structure = `# Rocketship Test Structure for: ${test_name}\n\n`;
    structure += `# Description: ${description}\n`;
    if (customer_journey) {
      structure += `# Customer Journey: ${customer_journey}\n`;
    }
    structure += `\n## Suggested Structure:\n\n`;
    structure += "```yaml\n";
    structure += `name: "${test_name}"\n`;
    structure += `description: "${description}"\n`;
    structure += `version: "v1.0.0"\n\n`;

    structure += `# TODO: Define your environment variables\n`;
    structure += `vars:\n`;
    structure += `  base_url: "{{.env.BASE_URL}}"\n`;
    structure += `  auth_token: "{{.env.AUTH_TOKEN}}"\n`;
    structure += `  # Add more variables as needed\n\n`;

    structure += `tests:\n`;

    if (test_type === "e2e" || customer_journey) {
      structure += this.generateE2EStructure(customer_journey || description);
    } else if (test_type === "api") {
      structure += this.generateAPIStructure();
    } else if (test_type === "integration") {
      structure += this.generateIntegrationStructure();
    } else {
      structure += this.generateGenericStructure(test_type);
    }

    structure += "```\n\n";
    structure += `## Implementation Checklist:\n\n`;
    structure += `- [ ] Fill in all TODO sections with actual values\n`;
    structure += `- [ ] Add appropriate assertions for each step\n`;
    structure += `- [ ] Include error scenarios and edge cases\n`;
    structure += `- [ ] Add cleanup steps if needed\n`;
    structure += `- [ ] Create environment-specific variable files\n`;
    structure += `- [ ] Validate with: rocketship validate <filename>.yaml\n`;
    structure += `- [ ] Test locally before committing\n`;

    return {
      content: [
        {
          type: "text",
          text: structure,
        },
      ],
    };
  }

  private generateE2EStructure(journey: string): string {
    return `  - name: "${journey} - End to End Flow"
    steps:
      # TODO: Step 1 - Initial setup/authentication
      - name: "Setup and authenticate"
        plugin: http
        config:
          method: POST
          url: "{{.vars.base_url}}/auth/login"
          # TODO: Add authentication body
        assertions:
          - type: status_code
            expected: 200
        save:
          - name: auth_token
            from: response_json
            path: "$.token"
      
      # TODO: Step 2 - Main customer action
      - name: "Perform main action"
        plugin: http
        config:
          method: # TODO: Add method
          url: # TODO: Add URL
          headers:
            Authorization: "Bearer {{.steps.setup_and_authenticate.auth_token}}"
        assertions:
          # TODO: Add assertions
        save:
          # TODO: Save important data for next steps
      
      # TODO: Step 3 - Verify results
      - name: "Verify action completed"
        plugin: http
        config:
          # TODO: Add verification request
        assertions:
          # TODO: Verify expected state
      
      # TODO: Add more steps for complete journey
`;
  }

  private generateAPIStructure(): string {
    return `  - name: "API Endpoint Tests"
    steps:
      # TODO: Health check
      - name: "Health check"
        plugin: http
        config:
          method: GET
          url: "{{.vars.base_url}}/health"
        assertions:
          - type: status_code
            expected: 200
      
      # TODO: Main API endpoint test
      - name: "Test main endpoint"
        plugin: http
        config:
          method: # TODO: GET/POST/PUT/DELETE
          url: # TODO: Add endpoint URL
          headers:
            Authorization: "Bearer {{.vars.auth_token}}"
            Content-Type: "application/json"
          # TODO: Add body if needed
        assertions:
          - type: status_code
            expected: # TODO: Expected status
          - type: response_json
            path: # TODO: JSONPath expression
            expected: # TODO: Expected value
        save:
          # TODO: Save response data if needed
`;
  }

  private generateIntegrationStructure(): string {
    return `  - name: "Integration Test Flow"
    steps:
      # TODO: Setup test data
      - name: "Setup test data"
        plugin: # TODO: sql/http
        config:
          # TODO: Setup configuration
        
      # TODO: Execute main flow
      - name: "Execute integration flow"
        plugin: http
        config:
          # TODO: Add request details
        assertions:
          # TODO: Add assertions
        save:
          # TODO: Save data for verification
      
      # TODO: Verify side effects
      - name: "Verify database state"
        plugin: sql
        config:
          driver: postgres
          dsn: "{{.vars.db_connection}}"
          commands:
            - # TODO: Add SQL query
        assertions:
          # TODO: Verify database state
      
      # TODO: Cleanup
      - name: "Cleanup test data"
        plugin: # TODO: sql/http
        config:
          # TODO: Cleanup configuration
`;
  }

  private generateGenericStructure(test_type: string): string {
    return `  - name: "${test_type} Test"
    steps:
      # TODO: Implement your ${test_type} test steps
      - name: "First step"
        plugin: ${
          test_type === "sql"
            ? "sql"
            : test_type === "browser"
            ? "browser"
            : "http"
        }
        config:
          # TODO: Add configuration
        assertions:
          # TODO: Add assertions
`;
  }

  private async handleGetAssertions(args: any) {
    const { response_type, test_scenario } = args;

    let response = `# Assertion Patterns for ${response_type.toUpperCase()} Testing\n\n`;
    response += `Scenario: ${test_scenario}\n\n`;

    const assertionExamples = this.getAssertionExamples(
      response_type,
      test_scenario
    );

    response += `## Available Assertions:\n\n`;
    response += "```yaml\n";
    response += assertionExamples;
    response += "```\n\n";

    response += `## Tips:\n`;
    response += `- Use multiple assertions to thoroughly validate responses\n`;
    response += `- JSONPath expressions start with $ for root\n`;
    response += `- Arrays can be accessed with [index] or [*] for all\n`;
    response += `- Use exists assertions before value assertions\n`;
    response += `- Consider both positive and negative test cases\n`;

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private getAssertionExamples(type: string, scenario: string): string {
    const examples: Record<string, string> = {
      json: `assertions:
  # Check status code first
  - type: status_code
    expected: 200
    
  # Validate specific JSON field
  - type: response_json
    path: "$.data.id"
    expected: "12345"
    
  # Check if field exists
  - type: response_json_exists
    path: "$.data.user"
    
  # Validate array length
  - type: response_json
    path: "$.data.items"
    length: 10
    
  # Check array contains value
  - type: response_json
    path: "$.data.tags"
    contains: "important"
    
  # Validate nested object
  - type: response_json
    path: "$.data.user.email"
    matches: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
    
  # Check field type
  - type: response_json
    path: "$.data.count"
    type: "number"
    
  # Complex validation with min/max
  - type: response_json
    path: "$.data.items"
    min_length: 1
    max_length: 100`,

      status: `assertions:
  # Basic status check
  - type: status_code
    expected: 200
    
  # Status code range
  - type: status_code
    min: 200
    max: 299
    
  # Multiple acceptable codes
  - type: status_code
    one_of: [200, 201, 202]
    
  # Not found check
  - type: status_code
    expected: 404
    
  # Server error check
  - type: status_code
    min: 500
    max: 599`,

      headers: `assertions:
  # Check specific header
  - type: response_header
    name: "Content-Type"
    expected: "application/json"
    
  # Header contains value
  - type: response_header
    name: "Cache-Control"
    contains: "no-cache"
    
  # Header exists
  - type: response_header_exists
    name: "X-Request-ID"
    
  # Multiple headers
  - type: response_header
    name: "X-Rate-Limit-Remaining"
    min: 0
    max: 1000`,

      sql: `assertions:
  # Row count check
  - type: sql_result
    query_index: 0
    row_count: 5
    
  # Specific value check
  - type: sql_result
    query_index: 0
    row_index: 0
    column: "user_count"
    expected: 10
    
  # Column exists
  - type: sql_result
    query_index: 0
    column_exists: "email"
    
  # No results check
  - type: sql_result
    query_index: 0
    row_count: 0`,

      browser: `assertions:
  # URL check
  - type: url
    expected: "https://example.com/dashboard"
    
  # URL contains
  - type: url
    contains: "/success"
    
  # Element visible
  - type: element_visible
    selector: "#welcome-message"
    
  # Element text
  - type: element_text
    selector: ".user-name"
    expected: "John Doe"
    
  # Element count
  - type: element_count
    selector: ".list-item"
    expected: 5`,

      text: `assertions:
  # Response contains text
  - type: response_body
    contains: "Success"
    
  # Response matches regex
  - type: response_body
    matches: "Order #[0-9]+ confirmed"
    
  # Response does not contain
  - type: response_body
    not_contains: "Error"
    
  # Exact match
  - type: response_body
    expected: "OK"`,

      xml: `assertions:
  # XPath validation
  - type: response_xml
    xpath: "//user/email"
    expected: "test@example.com"
    
  # XML element exists
  - type: response_xml_exists
    xpath: "//response/data"
    
  # Count elements
  - type: response_xml
    xpath: "//items/item"
    count: 5`,
    };

    return examples[type] || examples.json;
  }

  private async handleGetPluginConfig(args: any) {
    const { plugin, use_case } = args;

    const pluginConfigs: Record<string, any> = {
      http: {
        description: "HTTP plugin for API testing",
        basic: `plugin: http
config:
  method: GET
  url: "{{.vars.base_url}}/api/endpoint"
  headers:
    Authorization: "Bearer {{.vars.token}}"
    Content-Type: "application/json"
  timeout: 30s`,

        advanced: `plugin: http
config:
  method: POST
  url: "{{.vars.base_url}}/api/users"
  headers:
    Authorization: "Bearer {{.vars.token}}"
    Content-Type: "application/json"
    X-Request-ID: "{{.run.uuid}}"
  body: |
    {
      "name": "Test User",
      "email": "test-{{.run.timestamp}}@example.com",
      "metadata": {
        "source": "automated_test",
        "timestamp": "{{.run.timestamp}}"
      }
    }
  timeout: 30s
  retry:
    count: 3
    delay: 2s
    exponential: true`,

        features: [
          "Supports all HTTP methods (GET, POST, PUT, DELETE, PATCH)",
          "JSON, XML, and form data body types",
          "Custom headers and authentication",
          "Retry logic with exponential backoff",
          "Request/response logging",
          "Proxy support",
        ],
      },

      sql: {
        description: "SQL plugin for database testing",
        basic: `plugin: sql
config:
  driver: postgres  # postgres, mysql, sqlite
  dsn: "{{.vars.database_url}}"
  commands:
    - "SELECT COUNT(*) as total FROM users WHERE active = true;"`,

        advanced: `plugin: sql
config:
  driver: postgres
  dsn: "{{.vars.database_url}}"
  transaction: true  # Run all commands in a transaction
  commands:
    - "INSERT INTO audit_log (action, user_id, timestamp) VALUES ('test_run', '{{.vars.test_user_id}}', NOW());"
    - "SELECT * FROM audit_log WHERE user_id = '{{.vars.test_user_id}}' ORDER BY timestamp DESC LIMIT 1;"
    - "UPDATE test_counters SET value = value + 1 WHERE name = 'test_runs';"
  timeout: 10s`,

        features: [
          "Supports PostgreSQL, MySQL, SQLite",
          "Transaction support",
          "Multiple command execution",
          "Parameterized queries",
          "Result set validation",
        ],
      },

      browser: {
        description: "Browser plugin for UI testing",
        basic: `plugin: browser
config:
  url: "{{.vars.app_url}}/login"
  actions:
    - type: fill
      selector: "#username"
      value: "{{.vars.test_user}}"
    - type: fill
      selector: "#password"
      value: "{{.vars.test_password}}"
    - type: click
      selector: "#login-button"
    - type: wait_for_navigation`,

        advanced: `plugin: browser
config:
  url: "{{.vars.app_url}}"
  viewport:
    width: 1920
    height: 1080
  actions:
    - type: wait_for_selector
      selector: ".cookie-banner"
      timeout: 5s
    - type: click
      selector: ".accept-cookies"
    - type: fill
      selector: "input[name='search']"
      value: "{{.vars.search_term}}"
    - type: press
      key: "Enter"
    - type: wait_for_selector
      selector: ".search-results"
    - type: screenshot
      path: "search-results.png"
    - type: evaluate
      script: |
        return document.querySelectorAll('.result-item').length;
      save_as: "result_count"`,

        features: [
          "Headless browser automation",
          "Form interaction and navigation",
          "Screenshot capture",
          "JavaScript execution",
          "Mobile viewport testing",
          "Cookie and localStorage management",
        ],
      },

      script: {
        description: "Script plugin for custom logic",
        basic: `plugin: script
config:
  language: javascript
  inline: |
    // Access context and previous steps
    const previousResponse = context.steps.previous_step.response;
    
    // Perform custom validation
    if (previousResponse.total < 100) {
      throw new Error("Insufficient data");
    }
    
    // Set variables for next steps
    context.setVariable("processed_count", previousResponse.total);`,

        advanced: `plugin: script
config:
  language: javascript
  file: "./scripts/custom-validation.js"
  env:
    - name: API_KEY
      value: "{{.vars.api_key}}"
    - name: ENVIRONMENT
      value: "{{.vars.environment}}"`,

        features: [
          "JavaScript execution",
          "Access to test context",
          "Variable manipulation",
          "External script files",
          "Environment variables",
          "Complex data transformations",
        ],
      },

      delay: {
        description: "Delay plugin for timing control",
        basic: `plugin: delay
config:
  duration: 5s`,

        advanced: `plugin: delay
config:
  duration: "{{.vars.wait_time}}"  # Dynamic delay
  jitter: 2s  # Random jitter ±2 seconds`,

        features: [
          "Fixed delays",
          "Variable-based delays",
          "Random jitter",
          "Useful for rate limiting",
          "Simulating real-world timing",
        ],
      },

      supabase: {
        description: "Supabase plugin for database operations",
        basic: `plugin: supabase
config:
  url: "{{.vars.supabase_url}}"
  key: "{{.vars.supabase_anon_key}}"
  operation: "select"
  table: "users"
  filters:
    - column: "email"
      operator: "eq"
      value: "test@example.com"`,

        advanced: `plugin: supabase
config:
  url: "{{.vars.supabase_url}}"
  key: "{{.vars.supabase_service_key}}"
  operation: "insert"
  table: "orders"
  data:
    user_id: "{{.steps.create_user.user_id}}"
    total: 99.99
    status: "pending"
    items:
      - product_id: "123"
        quantity: 2
  options:
    return: "representation"`,

        features: [
          "Direct Supabase API access",
          "CRUD operations",
          "Complex filtering",
          "RLS bypass with service key",
          "Batch operations",
          "Real-time subscriptions",
        ],
      },

      agent: {
        description: "AI Agent plugin for intelligent testing",
        basic: `plugin: agent
config:
  model: "gpt-4"
  prompt: |
    Analyze the following API response and verify:
    1. All required fields are present
    2. Data types are correct
    3. Business logic is valid
    
    Response: {{.steps.previous_step.response}}`,

        advanced: `plugin: agent
config:
  model: "claude-3"
  temperature: 0.2
  system_prompt: "You are a QA engineer validating test results."
  prompt: |
    Compare these two datasets and identify discrepancies:
    
    Expected: {{.vars.expected_data}}
    Actual: {{.steps.fetch_data.response}}
    
    Return a JSON object with:
    - matches: boolean
    - discrepancies: array of differences
    - severity: high/medium/low`,

        features: [
          "AI-powered test validation",
          "Natural language assertions",
          "Complex data comparison",
          "Pattern recognition",
          "Anomaly detection",
          "Dynamic test generation",
        ],
      },

      log: {
        description: "Log plugin for debugging and reporting",
        basic: `plugin: log
config:
  message: "Test checkpoint reached"
  level: "info"`,

        advanced: `plugin: log
config:
  message: |
    Test Status Report:
    - User ID: {{.steps.create_user.user_id}}
    - Order Total: {{.steps.create_order.total}}
    - Timestamp: {{.run.timestamp}}
  level: "info"
  metadata:
    test_id: "{{.run.uuid}}"
    environment: "{{.vars.environment}}"`,

        features: [
          "Structured logging",
          "Variable interpolation",
          "Multiple log levels",
          "Metadata attachment",
          "Test debugging",
          "Audit trails",
        ],
      },
    };

    const config = pluginConfigs[plugin];
    if (!config) {
      return {
        content: [
          {
            type: "text",
            text: `Unknown plugin: ${plugin}`,
          },
        ],
      };
    }

    let response = `# ${plugin.toUpperCase()} Plugin Configuration\n\n`;
    response += `${config.description}\n\n`;
    response += `Use case: ${use_case}\n\n`;

    response += `## Basic Configuration:\n\n`;
    response += "```yaml\n" + config.basic + "\n```\n\n";

    response += `## Advanced Configuration:\n\n`;
    response += "```yaml\n" + config.advanced + "\n```\n\n";

    response += `## Features:\n\n`;
    for (const feature of config.features) {
      response += `- ${feature}\n`;
    }

    response += `\n## Tips for ${plugin}:\n`;
    response += this.getPluginTips(plugin, use_case);

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private getPluginTips(plugin: string, use_case: string): string {
    const tips: Record<string, string[]> = {
      http: [
        "Always use variables for URLs and credentials",
        "Include appropriate timeouts for slow endpoints",
        "Use retry logic for flaky services",
        "Validate both success and error responses",
        "Log requests for debugging",
      ],
      sql: [
        "Use transactions for data modifications",
        "Always clean up test data",
        "Parameterize queries to prevent SQL injection",
        "Test both data presence and absence",
        "Consider database connection pooling",
      ],
      browser: [
        "Wait for elements before interacting",
        "Use stable selectors (IDs over classes)",
        "Take screenshots for debugging",
        "Test multiple viewport sizes",
        "Handle popups and alerts",
      ],
      script: [
        "Keep scripts focused and testable",
        "Use error handling for robustness",
        "Document complex logic",
        "Avoid hardcoded values",
        "Return meaningful error messages",
      ],
      supabase: [
        "Use anon key for public operations",
        "Use service key for admin operations",
        "Handle RLS policies appropriately",
        "Test error scenarios",
        "Clean up test data after runs",
      ],
    };

    const pluginTips = tips[plugin] || ["No specific tips available"];
    return pluginTips.map((tip) => `- ${tip}`).join("\n");
  }

  private async handleValidateAndSuggest(args: any) {
    const { yaml_content, improvement_focus } = args;

    // Basic YAML structure validation
    const issues: string[] = [];
    const suggestions: string[] = [];

    // Check for required fields
    if (!yaml_content.includes("version:")) {
      issues.push("Missing 'version' field");
      suggestions.push("Add 'version: \"v1.0.0\"' at the top of your file");
    } else if (!yaml_content.match(/version:\s*["']?v\d+\.\d+\.\d+/)) {
      issues.push("Version format incorrect");
      suggestions.push("Use format 'version: \"v1.0.0\"'");
    }

    if (!yaml_content.includes("name:")) {
      issues.push("Missing 'name' field");
      suggestions.push("Add a descriptive name for your test suite");
    }

    if (!yaml_content.includes("tests:")) {
      issues.push("Missing 'tests' section");
      suggestions.push("Add a 'tests:' section with at least one test");
    }

    // Check for best practices
    if (!yaml_content.includes("vars:") && !yaml_content.includes(".env.")) {
      suggestions.push(
        "Consider using variables for reusable values like URLs and tokens"
      );
    }

    if (!yaml_content.includes("assertions:")) {
      suggestions.push("Add assertions to validate your test results");
    }

    if (!yaml_content.includes("save:") && yaml_content.includes("steps:")) {
      suggestions.push("Consider using 'save:' to pass data between steps");
    }

    // Focus-specific suggestions
    if (improvement_focus === "performance") {
      suggestions.push("Add 'timeout' configurations to prevent hanging tests");
      suggestions.push(
        "Consider using 'delay' plugin between requests to avoid rate limiting"
      );
      suggestions.push("Use concurrent test execution where possible");
    } else if (improvement_focus === "assertions") {
      suggestions.push("Add more specific assertions beyond status codes");
      suggestions.push("Validate response structure with JSON path assertions");
      suggestions.push("Include negative test cases");
    } else if (improvement_focus === "coverage") {
      suggestions.push("Add tests for error scenarios (4xx, 5xx responses)");
      suggestions.push("Test edge cases and boundary conditions");
      suggestions.push("Include authentication and authorization tests");
    }

    let response = `# Rocketship Test Validation Results\n\n`;

    if (issues.length > 0) {
      response += `## Issues Found:\n\n`;
      for (const issue of issues) {
        response += `- ❌ ${issue}\n`;
      }
      response += `\n`;
    } else {
      response += `✅ Basic structure looks good!\n\n`;
    }

    response += `## Suggestions for Improvement:\n\n`;
    for (const suggestion of suggestions) {
      response += `- 💡 ${suggestion}\n`;
    }

    response += `\n## Next Steps:\n`;
    response += `1. Address any issues found above\n`;
    response += `2. Run 'rocketship validate <your-file>.yaml' to check syntax\n`;
    response += `3. Test with a single test first before running the full suite\n`;
    response += `4. Use 'rocketship run -af <your-file>.yaml --dry-run' to preview execution\n`;

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private async handleGetCLICommands(args: any) {
    const { command, context } = args;

    const commands: Record<string, any> = {
      run: {
        description: "Execute Rocketship tests",
        basic: `# Run a test file
rocketship run -f test.yaml

# Auto-start engine and run
rocketship run -af test.yaml

# Run with specific environment
rocketship run -af test.yaml --var-file staging-vars.yaml

# Run specific test by name
rocketship run -af test.yaml --test-name "Login Flow"`,

        advanced: `# Run with custom variables
rocketship run -af test.yaml --var API_KEY=abc123 --var BASE_URL=https://api.example.com

# Run in CI/CD mode (no interactive output)
rocketship run -af test.yaml --ci

# Run with specific engine
rocketship run -f test.yaml --engine localhost:7700

# Dry run (validate without executing)
rocketship run -af test.yaml --dry-run

# Run with debug logging
ROCKETSHIP_LOG=DEBUG rocketship run -af test.yaml`,

        flags: [
          "-f, --file: Test file to run",
          "-a, --auto: Auto-start local engine",
          "--var-file: Variable file path",
          "--var: Set individual variables",
          "--test-name: Run specific test",
          "--engine: Engine URL",
          "--ci: CI mode (non-interactive)",
          "--dry-run: Validate without running",
        ],
      },

      validate: {
        description: "Validate test file syntax",
        basic: `# Validate a test file
rocketship validate test.yaml

# Validate multiple files
rocketship validate tests/*.yaml`,

        advanced: `# Validate with variable resolution
rocketship validate test.yaml --var-file prod-vars.yaml

# Validate and show resolved variables
rocketship validate test.yaml --show-resolved

# Strict validation (fail on warnings)
rocketship validate test.yaml --strict`,

        flags: [
          "file: Test file(s) to validate",
          "--var-file: Variable file for resolution",
          "--show-resolved: Display resolved variables",
          "--strict: Treat warnings as errors",
        ],
      },

      start: {
        description: "Start Rocketship engine server",
        basic: `# Start local engine
rocketship start server --local

# Start in background
rocketship start server --local --background

# Start with custom port
rocketship start server --local --port 8800`,

        advanced: `# Start with custom Temporal
rocketship start server --local --temporal-address localhost:7233

# Start with debug logging
ROCKETSHIP_LOG=DEBUG rocketship start server --local

# Start with specific version
rocketship start server --version v1.2.0`,

        flags: [
          "--local: Start locally",
          "--background: Run in background",
          "--port: Engine port (default 7700)",
          "--temporal-address: Temporal server",
          "--version: Specific version",
        ],
      },

      stop: {
        description: "Stop Rocketship engine server",
        basic: `# Stop local engine
rocketship stop server

# Force stop
rocketship stop server --force`,

        advanced: `# Stop specific instance
rocketship stop server --pid 12345

# Stop all instances
rocketship stop server --all`,

        flags: [
          "--force: Force stop",
          "--pid: Stop specific process",
          "--all: Stop all instances",
        ],
      },

      general: {
        description: "General Rocketship CLI usage",
        basic: `# Show help
rocketship --help

# Show version
rocketship version

# List available commands
rocketship help`,

        advanced: `# Enable debug logging globally
export ROCKETSHIP_LOG=DEBUG

# Set default engine URL
export ROCKETSHIP_ENGINE_URL=https://engine.example.com

# Use config file
rocketship --config ~/.rocketship/config.yaml run -f test.yaml`,

        tips: [
          "Use 'rocketship run -af' for quick local testing",
          "Always validate files before running in production",
          "Use variable files to avoid hardcoding values",
          "Set ROCKETSHIP_LOG=DEBUG for troubleshooting",
          "Run 'rocketship help <command>' for detailed help",
        ],
      },
    };

    const cmdInfo = commands[command] || commands.general;

    let response = `# Rocketship CLI: ${command.toUpperCase()} Command\n\n`;
    response += `${cmdInfo.description}\n\n`;

    if (context) {
      response += `Context: ${context}\n\n`;
    }

    response += `## Basic Usage:\n\n`;
    response += "```bash\n" + cmdInfo.basic + "\n```\n\n";

    response += `## Advanced Usage:\n\n`;
    response += "```bash\n" + cmdInfo.advanced + "\n```\n\n";

    if (cmdInfo.flags) {
      response += `## Available Flags:\n\n`;
      for (const flag of cmdInfo.flags) {
        response += `- ${flag}\n`;
      }
      response += `\n`;
    }

    if (cmdInfo.tips) {
      response += `## Tips:\n\n`;
      for (const tip of cmdInfo.tips) {
        response += `- ${tip}\n`;
      }
    }

    response += `\n## Common Workflows:\n`;
    response += this.getWorkflowExamples(command);

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private getWorkflowExamples(command: string): string {
    if (command === "run") {
      return `
### Local Development:
\`\`\`bash
# Quick test during development
rocketship run -af my-test.yaml

# Test with staging variables
rocketship run -af my-test.yaml --var-file staging.yaml
\`\`\`

### CI/CD Pipeline:
\`\`\`bash
# In GitHub Actions or similar
rocketship validate tests/*.yaml
rocketship run -af tests/smoke-test.yaml --ci --var-file \${ENVIRONMENT}.yaml
\`\`\`

### Production Testing:
\`\`\`bash
# Run against production with specific engine
rocketship run -f prod-test.yaml --engine https://engine.prod.example.com --var-file prod-secure.yaml
\`\`\``;
    }

    return "See documentation for more workflow examples.";
  }

  async run() {
    const transport = new StdioServerTransport();
    await this.server.connect(transport);
  }
}

// Only run server if this file is executed directly
if (require.main === module) {
  const server = new RocketshipMCPServer();
  server.run().catch(console.error);
}
