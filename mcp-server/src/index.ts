#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import * as fs from "fs";
import * as path from "path";

// Dynamic schema and examples loader
class RocketshipKnowledgeLoader {
  private projectRoot: string;
  private schema: any = null;
  private examples: Map<string, any> = new Map();
  private docs: Map<string, string> = new Map();

  constructor() {
    try {
      // Find project root by looking for schema.json
      this.projectRoot = this.findProjectRoot();
      console.log(`Initializing Rocketship MCP Server from project root: ${this.projectRoot}`);
      
      this.loadSchema();
      console.log(`✓ Loaded schema from ${this.projectRoot}/internal/dsl/schema.json`);
      
      this.loadExamples();
      console.log(`✓ Loaded ${this.examples.size} examples`);
      
      this.loadDocumentation();
      console.log(`✓ Loaded ${this.docs.size} documentation files`);
    } catch (error) {
      console.error(`Failed to initialize RocketshipKnowledgeLoader: ${error}`);
      throw error;
    }
  }

  private findProjectRoot(): string {
    let currentDir = process.cwd();
    
    // Look for schema.json to identify project root
    while (currentDir !== path.dirname(currentDir)) {
      const schemaPath = path.join(currentDir, "internal", "dsl", "schema.json");
      if (fs.existsSync(schemaPath)) {
        return currentDir;
      }
      currentDir = path.dirname(currentDir);
    }
    
    throw new Error(`Cannot find project root. No schema.json found in any parent directory from ${process.cwd()}`);
  }

  private loadSchema(): void {
    const schemaPath = path.join(this.projectRoot, "internal", "dsl", "schema.json");
    if (!fs.existsSync(schemaPath)) {
      throw new Error(`Schema file not found at ${schemaPath}. Cannot initialize MCP server without schema.`);
    }
    const schemaContent = fs.readFileSync(schemaPath, "utf-8");
    this.schema = JSON.parse(schemaContent);
  }

  private loadExamples(): void {
    const examplesDir = path.join(this.projectRoot, "examples");
    if (!fs.existsSync(examplesDir)) {
      throw new Error(`Examples directory not found at ${examplesDir}. Cannot initialize MCP server without examples.`);
    }

    const subdirs = fs.readdirSync(examplesDir, { withFileTypes: true })
      .filter(dirent => dirent.isDirectory())
      .map(dirent => dirent.name);

    if (subdirs.length === 0) {
      throw new Error(`No example subdirectories found in ${examplesDir}`);
    }

    for (const subdir of subdirs) {
      const yamlPath = path.join(examplesDir, subdir, "rocketship.yaml");
      if (fs.existsSync(yamlPath)) {
        const content = fs.readFileSync(yamlPath, "utf-8");
        this.examples.set(subdir, { content, path: yamlPath });
      }
    }

    if (this.examples.size === 0) {
      throw new Error(`No rocketship.yaml files found in example directories`);
    }
  }

  private loadDocumentation(): void {
    // Load reference docs
    const refDir = path.join(this.projectRoot, "docs", "src", "reference");
    if (!fs.existsSync(refDir)) {
      throw new Error(`Reference documentation directory not found at ${refDir}`);
    }
    
    const refFiles = fs.readdirSync(refDir).filter(f => f.endsWith(".md"));
    if (refFiles.length === 0) {
      throw new Error(`No reference documentation files found in ${refDir}`);
    }
    
    for (const file of refFiles) {
      const content = fs.readFileSync(path.join(refDir, file), "utf-8");
      this.docs.set(`reference/${file}`, content);
    }

    // Load example docs
    const exampleDocsDir = path.join(this.projectRoot, "docs", "src", "examples");
    if (!fs.existsSync(exampleDocsDir)) {
      throw new Error(`Example documentation directory not found at ${exampleDocsDir}`);
    }
    
    const exampleFiles = fs.readdirSync(exampleDocsDir).filter(f => f.endsWith(".md"));
    if (exampleFiles.length === 0) {
      throw new Error(`No example documentation files found in ${exampleDocsDir}`);
    }
    
    for (const file of exampleFiles) {
      const content = fs.readFileSync(path.join(exampleDocsDir, file), "utf-8");
      this.docs.set(`examples/${file}`, content);
    }
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

}

// Initialize knowledge loader
const knowledgeLoader = new RocketshipKnowledgeLoader();

// Tool descriptions that emphasize the assistant nature
const TOOL_DESCRIPTIONS = {
  get_rocketship_examples: `Provides real examples from the Rocketship codebase for specific features or use cases.

💡 YOU (the coding agent) create the test files based on these examples.
💡 For frontend projects, consider using the browser plugin for user journey testing.
💡 Structure: .rocketship/ directory with subdirectories, each containing rocketship.yaml
💡 Syntax: Use {{ variable_name }} for variables, .field_name for JSON paths (no $ prefix)`,

  suggest_test_structure: `Suggests proper Rocketship file structure and test organization for your project.

💡 YOU (the coding agent) create the directory structure and files.
💡 For frontend projects: Consider browser-based user journey testing.
💡 For API projects: Focus on user workflows rather than just coverage.`,

  get_schema_info: `Provides the current Rocketship schema information for validation and proper syntax.

💡 Use this to ensure your YAML follows the correct schema.
💡 Pay attention to required fields, valid plugin names, and assertion types.`,

  get_cli_guidance: `Provides correct Rocketship CLI usage patterns and commands.

💡 YOU (the coding agent) will run these commands to execute tests.
💡 Use rocketship run -af for auto-start, rocketship run -ad for directories.`,

  analyze_codebase_for_testing: `Analyzes a codebase to suggest meaningful test scenarios based on user journeys.

💡 Focus on customer-facing flows and critical business logic.
💡 For frontends: Consider browser testing of key user paths.
💡 For APIs: Test the endpoints that support those user paths.`,
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
                  enum: ["browser", "http", "sql", "agent", "supabase", "delay", "log", "script"],
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
                  description: "Key user journeys to test (e.g., 'user registration', 'purchase flow')",
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
                  description: "Description of the codebase structure and functionality",
                },
                focus_area: {
                  type: "string",
                  enum: ["user_journeys", "api_endpoints", "critical_paths", "integration_points"],
                  description: "What aspect to focus testing on",
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
    
    // Get real examples from codebase
    const allExamples = this.knowledgeLoader.getAllExamples();
    const relevantExamples = allExamples.filter(name => 
      name.includes(feature_type) || 
      (feature_type === "browser" && name.includes("browser")) ||
      (feature_type === "http" && (name.includes("http") || name.includes("request"))) ||
      (feature_type === "sql" && name.includes("sql")) ||
      (feature_type === "agent" && name.includes("agent"))
    );

    let response = `# Real Rocketship Examples for ${feature_type}\n\n`;
    
    if (use_case) {
      response += `Use case: ${use_case}\n\n`;
    }

    // Add suggestions for frontend projects
    if (feature_type === "browser" || use_case?.toLowerCase().includes("frontend")) {
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
    
    for (const exampleName of relevantExamples.slice(0, 3)) {
      const example = this.knowledgeLoader.getExample(exampleName);
      if (example) {
        response += `### ${exampleName}\n\n`;
        response += `\`\`\`yaml\n${example.content}\`\`\`\n\n`;
      }
    }

    // Add syntax reminders
    response += `## Critical Syntax Rules\n\n`;
    response += `- Variables: \`{{ variable_name }}\` (NOT \`{{.vars.variable_name}}\`)\n`;
    response += `- JSON paths: \`.field_name\` or \`.items_0.id\` (NO $ prefix)\n`;
    response += `- File names: Always \`rocketship.yaml\` in subdirectories\n`;
    response += `- Step chaining: Save with \`json_path: ".field"\` and \`as: "var_name"\`\n\n`;

    // Add browser-specific guidance
    if (feature_type === "browser") {
      response += `## Browser Plugin Requirements\n\n`;
      response += `- \`task\`: Natural language description of what to do\n`;
      response += `- \`llm\`: Configuration with provider and model\n`;
      response += `- Common tasks: "Navigate to X and Y", "Fill form and submit", "Extract data from page"\n\n`;
      
      const browserDoc = this.knowledgeLoader.getDocumentation("examples/browser-testing.md");
      if (browserDoc) {
        // Extract key sections from browser documentation
        const configMatch = browserDoc.match(/## Basic Configuration\n\n```yaml\n([\s\S]*?)\n```/);
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
          const cleanName = flow.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "");
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

      response += `### Example Frontend Test Structure\n\n`;
      response += `\`\`\`yaml\n`;
      response += `name: "User Login Journey"\n`;
      response += `version: "v1.0.0"\n`;
      response += `\n`;
      response += `tests:\n`;
      response += `  - name: "Complete login flow"\n`;
      response += `    steps:\n`;
      response += `      - name: "Navigate and login"\n`;
      response += `        plugin: browser\n`;
      response += `        config:\n`;
      response += `          task: |\n`;
      response += `            1. Navigate to {{ app_url }}/login\n`;
      response += `            2. Fill in email: test@example.com\n`;
      response += `            3. Fill in password: testpass123\n`;
      response += `            4. Click login button\n`;
      response += `            5. Verify you reach the dashboard\n`;
      response += `          llm:\n`;
      response += `            provider: "openai"\n`;
      response += `            model: "gpt-4o"\n`;
      response += `            config:\n`;
      response += `              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"\n`;
      response += `          headless: true\n`;
      response += `          timeout: "2m"\n`;
      response += `        save:\n`;
      response += `          - json_path: ".success"\n`;
      response += `            as: "login_success"\n`;
      response += `        assertions:\n`;
      response += `          - type: "json_path"\n`;
      response += `            path: ".success"\n`;
      response += `            expected: true\n`;
      response += `\`\`\`\n\n`;
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
    response += `- Variables: \`{{ variable_name }}\` format\n`;
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
      const pluginEnum = schema?.properties?.tests?.items?.properties?.steps?.items?.properties?.plugin?.enum;
      if (pluginEnum) {
        for (const plugin of pluginEnum) {
          response += `- **${plugin}**: `;
          switch (plugin) {
            case "browser":
              response += `AI-powered browser automation for frontend testing\n`;
              break;
            case "http":
              response += `HTTP requests for API testing\n`;
              break;
            case "sql":
              response += `Database queries and validation\n`;
              break;
            case "agent":
              response += `AI agent interactions for complex testing\n`;
              break;
            case "delay":
              response += `Wait/pause between test steps\n`;
              break;
            case "log":
              response += `Output messages and debugging\n`;
              break;
            case "script":
              response += `Custom JavaScript or shell scripts\n`;
              break;
            case "supabase":
              response += `Supabase database operations\n`;
              break;
            default:
              response += `Plugin for ${plugin} operations\n`;
          }
        }
      }
      response += `\n`;
    }

    if (section === "assertions" || section === "full") {
      response += `## Valid Assertion Types\n\n`;
      const assertionTypes = schema?.properties?.tests?.items?.properties?.steps?.items?.properties?.assertions?.items?.properties?.type?.enum;
      if (assertionTypes) {
        for (const type of assertionTypes) {
          response += `- **${type}**: `;
          switch (type) {
            case "status_code":
              response += `HTTP status code validation\n`;
              break;
            case "json_path":
              response += `JSON field validation using .field.path syntax\n`;
              break;
            case "header":
              response += `HTTP response header validation\n`;
              break;
            default:
              response += `${type} validation\n`;
          }
        }
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
      response += `Use saved variables: \`{{ variable_name }}\`\n\n`;
    }

    if (section === "structure" || section === "full") {
      response += `## Required File Structure\n\n`;
      response += `\`\`\`yaml\n`;
      response += `name: "Test Suite Name"          # Required\n`;
      response += `version: "v1.0.0"                # Required: v1.0.0 format\n`;
      response += `description: "Optional description"\n`;
      response += `\n`;
      response += `vars:                            # Optional\n`;
      response += `  app_url: "{{ .env.APP_URL }}"\n`;
      response += `\n`;
      response += `tests:                           # Required: array\n`;
      response += `  - name: "Test name"            # Required\n`;
      response += `    steps:                       # Required: array\n`;
      response += `      - name: "Step name"        # Required\n`;
      response += `        plugin: "browser"        # Required: valid plugin\n`;
      response += `        config:                  # Required: plugin config\n`;
      response += `          # plugin-specific config\n`;
      response += `        assertions:              # Optional: array\n`;
      response += `          - type: "json_path"    # Required if assertions\n`;
      response += `            path: ".success"     # Required for json_path\n`;
      response += `            expected: true       # Required\n`;
      response += `        save:                    # Optional: array\n`;
      response += `          - json_path: ".result" # One of: json_path, header, sql_result\n`;
      response += `            as: "result_data"    # Required\n`;
      response += `\`\`\`\n\n`;
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

    let response = `# Rocketship CLI Guidance\n\n`;

    if (command === "run" || command === "structure") {
      response += `## Running Tests\n\n`;
      response += `### Auto-start mode (recommended for development)\n`;
      response += `\`\`\`bash\n`;
      response += `# Run single test file with auto-start/stop\n`;
      response += `rocketship run -af .rocketship/login-flow/rocketship.yaml\n`;
      response += `\n`;
      response += `# Run all tests in directory with auto-start/stop\n`;
      response += `rocketship run -ad .rocketship\n`;
      response += `\n`;
      response += `# Run with environment variables\n`;
      response += `rocketship run -ad .rocketship --var APP_URL=http://localhost:3000\n`;
      response += `\`\`\`\n\n`;

      response += `### Manual engine mode\n`;
      response += `\`\`\`bash\n`;
      response += `# Start engine in background\n`;
      response += `rocketship start server --local --background\n`;
      response += `\n`;
      response += `# Run tests against running engine\n`;
      response += `rocketship run -d .rocketship\n`;
      response += `\n`;
      response += `# Stop engine\n`;
      response += `rocketship stop server\n`;
      response += `\`\`\`\n\n`;
    }

    if (command === "validate" || command === "structure") {
      response += `## Validation\n\n`;
      response += `\`\`\`bash\n`;
      response += `# Validate single file\n`;
      response += `rocketship validate .rocketship/login-flow/rocketship.yaml\n`;
      response += `\n`;
      response += `# Validate all files in directory\n`;
      response += `rocketship validate .rocketship\n`;
      response += `\n`;
      response += `# Validate with variables\n`;
      response += `rocketship validate .rocketship --var APP_URL=http://localhost:3000\n`;
      response += `\`\`\`\n\n`;
    }

    response += `## Key Flags\n\n`;
    response += `- \`-a, --auto\`: Auto-start and stop engine\n`;
    response += `- \`-f, --file\`: Single test file\n`;
    response += `- \`-d, --dir\`: Directory of tests\n`;
    response += `- \`--var key=value\`: Set variables\n`;
    response += `- \`--var-file path\`: Load variables from file\n\n`;

    response += `## File Discovery\n`;
    response += `Rocketship automatically finds \`rocketship.yaml\` files recursively in directories.\n\n`;

    const runDoc = this.knowledgeLoader.getDocumentation("reference/rocketship_run.md");
    if (runDoc) {
      response += `## Complete CLI Reference\n\n`;
      response += runDoc;
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
    const { codebase_info, focus_area } = args;

    let response = `# Test Strategy Analysis\n\n`;
    response += `Codebase: ${codebase_info}\n\n`;

    // Detect if this is a frontend project
    const isFrontend = codebase_info.toLowerCase().includes("react") || 
                      codebase_info.toLowerCase().includes("vue") || 
                      codebase_info.toLowerCase().includes("frontend") ||
                      codebase_info.toLowerCase().includes("client") ||
                      codebase_info.toLowerCase().includes("ui");

    if (isFrontend && focus_area === "user_journeys") {
      response += `💡 **Frontend Detected - Browser Testing Recommended**\n\n`;
      response += `## Critical User Journeys to Test\n\n`;
      
      // Extract potential user flows from description
      const flows = this.extractUserFlows(codebase_info);
      
      for (let i = 0; i < flows.length; i++) {
        const flow = flows[i];
        const dirName = flow.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "");
        
        response += `### ${i + 1}. ${flow}\n\n`;
        response += `**Directory:** \`.rocketship/${dirName}/rocketship.yaml\`\n\n`;
        response += `**Browser Test Strategy:**\n`;
        response += `\`\`\`yaml\n`;
        response += `name: "${flow} Journey"\n`;
        response += `version: "v1.0.0"\n`;
        response += `\n`;
        response += `tests:\n`;
        response += `  - name: "${flow} E2E Test"\n`;
        response += `    steps:\n`;
        response += `      - name: "Execute ${flow.toLowerCase()}"\n`;
        response += `        plugin: browser\n`;
        response += `        config:\n`;
        response += `          task: |\n`;
        response += `            ${this.generateBrowserTask(flow)}\n`;
        response += `          llm:\n`;
        response += `            provider: "openai"\n`;
        response += `            model: "gpt-4o"\n`;
        response += `            config:\n`;
        response += `              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"\n`;
        response += `          headless: true\n`;
        response += `          timeout: "3m"\n`;
        response += `        save:\n`;
        response += `          - json_path: ".success"\n`;
        response += `            as: "flow_success"\n`;
        response += `          - json_path: ".result"\n`;
        response += `            as: "flow_result"\n`;
        response += `        assertions:\n`;
        response += `          - type: "json_path"\n`;
        response += `            path: ".success"\n`;
        response += `            expected: true\n`;
        response += `\`\`\`\n\n`;
      }
    } else if (focus_area === "api_endpoints") {
      response += `## API Testing Strategy\n\n`;
      response += `Focus on endpoints that support user journeys, not just coverage.\n\n`;
      
      response += `### Health & Authentication\n`;
      response += `\`\`\`yaml\n`;
      response += `- name: "API Health Check"\n`;
      response += `  plugin: http\n`;
      response += `  config:\n`;
      response += `    method: GET\n`;
      response += `    url: "{{ base_url }}/health"\n`;
      response += `  assertions:\n`;
      response += `    - type: status_code\n`;
      response += `      expected: 200\n`;
      response += `\`\`\`\n\n`;
    }

    response += `## Recommended Test Structure\n\n`;
    response += `\`\`\`\n`;
    response += `.rocketship/\n`;
    
    if (isFrontend) {
      const flows = this.extractUserFlows(codebase_info);
      for (const flow of flows.slice(0, 5)) {
        const dirName = flow.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "");
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
    const commonFlows = [
      "User Registration",
      "Login Flow", 
      "Dashboard Navigation",
      "Settings Management",
      "Search Functionality",
      "Data Entry",
      "Purchase Flow",
      "Profile Management",
      "Content Creation",
      "Report Generation"
    ];

    // Extract flows based on keywords in description
    const flows: string[] = [];
    const lowerDesc = description.toLowerCase();

    if (lowerDesc.includes("auth") || lowerDesc.includes("login") || lowerDesc.includes("sign")) {
      flows.push("User Authentication");
    }
    if (lowerDesc.includes("dashboard") || lowerDesc.includes("admin")) {
      flows.push("Dashboard Navigation");
    }
    if (lowerDesc.includes("search")) {
      flows.push("Search Functionality");
    }
    if (lowerDesc.includes("profile") || lowerDesc.includes("user")) {
      flows.push("Profile Management");
    }
    if (lowerDesc.includes("purchase") || lowerDesc.includes("checkout") || lowerDesc.includes("payment")) {
      flows.push("Purchase Flow");
    }
    if (lowerDesc.includes("fleet") || lowerDesc.includes("vehicle") || lowerDesc.includes("management")) {
      flows.push("Fleet Management");
    }

    // Add common flows if none detected
    if (flows.length === 0) {
      flows.push(...commonFlows.slice(0, 3));
    }

    return flows.slice(0, 5); // Limit to 5 flows
  }

  private generateBrowserTask(flow: string): string {
    const lowerFlow = flow.toLowerCase();
    
    if (lowerFlow.includes("login") || lowerFlow.includes("auth")) {
      return `            1. Navigate to {{ app_url }}/login
            2. Fill in email: test@example.com
            3. Fill in password: testpass123
            4. Click login button
            5. Verify successful login and dashboard access`;
    } else if (lowerFlow.includes("registration") || lowerFlow.includes("signup")) {
      return `            1. Navigate to {{ app_url }}/register
            2. Fill in registration form with test data
            3. Submit the form
            4. Verify account creation success`;
    } else if (lowerFlow.includes("dashboard")) {
      return `            1. Navigate to {{ app_url }}/dashboard
            2. Verify main dashboard elements load
            3. Check key metrics and data display
            4. Test navigation to main sections`;
    } else if (lowerFlow.includes("search")) {
      return `            1. Navigate to {{ app_url }}
            2. Use search functionality with test query
            3. Verify search results display
            4. Test result filtering and navigation`;
    } else if (lowerFlow.includes("purchase") || lowerFlow.includes("checkout")) {
      return `            1. Navigate to {{ app_url }}/products
            2. Select a product and add to cart
            3. Go to checkout
            4. Complete purchase flow with test data
            5. Verify order confirmation`;
    } else {
      return `            1. Navigate to {{ app_url }}
            2. Test the ${flow.toLowerCase()} functionality
            3. Verify expected behavior and results
            4. Check for any errors or issues`;
    }
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