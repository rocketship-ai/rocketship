#!/usr/bin/env node

/**
 * CLI Introspection script for Rocketship MCP server
 * Automatically extracts current CLI capabilities, help text, and documentation
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

// Detect project root by looking for go.mod
function findProjectRoot() {
  let dir = process.cwd();
  while (dir !== '/') {
    if (fs.existsSync(path.join(dir, 'go.mod'))) {
      return dir;
    }
    dir = path.dirname(dir);
  }
  throw new Error('Could not find project root (go.mod not found)');
}

const PROJECT_ROOT = findProjectRoot();

// Build the CLI binary if it doesn't exist or is outdated
function buildCLI() {
  const binaryPath = path.join(PROJECT_ROOT, 'tmp', 'rocketship-cli');
  
  // Create tmp directory if it doesn't exist
  const tmpDir = path.dirname(binaryPath);
  if (!fs.existsSync(tmpDir)) {
    fs.mkdirSync(tmpDir, { recursive: true });
  }

  console.log('🔨 Building Rocketship CLI for introspection...');
  try {
    execSync(`cd ${PROJECT_ROOT} && go build -o ${binaryPath} ./cmd/rocketship`, {
      stdio: 'pipe'
    });
    console.log('✓ CLI binary built successfully');
    return binaryPath;
  } catch (error) {
    throw new Error(`Failed to build CLI: ${error.message}`);
  }
}

// Execute CLI command and capture output
function execCLI(binaryPath, args) {
  try {
    const output = execSync(`${binaryPath} ${args}`, {
      encoding: 'utf8',
      stdio: 'pipe'
    });
    return output.trim();
  } catch (error) {
    // Some commands may exit with non-zero code but still provide useful output
    return error.stdout ? error.stdout.trim() : '';
  }
}

// Extract help text for all commands
function extractCLIHelp(binaryPath) {
  const helpData = {};
  
  // Main help
  helpData.main = execCLI(binaryPath, '--help');
  
  // Extract available commands from main help
  const mainHelpLines = helpData.main.split('\n');
  const commandsSection = mainHelpLines.findIndex(line => line.includes('Available Commands:'));
  
  if (commandsSection !== -1) {
    const commands = [];
    for (let i = commandsSection + 1; i < mainHelpLines.length; i++) {
      const line = mainHelpLines[i].trim();
      if (line === '' || line.startsWith('Flags:')) break;
      
      const match = line.match(/^\s*(\w+)\s+(.+)$/);
      if (match) {
        commands.push({
          name: match[1],
          description: match[2]
        });
      }
    }
    
    // Get help for each command
    for (const cmd of commands) {
      if (cmd.name === 'help' || cmd.name === 'completion') continue;
      
      helpData[cmd.name] = {
        description: cmd.description,
        help: execCLI(binaryPath, `${cmd.name} --help`)
      };
      
      // Check for subcommands
      if (cmd.name === 'start' || cmd.name === 'stop') {
        const subcommandHelp = helpData[cmd.name].help;
        const subcommandsSection = subcommandHelp.split('\n').findIndex(line => line.includes('Available Commands:'));
        
        if (subcommandsSection !== -1) {
          const subcommands = [];
          const lines = subcommandHelp.split('\n');
          for (let i = subcommandsSection + 1; i < lines.length; i++) {
            const line = lines[i].trim();
            if (line === '' || line.startsWith('Flags:')) break;
            
            const match = line.match(/^\s*(\w+)\s+(.+)$/);
            if (match) {
              subcommands.push(match[1]);
            }
          }
          
          helpData[cmd.name].subcommands = {};
          for (const subcmd of subcommands) {
            helpData[cmd.name].subcommands[subcmd] = execCLI(binaryPath, `${cmd.name} ${subcmd} --help`);
          }
        }
      }
    }
  }
  
  return helpData;
}

// Extract version and build information
function extractVersionInfo(binaryPath) {
  const version = execCLI(binaryPath, 'version');
  
  return {
    version: version,
    extractedAt: new Date().toISOString(),
    gitCommit: getGitCommit(),
    gitBranch: getGitBranch()
  };
}

function getGitCommit() {
  try {
    return execSync('git rev-parse HEAD', { encoding: 'utf8', stdio: 'pipe' }).trim();
  } catch {
    return 'unknown';
  }
}

function getGitBranch() {
  try {
    return execSync('git rev-parse --abbrev-ref HEAD', { encoding: 'utf8', stdio: 'pipe' }).trim();
  } catch {
    return 'unknown';
  }
}

// Extract installation info dynamically from documentation
function extractInstallationInfo() {
  const installationMethods = {
    methods: [],
    notAvailable: [],
    recommended: null
  };

  // Read from README
  const readmePath = path.join(PROJECT_ROOT, 'README.md');
  if (fs.existsSync(readmePath)) {
    const readme = fs.readFileSync(readmePath, 'utf8');
    
    // Extract installation section - look for #### Install pattern specifically
    const installSection = readme.match(/#{3,4}\s*Install\b[\s\S]*?(?=#{2,4}\s*[A-Z]|$)/i);
    if (installSection) {
      const installText = installSection[0];
      installationMethods.fromReadme = installText;
      
      // Extract installation methods from README
      const codeBlocks = installText.match(/```[\s\S]*?```/g) || [];
      codeBlocks.forEach((block, index) => {
        const command = block.replace(/```\w*\n?|```/g, '').trim();
        if (command) {
          // Only include commands that actually install Rocketship itself
          // Skip prerequisites like 'brew install temporal'
          if (command.includes('rocketship') && !command.includes('brew install temporal')) {
            // Determine installation method type
            let methodName = `Method ${index + 1}`;
            if (command.includes('curl') && command.includes('rocketship')) {
              if (command.includes('darwin-arm64')) {
                methodName = 'Direct Download (macOS ARM64)';
              } else if (command.includes('darwin-amd64')) {
                methodName = 'Direct Download (macOS Intel)';
              } else if (command.includes('linux-amd64')) {
                methodName = 'Direct Download (Linux AMD64)';
              } else if (command.includes('linux-arm64')) {
                methodName = 'Direct Download (Linux ARM64)';
              } else if (command.includes('windows')) {
                methodName = 'Direct Download (Windows)';
              } else {
                methodName = 'Direct Download';
              }
            }
            
            installationMethods.methods.push({
              name: methodName,
              description: 'Installation method from documentation',
              command: command
            });
          }
        }
      });
      
      // Look for "not available" mentions and add known limitations
      if (installText.toLowerCase().includes('homebrew') && installText.toLowerCase().includes('not')) {
        installationMethods.notAvailable.push('Homebrew (brew install) - not currently supported');
      } else {
        // Since Rocketship is not available via Homebrew, explicitly note this
        installationMethods.notAvailable.push('Homebrew (brew install rocketship) - NOT Available');
      }
      
      if (installText.toLowerCase().includes('apt') && installText.toLowerCase().includes('not')) {
        installationMethods.notAvailable.push('Package managers (apt, yum, etc.) - not currently supported');
      } else {
        installationMethods.notAvailable.push('Package managers (apt install, yum install) - NOT Available');
      }
      
      // Extract recommended method
      const recommendedMatch = installText.match(/recommended[\s\S]*?(?:\n|$)/i);
      if (recommendedMatch) {
        installationMethods.recommended = recommendedMatch[0].trim();
      }
    }
  }
  
  // Fallback: extract from docs directory
  const docsPath = path.join(PROJECT_ROOT, 'docs');
  if (fs.existsSync(docsPath)) {
    const docFiles = fs.readdirSync(docsPath).filter(f => f.endsWith('.md'));
    for (const docFile of docFiles) {
      if (docFile.toLowerCase().includes('install') || docFile.toLowerCase().includes('setup')) {
        const docContent = fs.readFileSync(path.join(docsPath, docFile), 'utf8');
        installationMethods.fromDocs = installationMethods.fromDocs || {};
        installationMethods.fromDocs[docFile] = docContent;
      }
    }
  }

  return installationMethods;
}

// Extract usage patterns dynamically from CLI help and documentation
function extractUsagePatterns(binaryPath, helpData) {
  const patterns = {
    common_patterns: [],
    file_structure: {},
    examples_from_help: {},
    syntax_patterns: {}
  };
  
  // Extract examples from CLI help text
  Object.entries(helpData).forEach(([command, data]) => {
    if (typeof data === 'object' && data.help) {
      const help = data.help;
      
      // Look for "Examples:" section
      const examplesMatch = help.match(/Examples?:[\s\S]*?(?=\n\n|\nFlags:|$)/i);
      if (examplesMatch) {
        patterns.examples_from_help[command] = examplesMatch[0];
        
        // Extract individual command examples
        const commandLines = examplesMatch[0].split('\n')
          .filter(line => line.trim().startsWith('rocketship'))
          .map(line => line.trim());
          
        commandLines.forEach((cmdLine, index) => {
          patterns.common_patterns.push({
            name: `${command} example ${index + 1}`,
            command: cmdLine,
            description: `Example from ${command} help`,
            source: 'cli_help'
          });
        });
      }
      
      // Look for "Usage:" patterns
      const usageMatch = help.match(/Usage:[\s\S]*?(?=\n\n|\nFlags:|$)/i);
      if (usageMatch) {
        patterns.examples_from_help[`${command}_usage`] = usageMatch[0];
      }
    }
  });
  
  // Extract file structure patterns from documentation
  patterns.file_structure = extractFileStructureFromDocs();
  
  // Extract syntax patterns from documentation
  patterns.syntax_patterns = extractSyntaxPatternsFromDocs();
  
  return patterns;
}

// Extract file structure patterns from documentation
function extractFileStructureFromDocs() {
  const structure = {
    recommended: null,
    pattern: null,
    examples: {},
    from_docs: {}
  };
  
  // Check CLAUDE.md for structure guidance
  const claudePath = path.join(PROJECT_ROOT, 'CLAUDE.md');
  if (fs.existsSync(claudePath)) {
    const claudeContent = fs.readFileSync(claudePath, 'utf8');
    
    // Extract file structure examples
    const structureMatches = claudeContent.match(/```[\s\S]*?\.rocketship[\s\S]*?```/g) || [];
    structureMatches.forEach((match, index) => {
      structure.examples[`example_${index + 1}`] = match.replace(/```\w*\n?|```/g, '').trim();
    });
    
    // Look for recommended patterns
    const patternMatch = claudeContent.match(/\.rocketship\/[\w-]+\/rocketship\.yaml/g);
    if (patternMatch) {
      structure.pattern = patternMatch[0];
      structure.recommended = '.rocketship/';
    }
    
    structure.from_docs.claude = claudeContent;
  }
  
  // Check .rocketship directory for test suite YAML files
  const rocketshipPath = path.join(PROJECT_ROOT, '.rocketship');
  if (fs.existsSync(rocketshipPath)) {
    const yamlFiles = fs.readdirSync(rocketshipPath, { withFileTypes: true })
      .filter(dirent => dirent.isFile() && dirent.name.endsWith('.yaml'))
      .map(dirent => dirent.name);

    structure.real_examples = {};
    yamlFiles.forEach(file => {
      const name = file.replace('.yaml', '');
      structure.real_examples[name] = {
        path: `.rocketship/${file}`,
        exists: true
      };
    });
  }
  
  return structure;
}

// Extract syntax patterns from documentation
function extractSyntaxPatternsFromDocs() {
  const syntax = {
    variables: {},
    assertions: {},
    plugins: {},
    save_operations: {},
    from_docs: {}
  };
  
  // Extract from CLAUDE.md
  const claudePath = path.join(PROJECT_ROOT, 'CLAUDE.md');
  if (fs.existsSync(claudePath)) {
    const claudeContent = fs.readFileSync(claudePath, 'utf8');
    
    // Extract variable syntax
    const variableMatches = claudeContent.match(/{{[^}]+}}/g) || [];
    variableMatches.forEach(match => {
      if (match.includes('.vars.')) {
        syntax.variables.config = syntax.variables.config || [];
        syntax.variables.config.push(match);
      } else if (match.includes('.env.')) {
        syntax.variables.environment = syntax.variables.environment || [];
        syntax.variables.environment.push(match);
      } else {
        syntax.variables.runtime = syntax.variables.runtime || [];
        syntax.variables.runtime.push(match);
      }
    });
    
    // Extract save operation patterns
    const saveMatches = claudeContent.match(/save:[\s\S]*?(?=\n\w|\n#|$)/gm) || [];
    saveMatches.forEach((match, index) => {
      syntax.save_operations[`example_${index + 1}`] = match.trim();
    });
    
    syntax.from_docs.claude = claudeContent;
  }
  
  // Extract from documentation files
  const docsPath = path.join(PROJECT_ROOT, 'docs');
  if (fs.existsSync(docsPath)) {
    const findDocs = (dir) => {
      const files = [];
      if (fs.existsSync(dir)) {
        fs.readdirSync(dir, { withFileTypes: true }).forEach(dirent => {
          const fullPath = path.join(dir, dirent.name);
          if (dirent.isDirectory()) {
            files.push(...findDocs(fullPath));
          } else if (dirent.name.endsWith('.md')) {
            files.push(fullPath);
          }
        });
      }
      return files;
    };
    
    const docFiles = findDocs(docsPath);
    docFiles.forEach(filePath => {
      const content = fs.readFileSync(filePath, 'utf8');
      const relativePath = path.relative(PROJECT_ROOT, filePath);
      syntax.from_docs[relativePath] = content;
      
      // Extract plugin-specific syntax
      const pluginMatches = content.match(/plugin:\s*(\w+)/g) || [];
      pluginMatches.forEach(match => {
        const plugin = match.split(':')[1].trim();
        syntax.plugins[plugin] = syntax.plugins[plugin] || [];
        
        // Get the surrounding context
        const lines = content.split('\n');
        const lineIndex = lines.findIndex(line => line.includes(match));
        if (lineIndex !== -1) {
          const context = lines.slice(Math.max(0, lineIndex - 5), lineIndex + 10).join('\n');
          syntax.plugins[plugin].push(context);
        }
      });
    });
  }
  
  return syntax;
}

// Extract all documentation content
function extractDocumentationContent() {
  const docs = {};
  
  // Get all markdown files
  const findMarkdownFiles = (dir, prefix = '') => {
    if (!fs.existsSync(dir)) return;
    
    fs.readdirSync(dir, { withFileTypes: true }).forEach(dirent => {
      const fullPath = path.join(dir, dirent.name);
      const relativePath = prefix ? `${prefix}/${dirent.name}` : dirent.name;
      
      if (dirent.isDirectory()) {
        findMarkdownFiles(fullPath, relativePath);
      } else if (dirent.name.endsWith('.md')) {
        docs[relativePath] = fs.readFileSync(fullPath, 'utf8');
      }
    });
  };
  
  // Scan key directories
  findMarkdownFiles(path.join(PROJECT_ROOT, 'docs'));
  findMarkdownFiles(path.join(PROJECT_ROOT, 'examples'));
  
  // Include key files
  const keyFiles = ['README.md', 'CLAUDE.md'];
  keyFiles.forEach(file => {
    const filePath = path.join(PROJECT_ROOT, file);
    if (fs.existsSync(filePath)) {
      docs[file] = fs.readFileSync(filePath, 'utf8');
    }
  });
  
  return docs;
}

// Extract schema information from actual schema files
function extractSchemaInformation() {
  const schemaInfo = {};
  
  // Look for schema files
  const findSchemaFiles = (dir, prefix = '') => {
    if (!fs.existsSync(dir)) return;
    
    fs.readdirSync(dir, { withFileTypes: true }).forEach(dirent => {
      const fullPath = path.join(dir, dirent.name);
      const relativePath = prefix ? `${prefix}/${dirent.name}` : dirent.name;
      
      if (dirent.isDirectory()) {
        findSchemaFiles(fullPath, relativePath);
      } else if (dirent.name.includes('schema') || dirent.name.endsWith('.json')) {
        try {
          const content = fs.readFileSync(fullPath, 'utf8');
          if (content.trim().startsWith('{')) {
            schemaInfo[relativePath] = JSON.parse(content);
          } else {
            schemaInfo[relativePath] = content;
          }
        } catch (error) {
          schemaInfo[relativePath] = { error: `Failed to parse: ${error.message}` };
        }
      }
    });
  };
  
  // Scan for schema files
  findSchemaFiles(PROJECT_ROOT);
  
  return schemaInfo;
}

// Main introspection function
function performCLIIntrospection() {
  console.log('🔍 Starting CLI introspection...');
  
  const binaryPath = buildCLI();
  const helpData = extractCLIHelp(binaryPath);
  
  const introspectionData = {
    timestamp: new Date().toISOString(),
    version: extractVersionInfo(binaryPath),
    help: helpData,
    installation: extractInstallationInfo(),
    usage: extractUsagePatterns(binaryPath, helpData),
    flags: extractFlagDocumentation(binaryPath),
    documentation: extractDocumentationContent(),
    schema_info: extractSchemaInformation()
  };
  
  console.log('✓ CLI introspection completed');
  return introspectionData;
}

// Extract comprehensive flag documentation
function extractFlagDocumentation(binaryPath) {
  const flagDocs = {};
  
  // Extract flags from help text
  const runHelp = execCLI(binaryPath, 'run --help');
  const flagsSection = runHelp.split('\n').findIndex(line => line.includes('Flags:'));
  
  if (flagsSection !== -1) {
    const lines = runHelp.split('\n');
    for (let i = flagsSection + 1; i < lines.length; i++) {
      const line = lines[i];
      if (line.startsWith('Use ') || line.trim() === '') continue;
      
      // Parse flag lines like: -a, --auto    description
      const flagMatch = line.match(/^\s*(-\w,?\s*)?--(\w+(?:-\w+)*)\s+(.+)$/);
      if (flagMatch) {
        const shortFlag = flagMatch[1] ? flagMatch[1].replace(/,\s*/, '').trim() : null;
        const longFlag = flagMatch[2];
        const description = flagMatch[3];
        
        flagDocs[longFlag] = {
          long: `--${longFlag}`,
          short: shortFlag,
          description: description
        };
      }
    }
  }
  
  return flagDocs;
}

export { performCLIIntrospection };

// If run directly
if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    const data = performCLIIntrospection();
    console.log('\n📋 Introspection Summary:');
    console.log(`- Version: ${data.version.version}`);
    console.log(`- Commands: ${Object.keys(data.help).length}`);
    console.log(`- Flags documented: ${Object.keys(data.flags).length}`);
    console.log(`- Installation methods: ${data.installation.methods.length}`);
    
    // Write to file for inspection
    const outputPath = path.join(process.cwd(), 'cli-introspection.json');
    fs.writeFileSync(outputPath, JSON.stringify(data, null, 2));
    console.log(`\n📄 Full data written to: ${outputPath}`);
  } catch (error) {
    console.error('❌ CLI introspection failed:', error.message);
    process.exit(1);
  }
}