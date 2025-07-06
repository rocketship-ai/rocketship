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

// REMOVED: Hard-coded tool descriptions replaced with dynamic generation
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

// Generate dynamic tool descriptions based on CLI introspection data
function generateToolDescriptions(knowledgeLoader: RocketshipKnowledgeLoader) {
  const cliData = knowledgeLoader.getCLIData();
  const schema = knowledgeLoader.getSchema();
  const availablePlugins = schema?.properties?.tests?.items?.properties?.steps?.items?.properties?.plugin?.enum || [];

  // Extract dynamic information
  const filePattern = cliData?.usage?.file_structure?.pattern || 'rocketship.yaml';
  const varExamples = cliData?.usage?.syntax_patterns?.variables || {};
  const commonCommands = cliData?.usage?.common_patterns || [];
  
  // Build variable syntax examples from extracted data
  let variableSyntax = '';
  if (varExamples.config && varExamples.config.length > 0) {
    variableSyntax += `💡 Config variables: ${varExamples.config.slice(0, 2).join(', ')}\n`;
  }
  if (varExamples.environment && varExamples.environment.length > 0) {
    variableSyntax += `💡 Environment variables: ${varExamples.environment.slice(0, 2).join(', ')}\n`;
  }
  if (varExamples.runtime && varExamples.runtime.length > 0) {
    variableSyntax += `💡 Runtime variables: ${varExamples.runtime.slice(0, 2).join(', ')}\n`;
  }

  // Build CLI command examples from extracted data
  let cliExamples = '';
  const runCommands = commonCommands.filter((c: any) => c.command.includes('run')).slice(0, 2);
  if (runCommands.length > 0) {
    cliExamples = `💡 Example commands: ${runCommands.map((c: any) => c.command).join(', ')}`;
  }

  // Build plugin recommendations from available plugins
  let pluginRecommendations = '';
  if (availablePlugins.includes('browser')) {
    pluginRecommendations += '💡 For frontend projects: browser plugin available for user journey testing\n';
  }
  if (availablePlugins.includes('http')) {
    pluginRecommendations += '💡 For API projects: http plugin available for endpoint testing\n';
  }

  return {
    get_rocketship_examples: `Provides real examples from the current codebase for specific features or use cases.

💡 YOU (the coding agent) create the test files based on these examples.
${pluginRecommendations}💡 File pattern: ${filePattern}
${variableSyntax}`,

    suggest_test_structure: `Suggests proper file structure and test organization based on current project configuration.

💡 YOU (the coding agent) create the directory structure and files.
${pluginRecommendations}💡 Available plugins: ${availablePlugins.join(', ')}`,

    get_schema_info: `Provides current schema information for validation and proper syntax.

💡 Use this to ensure your YAML follows the correct schema.
💡 Available plugins: ${availablePlugins.join(', ')}
💡 Schema validation ensures compatibility with current version.`,

    get_cli_guidance: `Provides current CLI usage patterns and commands from introspection.

💡 YOU (the coding agent) will run these commands to execute tests.
${cliExamples}
💡 All commands are extracted from current CLI version.`,

    analyze_codebase_for_testing: `Analyzes a codebase to suggest meaningful test scenarios based on available plugins.

💡 Focus on customer-facing flows and critical business logic.
${pluginRecommendations}💡 Suggestions are based on available plugins: ${availablePlugins.join(', ')}
💡 TIP: Include relevant keywords for better flow suggestions`,
  };
}

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
      // Generate dynamic tool descriptions based on current CLI introspection data
      const dynamicDescriptions = generateToolDescriptions(this.knowledgeLoader);
      const schema = this.knowledgeLoader.getSchema();
      const availablePlugins = schema?.properties?.tests?.items?.properties?.steps?.items?.properties?.plugin?.enum || [];
      
      return {
        tools: [
          {
            name: "get_rocketship_examples",
            description: dynamicDescriptions.get_rocketship_examples,
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
            description: dynamicDescriptions.suggest_test_structure,
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
            description: dynamicDescriptions.get_schema_info,
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
            description: dynamicDescriptions.get_cli_guidance,
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
            description: dynamicDescriptions.analyze_codebase_for_testing,
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

    // Use extracted file structure from CLI introspection
    const cliData = this.knowledgeLoader.getCLIData();
    if (cliData?.usage?.file_structure) {
      response += `## File Structure\n`;
      response += `Create this structure (YOU must create these files):\n`;
      
      if (cliData.usage.file_structure.examples) {
        Object.entries(cliData.usage.file_structure.examples).forEach(([key, example]) => {
          response += `\`\`\`\n${example}\`\`\`\n\n`;
        });
      } else if (cliData.usage.file_structure.pattern) {
        response += `Pattern: \`${cliData.usage.file_structure.pattern}\`\n\n`;
      }
    }

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

    // Use extracted syntax patterns from CLI introspection
    if (cliData?.usage?.syntax_patterns) {
      response += `## Syntax Rules (from Documentation)\n\n`;
      
      if (cliData.usage.syntax_patterns.variables) {
        response += `**Variable Types:**\n`;
        const vars = cliData.usage.syntax_patterns.variables;
        
        if (vars.config && vars.config.length > 0) {
          response += `- Config variables: Examples: ${vars.config.slice(0, 3).map(v => `\`${v}\``).join(', ')}\n`;
        }
        if (vars.environment && vars.environment.length > 0) {
          response += `- Environment variables: Examples: ${vars.environment.slice(0, 3).map(v => `\`${v}\``).join(', ')}\n`;
        }
        if (vars.runtime && vars.runtime.length > 0) {
          response += `- Runtime variables: Examples: ${vars.runtime.slice(0, 3).map(v => `\`${v}\``).join(', ')}\n`;
        }
        response += `\n`;
      }
      
      if (cliData.usage.syntax_patterns.save_operations && Object.keys(cliData.usage.syntax_patterns.save_operations).length > 0) {
        response += `**Save Operations:**\n`;
        Object.entries(cliData.usage.syntax_patterns.save_operations).slice(0, 2).forEach(([key, example]) => {
          response += `\`\`\`yaml\n${example}\`\`\`\n`;
        });
        response += `\n`;
      }
    }

    // Use extracted plugin-specific guidance from CLI introspection
    if (feature_type === "browser" && cliData?.usage?.syntax_patterns?.plugins?.browser) {
      response += `## Browser Plugin Guidance (from Documentation)\n\n`;
      cliData.usage.syntax_patterns.plugins.browser.forEach((context: string, index: number) => {
        response += `### Example ${index + 1}\n\`\`\`yaml\n${context}\`\`\`\n\n`;
      });
    }

    // Use extracted CLI commands for next steps
    response += `## Next Steps\n`;
    response += `1. YOU create the directory structure shown above\n`;
    response += `2. YOU write the rocketship.yaml files based on these examples\n`;
    
    // Use actual CLI commands from introspection
    if (cliData?.usage?.common_patterns) {
      const runPatterns = cliData.usage.common_patterns.filter((p: any) => 
        p.command.includes('run') && (p.command.includes('-ad') || p.command.includes('--dir'))
      );
      if (runPatterns.length > 0) {
        response += `3. Run: \`${runPatterns[0].command}\` to execute tests\n`;
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

  private async handleSuggestStructure(args: any) {
    const { project_type, user_flows } = args;
    const cliData = this.knowledgeLoader.getCLIData();

    let response = `# Rocketship Test Structure for ${project_type} Project\n\n`;
    response += `💡 **Note: YOU create all directories and files yourself**\n\n`;

    // Use extracted file structure patterns from CLI introspection
    if (cliData?.usage?.file_structure) {
      response += `## Recommended Structure (from Documentation)\n\n`;
      
      if (cliData.usage.file_structure.pattern) {
        response += `**Pattern:** \`${cliData.usage.file_structure.pattern}\`\n\n`;
      }
      
      if (cliData.usage.file_structure.examples) {
        Object.entries(cliData.usage.file_structure.examples).forEach(([key, example]) => {
          response += `### ${key}\n\`\`\`\n${example}\`\`\`\n\n`;
        });
      }
      
      // Show real examples from actual codebase
      if (cliData.usage.file_structure.real_examples) {
        response += `### Real Examples in Codebase\n`;
        Object.entries(cliData.usage.file_structure.real_examples).forEach(([dir, info]: [string, any]) => {
          response += `- \`${info.path}\` ✓\n`;
        });
        response += `\n`;
      }
    }

    // Project-specific guidance with dynamic examples
    const availablePlugins = this.knowledgeLoader.getSchema()?.properties?.tests?.items?.properties?.steps?.items?.properties?.plugin?.enum || [];
    
    if (project_type === "frontend" || project_type === "fullstack") {
      response += `## 🌐 Frontend Testing Strategy\n\n`;
      
      if (availablePlugins.includes("browser")) {
        response += `**Available: Browser plugin for user journey testing**\n\n`;
        response += `For complete working examples, use:\n`;
        response += `\`\`\`\nget_rocketship_examples feature_type="browser"\n\`\`\`\n\n`;
      }
      
      // Generate user flow structure dynamically
      if (user_flows && user_flows.length > 0) {
        response += `### Suggested Structure for Your Flows\n\`\`\`\n`;
        response += `.rocketship/\n`;
        for (const flow of user_flows.slice(0, 5)) {
          const cleanName = flow.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "");
          response += `├── ${cleanName}/\n`;
          response += `│   └── rocketship.yaml\n`;
        }
        response += `\`\`\`\n\n`;
      }
    } else {
      response += `## 🔌 API Testing Strategy\n\n`;
      response += `Focus on user journey endpoints (not just coverage)\n\n`;
      
      if (availablePlugins.includes("http")) {
        response += `**Available: HTTP plugin for API testing**\n\n`;
        response += `For complete working examples, use:\n`;
        response += `\`\`\`\nget_rocketship_examples feature_type="http"\n\`\`\`\n\n`;
      }
    }

    // Use extracted variable syntax from CLI introspection
    if (cliData?.usage?.syntax_patterns?.variables) {
      response += `### Variable Types (from Documentation)\n\n`;
      const vars = cliData.usage.syntax_patterns.variables;
      
      if (vars.config && vars.config.length > 0) {
        response += `- **Config variables**: ${vars.config.slice(0, 2).map(v => `\`${v}\``).join(', ')}\n`;
      }
      if (vars.environment && vars.environment.length > 0) {
        response += `- **Environment variables**: ${vars.environment.slice(0, 2).map(v => `\`${v}\``).join(', ')}\n`;
      }
      if (vars.runtime && vars.runtime.length > 0) {
        response += `- **Runtime variables**: ${vars.runtime.slice(0, 2).map(v => `\`${v}\``).join(', ')}\n`;
      }
      response += `\n`;
    }

    // Use extracted CLI commands from introspection
    response += `## CLI Commands (from Current CLI)\n\n`;
    if (cliData?.usage?.common_patterns) {
      response += `\`\`\`bash\n`;
      
      // Find validate patterns
      const validatePatterns = cliData.usage.common_patterns.filter((p: any) => p.command.includes('validate'));
      if (validatePatterns.length > 0) {
        response += `# ${validatePatterns[0].description}\n`;
        response += `${validatePatterns[0].command}\n\n`;
      }
      
      // Find run patterns
      const runPatterns = cliData.usage.common_patterns.filter((p: any) => p.command.includes('run'));
      runPatterns.slice(0, 3).forEach((pattern: any) => {
        response += `# ${pattern.description}\n`;
        response += `${pattern.command}\n\n`;
      });
      
      response += `\`\`\`\n\n`;
    }

    // Use extracted patterns for key reminders
    response += `## Key Information (from Documentation)\n`;
    if (cliData?.usage?.file_structure?.pattern) {
      response += `- File pattern: \`${cliData.usage.file_structure.pattern}\`\n`;
    }
    if (cliData?.usage?.syntax_patterns?.save_operations) {
      const saveExample = Object.values(cliData.usage.syntax_patterns.save_operations)[0] as string;
      if (saveExample) {
        response += `- Save syntax: See documentation for \`save:\` operations\n`;
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

    // Use actual CLI help data and usage patterns from introspection
    if (command === "run" || command === "structure") {
      response += `## Running Tests\n\n`;
      
      if (cliData?.help?.run?.help) {
        response += `### Current CLI Usage\n`;
        response += `\`\`\`\n${cliData.help.run.help}\`\`\`\n\n`;
      }

      // Use extracted usage patterns instead of hard-coded ones
      if (cliData?.usage?.common_patterns) {
        response += `### Common Patterns\n`;
        response += `\`\`\`bash\n`;
        
        cliData.usage.common_patterns.forEach((pattern: any) => {
          response += `# ${pattern.description}\n`;
          response += `${pattern.command}\n\n`;
        });
        
        response += `\`\`\`\n\n`;
      }

      // Use extracted examples from CLI help
      if (cliData?.usage?.examples_from_help?.run) {
        response += `### Examples from CLI Help\n`;
        response += `\`\`\`\n${cliData.usage.examples_from_help.run}\`\`\`\n\n`;
      }

      // Manual server mode using actual CLI data
      if (cliData?.help?.start?.subcommands?.server) {
        response += `### Manual Engine Mode\n`;
        response += `\`\`\`\n${cliData.help.start.subcommands.server}\`\`\`\n\n`;
      }
    }

    if (command === "validate" || command === "structure") {
      response += `## Validation\n\n`;
      
      if (cliData?.help?.validate?.help) {
        response += `### Current Validate Command\n`;
        response += `\`\`\`\n${cliData.help.validate.help}\`\`\`\n\n`;
      }

      // Use extracted examples if available
      if (cliData?.usage?.examples_from_help?.validate) {
        response += `### Examples from CLI Help\n`;
        response += `\`\`\`\n${cliData.usage.examples_from_help.validate}\`\`\`\n\n`;
      }
    }

    // Use actual flag data - NO FALLBACKS
    response += `## Current CLI Flags\n\n`;
    if (cliData?.flags && Object.keys(cliData.flags).length > 0) {
      for (const [flag, info] of Object.entries(cliData.flags as any)) {
        const flagInfo = info as any;
        response += `- \`${flagInfo.short ? flagInfo.short + ', ' : ''}${flagInfo.long}\`: ${flagInfo.description}\n`;
      }
    } else {
      response += `Flag documentation not available in CLI introspection data.\n`;
    }
    response += `\n`;

    // Use actual installation data - NO FALLBACKS
    response += `## Installation Methods\n\n`;
    if (cliData?.installation) {
      if (cliData.installation.recommended) {
        response += `**Recommended:** ${cliData.installation.recommended}\n\n`;
      }
      
      if (cliData.installation.methods && cliData.installation.methods.length > 0) {
        response += `**Available Methods:**\n`;
        for (const method of cliData.installation.methods) {
          response += `- **${method.name}**: ${method.description}\n`;
          response += `  \`\`\`bash\n  ${method.command}\n  \`\`\`\n`;
        }
      }
      
      if (cliData.installation.notAvailable && cliData.installation.notAvailable.length > 0) {
        response += `\n**NOT Available:**\n`;
        for (const na of cliData.installation.notAvailable) {
          response += `- ${na}\n`;
        }
      }

      // Include actual README installation section if available
      if (cliData.installation.fromReadme) {
        response += `\n### From Documentation\n\n`;
        response += `${cliData.installation.fromReadme}\n\n`;
      }
    } else {
      response += `Installation information not available in CLI introspection data.\n\n`;
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
