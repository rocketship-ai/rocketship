#!/usr/bin/env node

/**
 * Test script to verify the MCP server provides correct CLI guidance
 * addressing the specific issues mentioned in the requirements
 */

const { RocketshipMCPServer } = require('./dist/index.js');

async function testCLIGuidance() {
  console.log('🧪 Testing CLI guidance for specific issues...\n');
  
  // Create server instance
  const server = new RocketshipMCPServer();
  
  // Test the CLI guidance method directly
  console.log('📋 Testing installation guidance...');
  const installationResult = await server.handleGetCLIGuidance({ command: 'run' });
  const installationText = installationResult.content[0].text;
  
  // Check for specific fixes
  console.log('\n🔍 Checking for specific issue fixes:\n');
  
  // Issue 1: Wrong installation method (Homebrew)
  const hasBrewWarning = installationText.includes('NOT Available') || 
                        installationText.includes('not currently supported') ||
                        installationText.includes('brew install') === false;
  console.log(`✅ Issue 1 - No incorrect Homebrew installation: ${hasBrewWarning ? 'FIXED' : 'NEEDS ATTENTION'}`);
  
  // Issue 2: Correct --var-file flag (not --vars)
  const hasCorrectVarFile = installationText.includes('--var-file') && 
                           !installationText.includes('--vars ');
  console.log(`✅ Issue 2 - Correct --var-file flag: ${hasCorrectVarFile ? 'FIXED' : 'NEEDS ATTENTION'}`);
  
  // Issue 3: Current CLI version information
  const hasVersionInfo = installationText.includes('v0.5.6') || 
                         installationText.includes('CLI Version');
  console.log(`✅ Issue 3 - Current CLI version info: ${hasVersionInfo ? 'FIXED' : 'NEEDS ATTENTION'}`);
  
  // Issue 4: Accurate flag documentation
  const hasCorrectFlags = installationText.includes('--auto') && 
                         installationText.includes('--file') &&
                         installationText.includes('--dir');
  console.log(`✅ Issue 4 - Accurate CLI flags: ${hasCorrectFlags ? 'FIXED' : 'NEEDS ATTENTION'}`);
  
  console.log('\n📄 Sample CLI guidance output:');
  console.log('─'.repeat(50));
  console.log(installationText.substring(0, 1000) + '...');
  console.log('─'.repeat(50));
  
  console.log('\n🎉 CLI guidance testing complete!');
}

// Run the test
testCLIGuidance().catch(console.error);