#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";

// Static knowledge loader with embedded content
class RocketshipKnowledgeLoader {
  private schema: any = null;
  private examples: Map<string, any> = new Map();
  private docs: Map<string, string> = new Map();
  private cliData: any = null;

  constructor() {
    console.log(`Initializing Rocketship MCP Server v0.4.1`);

    this.loadEmbeddedKnowledge();
    console.log(`✓ Loaded embedded Rocketship knowledge`);
    console.log(`✓ Available examples: ${this.examples.size}`);
    console.log(`✓ Available docs: ${this.docs.size}`);
  }

  private loadEmbeddedKnowledge(): void {
    // Load embedded knowledge from build step - fail fast if not available
    const {
      EMBEDDED_SCHEMA,
      EMBEDDED_EXAMPLES,
      EMBEDDED_DOCS,
      EMBEDDED_CLI_DATA,
    } = require("./embedded-knowledge");

    this.schema = EMBEDDED_SCHEMA;
    this.examples = EMBEDDED_EXAMPLES;
    this.docs = EMBEDDED_DOCS;
    this.cliData = EMBEDDED_CLI_DATA;

    console.log(`📦 Loaded real embedded knowledge from build step`);
    console.log(`📋 CLI Version: ${this.cliData?.version?.version || 'unknown'}`);
  }

  getSchema(): any {
    return this.schema;
  }

  getExample(name: string): any {
    return this.examples.get(name);
  }

  getAllExamples(): string[] {
    return Array.from(this.examples.keys());
  }

  getDocumentation(path: string): string | undefined {
    return this.docs.get(path);
  }

  getAllDocs(): string[] {
    return Array.from(this.docs.keys());
  }

  getCLIData(): any {
    return this.cliData;
  }
}

// Initialize knowledge loader
const knowledgeLoader = new RocketshipKnowledgeLoader();

// Tool descriptions that emphasize the assistant nature
const TOOL_DESCRIPTIONS = {
  get_rocketship_examples: `Provides real examples from the Rocketship codebase for specific features or use cases.

💡 YOU (the coding agent) create the test files based on these examples.
💡 For frontend projects, consider using the browser plugin for user journey testing.
💡 Structure: .rocketship/ directory with subdirectories, each containing rocketship.yaml
💡 Variables: {{ .vars.name }} (config), {{ .env.NAME }} (environment), {{ name }} (runtime)
💡 JSON paths: .field_name for JSON paths (no $ prefix)`,

  suggest_test_structure: `Suggests proper Rocketship file structure and test organization for your project.

💡 YOU (the coding agent) create the directory structure and files.
💡 For frontend projects: Consider browser-based user journey testing.
💡 For API projects: Focus on user workflows rather than just coverage.`,

  get_schema_info: `Provides the current Rocketship schema information for validation and proper syntax.

💡 Use this to ensure your YAML follows the correct schema.
💡 Pay attention to required fields, valid plugin names, and assertion types.`,

  get_cli_guidance: `Provides correct Rocketship CLI usage patterns and commands.

💡 YOU (the coding agent) will run these commands to execute tests.
💡 Use rocketship run -af for auto-start with single file, -ad for directories.`,

  analyze_codebase_for_testing: `Analyzes a codebase to suggest meaningful test scenarios based on user journeys.

💡 Focus on customer-facing flows and critical business logic.
💡 For frontends: Consider browser testing of key user paths.
💡 For APIs: Test the endpoints that support those user paths.
💡 TIP: Include relevant keywords in your description to get better flow suggestions:
   - authentication, login, access, permissions (for auth flows)
   - dashboard, main, overview, portal (for main interface)
   - search, find, filter, browse (for discovery)
   - create, edit, manage, records, crud (for data management)
   - settings, config, preferences, account (for configuration)
   - process, workflow, submit, approve (for business processes)
   - reports, analytics, export, metrics (for reporting)
   - notifications, messages, alerts, communication (for messaging)`,
};

export class RocketshipMCPServer {
  private server: Server;
  private knowledgeLoader: RocketshipKnowledgeLoader;

  constructor() {
    this.knowledgeLoader = knowledgeLoader;
    this.server = new Server(
      {
        name: "rocketship-mcp",
        version: "0.4.1",
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
      // Dynamically get available plugins from schema
      const schema = this.knowledgeLoader.getSchema();
      const availablePlugins = schema?.properties?.tests?.items?.properties?.steps?.items?.properties?.plugin?.enum || [];
      
      return {
        tools: [
          {
            name: "get_rocketship_examples",
            description: TOOL_DESCRIPTIONS.get_rocketship_examples,
            inputSchema: {
              type: "object",
              properties: {
                feature_type: {
                  type: "string",
                  enum: availablePlugins,
                  description: "The plugin/feature type to get examples for",
                },
                use_case: {
                  type: "string",
                  description: "Specific use case or scenario you're testing",
                },
              },
              required: ["feature_type"],
            },
          },
          {
            name: "suggest_test_structure",
            description: TOOL_DESCRIPTIONS.suggest_test_structure,
            inputSchema: {
              type: "object",
              properties: {
                project_type: {
                  type: "string",
                  enum: ["frontend", "backend", "fullstack", "api", "mobile"],
                  description: "Type of project being tested",
                },
                user_flows: {
                  type: "array",
                  items: { type: "string" },
                  description:
                    "Key user journeys to test (e.g., 'user registration', 'purchase flow'). TIP: Use keywords like 'authentication', 'dashboard', 'search', 'records', 'settings', 'workflow', 'reports', 'notifications' for better suggestions.",
                },
              },
              required: ["project_type"],
            },
          },
          {
            name: "get_schema_info",
            description: TOOL_DESCRIPTIONS.get_schema_info,
            inputSchema: {
              type: "object",
              properties: {
                section: {
                  type: "string",
                  enum: ["plugins", "assertions", "save", "structure", "full"],
                  description: "Which part of the schema to focus on",
                },
              },
              required: ["section"],
            },
          },
          {
            name: "get_cli_guidance",
            description: TOOL_DESCRIPTIONS.get_cli_guidance,
            inputSchema: {
              type: "object",
              properties: {
                command: {
                  type: "string",
                  enum: ["run", "validate", "structure"],
                  description: "CLI command guidance needed",
                },
              },
              required: ["command"],
            },
          },
          {
            name: "analyze_codebase_for_testing",
            description: TOOL_DESCRIPTIONS.analyze_codebase_for_testing,
            inputSchema: {
              type: "object",
              properties: {
                codebase_info: {
                  type: "string",
                  description:
                    "Description of the codebase structure and functionality",
                },
                focus_area: {
                  type: "string",
                  enum: [
                    "user_journeys",
                    "api_endpoints",
                    "critical_paths",
                    "integration_points",
                  ],
                  description: "What aspect to focus testing on",
                },
                suggested_flows: {
                  type: "array",
                  items: { type: "string" },
                  description:
                    "Optional: Specific flows you think are most relevant (e.g., 'authentication', 'data-management', 'reporting')",
                },
              },
              required: ["codebase_info", "focus_area"],
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
          case "get_schema_info":
            return this.handleGetSchemaInfo(args);
          case "get_cli_guidance":
            return this.handleGetCLIGuidance(args);
          case "analyze_codebase_for_testing":
            return this.handleAnalyzeCodebase(args);
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
    const { feature_type, use_case } = args;

    // Get embedded examples
    const allExamples = this.knowledgeLoader.getAllExamples();
    const relevantExamples = allExamples.filter(
      (name) =>
        name.includes(feature_type) ||
        (feature_type === "browser" && name.includes("browser")) ||
        (feature_type === "http" &&
          (name.includes("http") || name.includes("request"))) ||
        (feature_type === "sql" && name.includes("sql")) ||
        (feature_type === "agent" && name.includes("agent"))
    );

    let response = `# Real Rocketship Examples for ${feature_type}\n\n`;

    if (use_case) {
      response += `Use case: ${use_case}\n\n`;
    }

    // Add suggestions for frontend projects
    if (
      feature_type === "browser" ||
      use_case?.toLowerCase().includes("frontend")
    ) {
      response += `💡 **Frontend Testing Considerations:**\n`;
      response += `- Browser plugin is great for testing user journeys\n`;
      response += `- Consider testing user flows in addition to API endpoints\n`;
      response += `- Focus on the most important customer paths\n\n`;
    }

    response += `## File Structure\n`;
    response += `Create this structure (YOU must create these files):\n`;
    response += `\`\`\`\n`;
    response += `.rocketship/\n`;
    response += `├── login-flow/\n`;
    response += `│   └── rocketship.yaml\n`;
    response += `├── purchase-journey/\n`;
    response += `│   └── rocketship.yaml\n`;
    response += `└── dashboard-tests/\n`;
    response += `    └── rocketship.yaml\n`;
    response += `\`\`\`\n\n`;

    // Show real examples
    response += `## Real Examples from Codebase\n\n`;

    // Show all relevant examples (we have fewer now)
    for (const exampleName of relevantExamples) {
      const example = this.knowledgeLoader.getExample(exampleName);
      if (example) {
        response += `### ${exampleName}\n\n`;
        response += `\`\`\`yaml\n${example.content}\`\`\`\n\n`;
      }
    }

    // If no specific examples found, show a generic one
    if (relevantExamples.length === 0) {
      const firstExample = this.knowledgeLoader.getExample(allExamples[0]);
      if (firstExample) {
        response += `### Example (${allExamples[0]})\n\n`;
        response += `\`\`\`yaml\n${firstExample.content}\`\`\`\n\n`;
      }
    }

    // Add syntax reminders
    response += `## Critical Syntax Rules\n\n`;
    response += `**Variable Types:**\n`;
    response += `- Config variables: \`{{ .vars.variable_name }}\` (from vars section)\n`;
    response += `- Environment variables: \`{{ .env.VARIABLE_NAME }}\` (from system env)\n`;
    response += `- Runtime variables: \`{{ variable_name }}\` (from save operations)\n\n`;
    response += `**Other syntax:**\n`;
    response += `- JSON paths: \`.field_name\` or \`.items_0.id\` (NO $ prefix)\n`;
    response += `- File names: Always \`rocketship.yaml\` in subdirectories\n`;
    response += `- Step chaining: Save with \`json_path: ".field"\` and \`as: "var_name"\`\n\n`;

    // Add browser-specific guidance
    if (feature_type === "browser") {
      response += `## Browser Plugin Requirements\n\n`;
      response += `- \`task\`: Natural language description of what to do\n`;
      response += `- \`llm\`: Configuration with provider and model\n`;
      response += `- Common tasks: "Navigate to X and Y", "Fill form and submit", "Extract data from page"\n\n`;

      const browserDoc = this.knowledgeLoader.getDocumentation(
        "examples/browser-testing.md"
      );
      if (browserDoc) {
        // Extract key sections from browser documentation
        const configMatch = browserDoc.match(
          /## Basic Configuration\n\n```yaml\n([\s\S]*?)\n```/
        );
        if (configMatch) {
          response += `## Browser Plugin Configuration\n\n\`\`\`yaml\n${configMatch[1]}\`\`\`\n\n`;
        }
      }
    }

    response += `## Next Steps\n`;
    response += `1. YOU create the .rocketship/ directory structure\n`;
    response += `2. YOU write the rocketship.yaml files based on these examples\n`;
    response += `3. Run: \`rocketship run -ad .rocketship\` to execute all tests\n`;

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
    const { project_type, user_flows } = args;

    let response = `# Rocketship Test Structure for ${project_type} Project\n\n`;

    response += `💡 **Note: YOU create all directories and files yourself**\n\n`;

    // Project-specific guidance
    if (project_type === "frontend" || project_type === "fullstack") {
      response += `## 🌐 Frontend Testing Strategy\n\n`;
      response += `**Recommended: Browser plugin for user journey testing**\n\n`;

      response += `Create this structure:\n\`\`\`\n`;
      response += `.rocketship/\n`;
      if (user_flows && user_flows.length > 0) {
        for (const flow of user_flows.slice(0, 5)) {
          const cleanName = flow
            .toLowerCase()
            .replace(/\s+/g, "-")
            .replace(/[^a-z0-9-]/g, "");
          response += `├── ${cleanName}/\n`;
          response += `│   └── rocketship.yaml\n`;
        }
      } else {
        response += `├── user-registration/\n`;
        response += `│   └── rocketship.yaml\n`;
        response += `├── login-flow/\n`;
        response += `│   └── rocketship.yaml\n`;
        response += `├── main-dashboard/\n`;
        response += `│   └── rocketship.yaml\n`;
      }
      response += `\`\`\`\n\n`;

      response += `### Getting Started\n\n`;
      
      // Direct users to use the examples tool
      response += `For complete working examples, use:\n`;
      response += `\`\`\`\nget_rocketship_examples feature_type="browser"\n\`\`\`\n\n`;
      response += `This will show you real browser testing examples from the Rocketship codebase.\n\n`;
      
      response += `### Variable Types\n\n`;
      response += `Rocketship supports three types of variables:\n`;
      response += `- **Config variables**: \`{{ .vars.variable_name }}\` (from vars section)\n`;
      response += `- **Environment variables**: \`{{ .env.VARIABLE_NAME }}\` (from system env)\n`;
      response += `- **Runtime variables**: \`{{ variable_name }}\` (from save operations)\n\n`;
    } else {
      response += `## 🔌 API Testing Strategy\n\n`;
      response += `Focus on user journey endpoints (not just coverage)\n\n`;

      response += `Create this structure:\n\`\`\`\n`;
      response += `.rocketship/\n`;
      response += `├── health-checks/\n`;
      response += `│   └── rocketship.yaml\n`;
      response += `├── user-authentication/\n`;
      response += `│   └── rocketship.yaml\n`;
      response += `├── core-business-flow/\n`;
      response += `│   └── rocketship.yaml\n`;
      response += `\`\`\`\n\n`;
    }

    response += `## CLI Commands YOU Need to Run\n\n`;
    response += `\`\`\`bash\n`;
    response += `# Validate your YAML files\n`;
    response += `rocketship validate .rocketship\n`;
    response += `\n`;
    response += `# Run all tests with auto-start/stop\n`;
    response += `rocketship run -ad .rocketship\n`;
    response += `\n`;
    response += `# Run specific test\n`;
    response += `rocketship run -af .rocketship/login-flow/rocketship.yaml\n`;
    response += `\`\`\`\n\n`;

    response += `## Key Reminders\n`;
    response += `- File names: Always \`rocketship.yaml\` (exact name)\n`;
    response += `- Directory structure: \`.rocketship/test-name/rocketship.yaml\`\n`;
    response += `- Config variables: \`{{ .vars.name }}\`, Environment: \`{{ .env.NAME }}\`, Runtime: \`{{ name }}\`\n`;
    response += `- JSON paths: \`.field_name\` (no $ prefix)\n`;

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private async handleGetSchemaInfo(args: any) {
    const { section } = args;
    const schema = this.knowledgeLoader.getSchema();

    let response = `# Rocketship Schema Information\n\n`;

    if (section === "plugins" || section === "full") {
      response += `## Available Plugins\n\n`;
      const pluginEnum =
        schema?.properties?.tests?.items?.properties?.steps?.items?.properties
          ?.plugin?.enum;
      if (pluginEnum) {
        response += `The following plugins are available in the current schema:\n\n`;
        
        // Just list the plugins - let users explore examples for details
        for (const plugin of pluginEnum) {
          response += `- **${plugin}**`;
          
          // Check if we have examples for this plugin
          const examples = this.knowledgeLoader.getAllExamples().filter(e => e.includes(plugin));
          if (examples.length > 0) {
            response += ` - See examples: ${examples.join(", ")}\n`;
          } else {
            response += `\n`;
          }
        }
        response += `\nFor detailed usage and configuration, use \`get_rocketship_examples feature_type="<plugin>"\`.\n`;
      }
      response += `\n`;
    }

    if (section === "assertions" || section === "full") {
      response += `## Valid Assertion Types\n\n`;
      const assertionTypes =
        schema?.properties?.tests?.items?.properties?.steps?.items?.properties
          ?.assertions?.items?.properties?.type?.enum;
      if (assertionTypes) {
        response += `Available assertion types from the schema:\n\n`;
        for (const type of assertionTypes) {
          response += `- **${type}**\n`;
        }
        response += `\nFor usage examples and syntax, use \`get_rocketship_examples\` to see real test files.\n`;
      }
      response += `\n`;
    }

    if (section === "save" || section === "full") {
      response += `## Save Syntax\n\n`;
      response += `Save data from responses for use in later steps:\n\n`;
      response += `\`\`\`yaml\n`;
      response += `save:\n`;
      response += `  - json_path: ".field_name"  # No $ prefix!\n`;
      response += `    as: "variable_name"\n`;
      response += `  - header: "Content-Type"\n`;
      response += `    as: "content_type"\n`;
      response += `\`\`\`\n\n`;
      response += `**Variable usage examples:**\n`;
      response += `- Config: \`{{ .vars.api_url }}\` (from vars section)\n`;
      response += `- Environment: \`{{ .env.API_KEY }}\` (from system)\n`;
      response += `- Runtime: \`{{ user_id }}\` (from save operations)\n\n`;
    }

    if (section === "structure" || section === "full") {
      response += `## File Structure Requirements\n\n`;
      response += `Based on the current schema:\n\n`;
      
      // Dynamically extract all structural information from schema
      const requiredTopLevel = schema?.required || [];
      const topLevelProps = schema?.properties || {};
      
      response += `### Top-level fields:\n`;
      for (const [key, prop] of Object.entries(topLevelProps)) {
        const isRequired = requiredTopLevel.includes(key);
        response += `- **${key}** (${isRequired ? 'required' : 'optional'})`;
        if (prop && typeof prop === 'object' && 'type' in prop) {
          response += `: ${(prop as any).type}`;
        }
        response += `\n`;
      }
      response += `\n`;
      
      // Show test structure if available
      if (topLevelProps.tests?.items?.properties) {
        const testRequired = topLevelProps.tests.items.required || [];
        const testProps = topLevelProps.tests.items.properties || {};
        
        response += `### Test structure:\n`;
        for (const [key, prop] of Object.entries(testProps)) {
          const isRequired = testRequired.includes(key);
          response += `- **${key}** (${isRequired ? 'required' : 'optional'})`;
          if (prop && typeof prop === 'object' && 'type' in prop) {
            response += `: ${(prop as any).type}`;
          }
          response += `\n`;
        }
        response += `\n`;
      }
      
      // Show step structure if available
      if (topLevelProps.tests?.items?.properties?.steps?.items?.properties) {
        const stepRequired = topLevelProps.tests.items.properties.steps.items.required || [];
        const stepProps = topLevelProps.tests.items.properties.steps.items.properties || {};
        
        response += `### Step structure:\n`;
        for (const [key, prop] of Object.entries(stepProps)) {
          const isRequired = stepRequired.includes(key);
          response += `- **${key}** (${isRequired ? 'required' : 'optional'})`;
          if (prop && typeof prop === 'object' && 'type' in prop) {
            response += `: ${(prop as any).type}`;
          }
          response += `\n`;
        }
        response += `\n`;
      }
      
      response += `For complete examples with proper syntax, use \`get_rocketship_examples\`.\n\n`;
    }

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private async handleGetCLIGuidance(args: any) {
    const { command } = args;
    const cliData = this.knowledgeLoader.getCLIData();

    let response = `# Rocketship CLI Guidance\n\n`;
    
    if (cliData?.version?.version) {
      response += `*Current CLI Version: ${cliData.version.version}*\n\n`;
    }

    // Use actual CLI help data if available
    if (command === "run" || command === "structure") {
      response += `## Running Tests\n\n`;
      
      if (cliData?.help?.run?.help) {
        // Extract examples from actual CLI help
        const runHelp = cliData.help.run.help;
        if (runHelp.includes('Usage:')) {
          response += `### Current CLI Usage\n`;
          response += `\`\`\`\n${runHelp}\`\`\`\n\n`;
        }
      }

      response += `### Common Patterns (Updated)\n`;
      response += `\`\`\`bash\n`;
      response += `# Run single test file with auto-start/stop\n`;
      response += `rocketship run -af .rocketship/login-flow/rocketship.yaml\n`;
      response += `\n`;
      response += `# Run all tests in directory with auto-start/stop\n`;
      response += `rocketship run -ad .rocketship\n`;
      response += `\n`;
      response += `# Run with variables (CORRECT FLAG)\n`;
      response += `rocketship run -ad .rocketship --var APP_URL=http://localhost:3000\n`;
      response += `\n`;
      response += `# Load variables from file (CORRECT FLAG)\n`;
      response += `rocketship run -af test.yaml --var-file vars.yaml\n`;
      response += `\`\`\`\n\n`;

      // Manual server mode using actual CLI data
      if (cliData?.help?.start?.subcommands?.server) {
        response += `### Manual Engine Mode\n`;
        response += `\`\`\`bash\n`;
        response += `# Start engine in background\n`;
        response += `rocketship start server -b\n`;
        response += `\n`;
        response += `# Run tests against running engine\n`;
        response += `rocketship run -d .rocketship\n`;
        response += `\n`;
        response += `# Stop engine\n`;
        response += `rocketship stop server\n`;
        response += `\`\`\`\n\n`;
      }
    }

    if (command === "validate" || command === "structure") {
      response += `## Validation\n\n`;
      
      if (cliData?.help?.validate?.help) {
        response += `### Current Validate Command\n`;
        response += `\`\`\`\n${cliData.help.validate.help}\`\`\`\n\n`;
      }
      
      response += `### Validation Examples\n`;
      response += `\`\`\`bash\n`;
      response += `# Validate single file\n`;
      response += `rocketship validate .rocketship/login-flow/rocketship.yaml\n`;
      response += `\n`;
      response += `# Validate directory\n`;
      response += `rocketship validate .rocketship\n`;
      response += `\`\`\`\n\n`;
    }

    // Use actual flag data
    response += `## Current CLI Flags\n\n`;
    if (cliData?.flags) {
      for (const [flag, info] of Object.entries(cliData.flags as any)) {
        const flagInfo = info as any;
        response += `- \`${flagInfo.short ? flagInfo.short + ', ' : ''}${flagInfo.long}\`: ${flagInfo.description}\n`;
      }
    } else {
      // Fallback to known correct flags
      response += `- \`-a, --auto\`: Auto-start and stop engine\n`;
      response += `- \`-f, --file\`: Single test file\n`;
      response += `- \`-d, --dir\`: Directory of tests\n`;
      response += `- \`--var key=value\`: Set variables (NOT --vars)\n`;
      response += `- \`--var-file path\`: Load variables from file (NOT --var-file)\n`;
    }
    response += `\n`;

    response += `## Installation (Current Methods)\n\n`;
    if (cliData?.installation) {
      response += `**Recommended:** ${cliData.installation.recommended}\n\n`;
      
      if (cliData.installation.methods) {
        response += `**Available Methods:**\n`;
        for (const method of cliData.installation.methods) {
          response += `- **${method.name}**: ${method.description}\n`;
          response += `  \`\`\`bash\n  ${method.command}\n  \`\`\`\n`;
        }
      }
      
      if (cliData.installation.notAvailable) {
        response += `\n**NOT Available:**\n`;
        for (const na of cliData.installation.notAvailable) {
          response += `- ${na}\n`;
        }
      }
    }

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private async handleAnalyzeCodebase(args: any) {
    const { codebase_info, focus_area, suggested_flows } = args;

    let response = `# Test Strategy Analysis\n\n`;
    response += `Codebase: ${codebase_info}\n\n`;

    // Detect if this is a frontend project
    const isFrontend =
      codebase_info.toLowerCase().includes("react") ||
      codebase_info.toLowerCase().includes("vue") ||
      codebase_info.toLowerCase().includes("frontend") ||
      codebase_info.toLowerCase().includes("client") ||
      codebase_info.toLowerCase().includes("ui");

    if (isFrontend && focus_area === "user_journeys") {
      response += `💡 **Frontend Detected - Browser Testing Recommended**\n\n`;
      response += `## Critical User Journeys to Test\n\n`;

      // Use suggested flows if provided, otherwise extract from description
      const flows =
        suggested_flows && suggested_flows.length > 0
          ? suggested_flows.map(
              (f: string) =>
                f.charAt(0).toUpperCase() + f.slice(1).replace(/-/g, " ")
            )
          : this.extractUserFlows(codebase_info);

      if (suggested_flows && suggested_flows.length > 0) {
        response += `*Using your suggested flows: ${suggested_flows.join(
          ", "
        )}*\n\n`;
      }

      for (let i = 0; i < flows.length; i++) {
        const flow = flows[i];
        const dirName = flow
          .toLowerCase()
          .replace(/\s+/g, "-")
          .replace(/[^a-z0-9-]/g, "");

        response += `### ${i + 1}. ${flow}\n\n`;
        response += `**Directory:** \`.rocketship/${dirName}/rocketship.yaml\`\n\n`;
        response += `**Test Strategy:**\n`;
        response += `- Use the browser plugin for E2E testing\n`;
        response += `- Define natural language tasks for the AI agent\n`;
        response += `- Add assertions to verify expected outcomes\n\n`;
        response += `For specific YAML syntax and examples, use:\n`;
        response += `\`\`\`\nget_rocketship_examples feature_type="browser"\n\`\`\`\n\n`;
      }
    } else if (focus_area === "api_endpoints") {
      response += `## API Testing Strategy\n\n`;
      response += `Focus on endpoints that support user journeys, not just coverage.\n\n`;

      // Try to find HTTP examples from embedded knowledge
      const httpExamples = this.knowledgeLoader.getAllExamples().filter(e => e.includes("http"));
      if (httpExamples.length > 0) {
        response += `See real HTTP testing examples with \`get_rocketship_examples feature_type="http"\`.\n\n`;
      } else {
        response += `For API testing examples, use \`get_rocketship_examples\`.\n\n`;
      }
    }

    response += `## Recommended Test Structure\n\n`;
    response += `\`\`\`\n`;
    response += `.rocketship/\n`;

    if (isFrontend) {
      const flows =
        suggested_flows && suggested_flows.length > 0
          ? suggested_flows.map(
              (f: string) =>
                f.charAt(0).toUpperCase() + f.slice(1).replace(/-/g, " ")
            )
          : this.extractUserFlows(codebase_info);
      for (const flow of flows.slice(0, 5)) {
        const dirName = flow
          .toLowerCase()
          .replace(/\s+/g, "-")
          .replace(/[^a-z0-9-]/g, "");
        response += `├── ${dirName}/\n`;
        response += `│   └── rocketship.yaml    # Browser-based E2E test\n`;
      }
    } else {
      response += `├── health-checks/\n`;
      response += `│   └── rocketship.yaml    # API health validation\n`;
      response += `├── authentication/\n`;
      response += `│   └── rocketship.yaml    # Auth flows\n`;
      response += `├── core-workflows/\n`;
      response += `│   └── rocketship.yaml    # Main business logic\n`;
    }

    response += `\`\`\`\n\n`;

    response += `## Next Steps\n\n`;
    response += `1. **YOU create the directory structure above**\n`;
    response += `2. **YOU write the rocketship.yaml files**\n`;
    response += `3. Run: \`rocketship validate .rocketship\`\n`;
    response += `4. Run: \`rocketship run -ad .rocketship\`\n\n`;

    if (isFrontend) {
      response += `💡 **Tip:** Consider using the browser plugin for frontend testing in addition to API calls.\n`;
    }

    if (!suggested_flows || suggested_flows.length === 0) {
      response += `\n💡 **Pro Tip:** For more targeted suggestions, you can specify \`suggested_flows\` like:\n`;
      response += `   ["authentication", "data-management", "reporting"] based on your codebase analysis.\n`;
    }

    return {
      content: [
        {
          type: "text",
          text: response,
        },
      ],
    };
  }

  private extractUserFlows(description: string): string[] {
    const universalFlows = [
      "User Authentication",
      "Main Dashboard",
      "Record Management",
      "Search & Filter",
      "Settings & Configuration",
      "Data Entry & Forms",
      "Reports & Analytics",
    ];

    // Extract flows based on universal software patterns
    const flows: string[] = [];
    const lowerDesc = description.toLowerCase();

    // Authentication & Access Control (universal)
    if (
      lowerDesc.includes("auth") ||
      lowerDesc.includes("login") ||
      lowerDesc.includes("sign") ||
      lowerDesc.includes("access") ||
      lowerDesc.includes("permission")
    ) {
      flows.push("User Authentication");
    }

    // Main Interface Navigation (universal)
    if (
      lowerDesc.includes("dashboard") ||
      lowerDesc.includes("home") ||
      lowerDesc.includes("main") ||
      lowerDesc.includes("overview") ||
      lowerDesc.includes("portal")
    ) {
      flows.push("Main Dashboard");
    }

    // Search & Discovery (universal)
    if (
      lowerDesc.includes("search") ||
      lowerDesc.includes("find") ||
      lowerDesc.includes("filter") ||
      lowerDesc.includes("browse") ||
      lowerDesc.includes("discover")
    ) {
      flows.push("Search & Filter");
    }

    // Data Management (universal CRUD)
    if (
      lowerDesc.includes("create") ||
      lowerDesc.includes("add") ||
      lowerDesc.includes("edit") ||
      lowerDesc.includes("update") ||
      lowerDesc.includes("delete") ||
      lowerDesc.includes("manage") ||
      lowerDesc.includes("crud") ||
      lowerDesc.includes("record") ||
      lowerDesc.includes("entry")
    ) {
      flows.push("Record Management");
    }

    // User Profile & Settings (universal)
    if (
      lowerDesc.includes("profile") ||
      lowerDesc.includes("account") ||
      lowerDesc.includes("settings") ||
      lowerDesc.includes("preferences") ||
      lowerDesc.includes("config")
    ) {
      flows.push("Settings & Configuration");
    }

    // Transaction/Processing Flows (universal)
    if (
      lowerDesc.includes("submit") ||
      lowerDesc.includes("process") ||
      lowerDesc.includes("approve") ||
      lowerDesc.includes("workflow") ||
      lowerDesc.includes("transaction") ||
      lowerDesc.includes("request") ||
      lowerDesc.includes("application")
    ) {
      flows.push("Process Workflow");
    }

    // Reporting & Analytics (universal)
    if (
      lowerDesc.includes("report") ||
      lowerDesc.includes("analytics") ||
      lowerDesc.includes("chart") ||
      lowerDesc.includes("export") ||
      lowerDesc.includes("download") ||
      lowerDesc.includes("view") ||
      lowerDesc.includes("metrics")
    ) {
      flows.push("Reports & Analytics");
    }

    // Communication & Notifications (universal)
    if (
      lowerDesc.includes("message") ||
      lowerDesc.includes("notification") ||
      lowerDesc.includes("alert") ||
      lowerDesc.includes("email") ||
      lowerDesc.includes("communication") ||
      lowerDesc.includes("chat")
    ) {
      flows.push("Notifications & Communication");
    }

    // Add universal flows if none detected
    if (flows.length === 0) {
      flows.push(...universalFlows.slice(0, 3));
    }

    return flows.slice(0, 5); // Limit to 5 flows
  }

  private generateBrowserTask(flow: string): string {
    // Generate generic task description
    return `Complete the ${flow.toLowerCase()} user journey`;
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
