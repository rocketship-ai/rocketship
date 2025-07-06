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

// Extract installation and usage patterns
function extractInstallationInfo() {
  // Read from README or installation docs
  const readmePath = path.join(PROJECT_ROOT, 'README.md');
  let installationMethods = {
    recommended: 'Direct download from GitHub releases',
    methods: [
      {
        name: 'GitHub Releases',
        description: 'Download pre-built binaries from GitHub releases',
        command: 'curl -L https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-amd64 -o rocketship && chmod +x rocketship'
      },
      {
        name: 'Go Install',
        description: 'Install from source using Go',
        command: 'go install github.com/rocketship-ai/rocketship/cmd/rocketship@latest'
      },
      {
        name: 'Docker',
        description: 'Run using Docker container',
        command: 'docker run --rm rocketshipai/rocketship:latest'
      }
    ],
    notAvailable: [
      'Homebrew (brew install) - not currently supported',
      'Package managers (apt, yum, etc.) - not currently supported'
    ]
  };

  if (fs.existsSync(readmePath)) {
    const readme = fs.readFileSync(readmePath, 'utf8');
    // Extract installation info from README if available
    const installSection = readme.match(/## Installation[\s\S]*?(?=##|$)/i);
    if (installSection) {
      installationMethods.fromReadme = installSection[0];
    }
  }

  return installationMethods;
}

// Extract common usage patterns and examples
function extractUsagePatterns() {
  return {
    common_patterns: [
      {
        name: 'Run single test with auto-start',
        command: 'rocketship run -af test.yaml',
        description: 'Automatically starts engine, runs test, stops engine'
      },
      {
        name: 'Run directory of tests',
        command: 'rocketship run -ad .rocketship/',
        description: 'Run all rocketship.yaml files in directory'
      },
      {
        name: 'Run with variables',
        command: 'rocketship run -af test.yaml --var URL=http://localhost:3000',
        description: 'Set runtime variables for tests'
      },
      {
        name: 'Run with variable file',
        command: 'rocketship run -af test.yaml --var-file vars.yaml',
        description: 'Load variables from YAML file'
      },
      {
        name: 'Validate tests',
        command: 'rocketship validate test.yaml',
        description: 'Check test syntax without running'
      },
      {
        name: 'Manual server mode',
        command: 'rocketship start server -b && rocketship run test.yaml && rocketship stop server',
        description: 'Manual control of server lifecycle'
      }
    ],
    file_structure: {
      recommended: '.rocketship/',
      pattern: '.rocketship/test-name/rocketship.yaml',
      example: {
        '.rocketship/': {
          'login-test/': {
            'rocketship.yaml': 'Test file for login functionality'
          },
          'api-tests/': {
            'rocketship.yaml': 'Test file for API endpoints'
          }
        }
      }
    }
  };
}

// Main introspection function
function performCLIIntrospection() {
  console.log('🔍 Starting CLI introspection...');
  
  const binaryPath = buildCLI();
  
  const introspectionData = {
    timestamp: new Date().toISOString(),
    version: extractVersionInfo(binaryPath),
    help: extractCLIHelp(binaryPath),
    installation: extractInstallationInfo(),
    usage: extractUsagePatterns(),
    flags: extractFlagDocumentation(binaryPath)
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