#!/usr/bin/env node

import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { CallToolRequestSchema, ListToolsRequestSchema } from '@modelcontextprotocol/sdk/types.js';
import { spawn } from 'child_process';
import { promises as fs } from 'fs';
import * as path from 'path';
import * as yaml from 'js-yaml';
import simpleGit from 'simple-git';

interface CodebaseAnalysis {
  api_endpoints: Array<{
    method: string;
    path: string;
    description?: string;
  }>;
  database_schemas: Array<{
    table: string;
    columns: string[];
    primary_key?: string;
  }>;
  service_configs: Array<{
    name: string;
    type: string;
  }>;
  environment_files: string[];
}

export class RocketshipMCPServer {
  private server: Server;

  constructor() {
    this.server = new Server(
      {
        name: 'rocketship-mcp',
        version: '0.1.0',
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
            name: 'scan_and_generate_test_suite',
            description: 'Analyze codebase context and generate comprehensive test suite structure',
            inputSchema: {
              type: 'object',
              properties: {
                project_root: {
                  type: 'string',
                  description: 'Root directory of the project',
                  default: '.',
                },
                environments: {
                  type: 'array',
                  items: { type: 'string' },
                  description: 'Target environments',
                  default: ['staging', 'prod'],
                },
                codebase_analysis: {
                  type: 'object',
                  description: 'Analysis of the codebase from agent context',
                  properties: {
                    api_endpoints: {
                      type: 'array',
                      items: { type: 'object' },
                    },
                    database_schemas: {
                      type: 'array',
                      items: { type: 'object' },
                    },
                    service_configs: {
                      type: 'array',
                      items: { type: 'object' },
                    },
                  },
                },
              },
              required: ['codebase_analysis'],
            },
          },
          {
            name: 'generate_test_from_prompt',
            description: 'Generate a Rocketship test file from natural language prompt',
            inputSchema: {
              type: 'object',
              properties: {
                prompt: {
                  type: 'string',
                  description: 'Natural language description of the test',
                },
                test_type: {
                  type: 'string',
                  enum: ['api', 'database', 'integration', 'auth'],
                  default: 'api',
                },
                output_path: {
                  type: 'string',
                  default: '.rocketship/generated-test.yaml',
                },
              },
              required: ['prompt'],
            },
          },
          {
            name: 'validate_test_file',
            description: 'Validate a Rocketship test file using the CLI',
            inputSchema: {
              type: 'object',
              properties: {
                file_path: {
                  type: 'string',
                  description: 'Path to the test file',
                },
              },
              required: ['file_path'],
            },
          },
          {
            name: 'run_and_analyze_tests',
            description: 'Execute tests and provide intelligent analysis',
            inputSchema: {
              type: 'object',
              properties: {
                file_path: {
                  type: 'string',
                  description: 'Path to the test file',
                },
                environment: {
                  type: 'string',
                  default: 'staging',
                },
                var_file: {
                  type: 'string',
                  description: 'Path to variable file',
                },
              },
              required: ['file_path'],
            },
          },
          {
            name: 'analyze_git_diff',
            description: 'Analyze git changes and suggest test updates',
            inputSchema: {
              type: 'object',
              properties: {
                base_branch: {
                  type: 'string',
                  default: 'main',
                },
                feature_branch: {
                  type: 'string',
                  default: 'HEAD',
                },
              },
            },
          },
        ],
      };
    });

    this.server.setRequestHandler(CallToolRequestSchema, async (request) => {
      const { name, arguments: args } = request.params;

      try {
        switch (name) {
          case 'scan_and_generate_test_suite':
            return await this.handleScanAndGenerate(args);
          case 'generate_test_from_prompt':
            return await this.handleGenerateFromPrompt(args);
          case 'validate_test_file':
            return await this.handleValidateTest(args);
          case 'run_and_analyze_tests':
            return await this.handleRunTests(args);
          case 'analyze_git_diff':
            return await this.handleGitDiff(args);
          default:
            throw new Error(`Unknown tool: ${name}`);
        }
      } catch (error) {
        return {
          content: [
            {
              type: 'text',
              text: `Error: ${error instanceof Error ? error.message : String(error)}`,
            },
          ],
        };
      }
    });
  }

  private async handleScanAndGenerate(args: any) {
    const { project_root = '.', environments = ['staging', 'prod'], codebase_analysis } = args;
    
    // Generate test structure based on analysis
    const structure = this.generateTestStructure(codebase_analysis, environments);
    
    // Create .rocketship directory
    const rocketshipDir = path.join(project_root, '.rocketship');
    await fs.mkdir(rocketshipDir, { recursive: true });
    
    const createdFiles: string[] = [];
    
    // Create environment files
    for (const [filename, config] of Object.entries(structure.environmentVars)) {
      const filePath = path.join(rocketshipDir, filename);
      await fs.writeFile(filePath, yaml.dump(config));
      createdFiles.push(filePath);
    }
    
    // Create test directories and files
    for (const dir of structure.directories) {
      await fs.mkdir(path.join(rocketshipDir, dir), { recursive: true });
    }
    
    for (const [filePath, content] of Object.entries(structure.files)) {
      const fullPath = path.join(rocketshipDir, filePath);
      await fs.mkdir(path.dirname(fullPath), { recursive: true });
      await fs.writeFile(fullPath, yaml.dump(content));
      createdFiles.push(fullPath);
    }
    
    return {
      content: [
        {
          type: 'text',
          text: `✅ Successfully generated Rocketship test suite structure!\n\n` +
                `📁 Created directories: ${structure.directories.join(', ')}\n` +
                `📄 Generated ${createdFiles.length} files\n` +
                `🌍 Environment configs: ${Object.keys(structure.environmentVars).join(', ')}\n\n` +
                `Next steps:\n` +
                `• Review generated environment variable files and update with actual values\n` +
                `• Customize test cases based on your specific API endpoints\n` +
                `• Run 'rocketship validate' on the generated test files\n` +
                `• Execute tests with: rocketship run -af .rocketship/[test-suite]/rocketship.yaml`,
        },
      ],
    };
  }

  private async handleGenerateFromPrompt(args: any) {
    const { prompt, test_type = 'api', output_path = '.rocketship/generated-test.yaml' } = args;
    
    const testYaml = this.generateTestFromPrompt(prompt, test_type);
    
    // Ensure directory exists
    await fs.mkdir(path.dirname(output_path), { recursive: true });
    await fs.writeFile(output_path, testYaml);
    
    return {
      content: [
        {
          type: 'text',
          text: `✅ Successfully generated test from prompt!\n\n` +
                `📝 Prompt: ${prompt}\n` +
                `📁 Output: ${output_path}\n` +
                `🎯 Type: ${test_type}\n\n` +
                `Next steps:\n` +
                `• Review and customize the generated test\n` +
                `• Validate with: rocketship validate ${output_path}\n` +
                `• Run with: rocketship run -af ${output_path}`,
        },
      ],
    };
  }

  private async handleValidateTest(args: any) {
    const { file_path } = args;
    
    try {
      await fs.access(file_path);
    } catch {
      return {
        content: [
          {
            type: 'text',
            text: `❌ Test file not found: ${file_path}`,
          },
        ],
      };
    }
    
    const result = await this.executeRocketshipCommand(['validate', file_path]);
    
    if (result.success) {
      return {
        content: [
          {
            type: 'text',
            text: `✅ Test file validation successful!\n\n📁 File: ${file_path}\n✨ Status: Valid and ready to run`,
          },
        ],
      };
    } else {
      return {
        content: [
          {
            type: 'text',
            text: `❌ Test file validation failed!\n\n📁 File: ${file_path}\n\nErrors:\n${result.output}`,
          },
        ],
      };
    }
  }

  private async handleRunTests(args: any) {
    const { file_path, environment = 'staging', var_file } = args;
    
    const cmd = ['run', '-af', file_path];
    if (var_file) {
      cmd.push('--var-file', var_file);
    }
    
    const result = await this.executeRocketshipCommand(cmd);
    
    if (result.success) {
      return {
        content: [
          {
            type: 'text',
            text: `✅ Test execution successful!\n\n📁 File: ${file_path}\n🌍 Environment: ${environment}\n\n${result.output.slice(0, 500)}...`,
          },
        ],
      };
    } else {
      const analysis = this.analyzeTestFailures(result.output);
      return {
        content: [
          {
            type: 'text',
            text: `❌ Test execution failed!\n\n📁 File: ${file_path}\n🌍 Environment: ${environment}\n\nAnalysis: ${analysis}\n\nOutput:\n${result.output.slice(0, 1000)}...`,
          },
        ],
      };
    }
  }

  private async handleGitDiff(args: any) {
    const { base_branch = 'main', feature_branch = 'HEAD' } = args;
    
    try {
      const git = simpleGit();
      const diff = await git.diff([`${base_branch}...${feature_branch}`, '--name-status']);
      
      const changes = this.parseGitDiff(diff);
      const suggestions = this.generateTestSuggestions(changes);
      
      return {
        content: [
          {
            type: 'text',
            text: `📊 Git Diff Analysis Complete!\n\n` +
                  `🔀 Comparing: ${base_branch}...${feature_branch}\n` +
                  `📁 Found ${changes.length} changed files\n\n` +
                  `Test Suggestions:\n${suggestions.join('\n')}\n\n` +
                  `💡 These suggestions are based on file change patterns. Human review recommended.`,
          },
        ],
      };
    } catch (error) {
      return {
        content: [
          {
            type: 'text',
            text: `❌ Git analysis failed: ${error instanceof Error ? error.message : String(error)}`,
          },
        ],
      };
    }
  }

  // Helper methods
  private generateTestStructure(analysis: CodebaseAnalysis, environments: string[]) {
    const structure = {
      directories: [] as string[],
      files: {} as Record<string, any>,
      environmentVars: {} as Record<string, any>,
    };

    // Generate environment files
    for (const env of environments) {
      structure.environmentVars[`${env}-vars.yaml`] = {
        base_url: `https://api-${env}.example.com`,
        timeout: 30,
        auth: {
          api_key: `{{ .env.API_KEY_${env.toUpperCase()} }}`,
          bearer_token: `{{ .env.BEARER_TOKEN_${env.toUpperCase()} }}`,
        },
      };
    }

    // Generate test suites based on analysis
    if (analysis.api_endpoints?.length > 0) {
      structure.directories.push('api-tests');
      structure.files['api-tests/rocketship.yaml'] = this.generateAPITestSuite(analysis);
    }

    if (analysis.database_schemas?.length > 0) {
      structure.directories.push('database-tests');
      structure.files['database-tests/rocketship.yaml'] = this.generateDatabaseTestSuite(analysis);
    }

    structure.directories.push('integration-tests');
    structure.files['integration-tests/rocketship.yaml'] = this.generateIntegrationTestSuite();

    return structure;
  }

  private generateAPITestSuite(analysis: CodebaseAnalysis) {
    return {
      name: 'API Integration Tests',
      description: 'Comprehensive API endpoint testing',
      vars: {
        api_base: '{{ .vars.base_url }}',
        auth_header: '{{ .vars.auth.api_key }}',
      },
      tests: [
        {
          name: 'API Health Check',
          steps: [
            {
              name: 'Health endpoint',
              plugin: 'http',
              config: {
                method: 'GET',
                url: '{{ .vars.api_base }}/health',
                timeout: '{{ .vars.timeout }}s',
              },
              assertions: [
                {
                  type: 'status_code',
                  expected: 200,
                },
              ],
            },
          ],
        },
      ],
    };
  }

  private generateDatabaseTestSuite(analysis: CodebaseAnalysis) {
    return {
      name: 'Database Tests',
      description: 'Database connectivity and schema validation',
      tests: [
        {
          name: 'Database Connection',
          steps: [
            {
              name: 'Test connection',
              plugin: 'sql',
              config: {
                driver: 'postgres',
                dsn: '{{ .vars.db_dsn }}',
                commands: ['SELECT 1 as test_connection;'],
              },
              assertions: [
                {
                  type: 'row_count',
                  query_index: 0,
                  expected: 1,
                },
              ],
            },
          ],
        },
      ],
    };
  }

  private generateIntegrationTestSuite() {
    return {
      name: 'Integration Tests',
      description: 'End-to-end workflow testing',
      tests: [
        {
          name: 'Basic Integration Flow',
          steps: [
            {
              name: 'System health check',
              plugin: 'http',
              config: {
                method: 'GET',
                url: '{{ .vars.api_base }}/health',
              },
              assertions: [
                {
                  type: 'status_code',
                  expected: 200,
                },
              ],
            },
          ],
        },
      ],
    };
  }

  private generateTestFromPrompt(prompt: string, testType: string): string {
    const testConfig = {
      name: this.generateTestName(prompt),
      description: `Generated from prompt: ${prompt}`,
      vars: {
        base_url: '{{ .vars.base_url }}',
        timeout: 30,
        auth_token: '{{ .vars.auth.bearer_token }}',
      },
      tests: [
        {
          name: `Test ${this.generateTestName(prompt).replace(' Tests', '')}`,
          steps: [
            {
              name: 'Generated test step',
              plugin: this.inferPlugin(prompt, testType),
              config: this.generateTestConfig(prompt, testType),
              assertions: [
                {
                  type: 'status_code',
                  expected: 200,
                },
              ],
            },
          ],
        },
      ],
    };

    return yaml.dump(testConfig);
  }

  private generateTestName(prompt: string): string {
    const words = prompt.toLowerCase().match(/\w+/g) || [];
    const keyWords = words.filter(w => w.length > 2 && !['test', 'testing', 'create', 'generate'].includes(w));
    return keyWords.slice(0, 4).map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ') + ' Tests';
  }

  private inferPlugin(prompt: string, testType: string): string {
    const promptLower = prompt.toLowerCase();
    
    if (promptLower.includes('supabase')) return 'supabase';
    if (promptLower.includes('database') || promptLower.includes('sql')) return 'sql';
    if (testType === 'database') return 'sql';
    return 'http';
  }

  private generateTestConfig(prompt: string, testType: string) {
    if (testType === 'database' || prompt.toLowerCase().includes('database')) {
      return {
        driver: 'postgres',
        dsn: '{{ .vars.db_dsn }}',
        commands: ['SELECT 1 as test;'],
      };
    }
    
    return {
      method: 'GET',
      url: '{{ .vars.base_url }}/endpoint',
      headers: {
        Authorization: 'Bearer {{ .vars.auth_token }}',
      },
    };
  }

  private async executeRocketshipCommand(args: string[]): Promise<{ success: boolean; output: string }> {
    return new Promise((resolve) => {
      const child = spawn('rocketship', args, { stdio: 'pipe' });
      let output = '';

      child.stdout?.on('data', (data) => {
        output += data.toString();
      });

      child.stderr?.on('data', (data) => {
        output += data.toString();
      });

      child.on('close', (code) => {
        resolve({
          success: code === 0,
          output,
        });
      });

      child.on('error', (error) => {
        resolve({
          success: false,
          output: `Command failed: ${error.message}`,
        });
      });
    });
  }

  private analyzeTestFailures(output: string): string {
    const patterns = [
      { pattern: /connection refused/i, message: 'Service appears to be down or unreachable' },
      { pattern: /timeout/i, message: 'Request timed out - service may be slow' },
      { pattern: /401/i, message: 'Authentication failed - check API keys' },
      { pattern: /404/i, message: 'Resource not found - verify URLs' },
      { pattern: /500/i, message: 'Server error - check service logs' },
    ];

    for (const { pattern, message } of patterns) {
      if (pattern.test(output)) {
        return message;
      }
    }

    return 'Test failed for unknown reasons';
  }

  private parseGitDiff(diffOutput: string) {
    const lines = diffOutput.trim().split('\n').filter(line => line);
    return lines.map(line => {
      const [status, filename] = line.split('\t');
      return { status, filename };
    });
  }

  private generateTestSuggestions(changes: Array<{ status: string; filename: string }>): string[] {
    const suggestions: string[] = [];
    
    const apiChanges = changes.filter(c => 
      c.filename.includes('route') || 
      c.filename.includes('controller') || 
      c.filename.includes('handler')
    );
    
    const dbChanges = changes.filter(c => 
      c.filename.includes('migration') || 
      c.filename.includes('model') || 
      c.filename.includes('schema')
    );

    if (apiChanges.length > 0) {
      suggestions.push(`• Add/update API tests for ${apiChanges.length} endpoint changes`);
    }

    if (dbChanges.length > 0) {
      suggestions.push(`• Update database tests for ${dbChanges.length} schema changes`);
    }

    if (suggestions.length === 0) {
      suggestions.push('• No immediate test changes required');
    }

    return suggestions;
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