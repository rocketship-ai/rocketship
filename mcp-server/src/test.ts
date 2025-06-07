#!/usr/bin/env node

/**
 * Simple test script to verify the MCP server can be imported and initialized
 */

import { RocketshipMCPServer } from './index.js';

async function runTests() {
  console.log('🚀 Testing Rocketship MCP Server...\n');
  
  try {
    console.log('📦 Testing server initialization...');
    const server = new RocketshipMCPServer();
    console.log('✅ Server initialized successfully');
    
    console.log('\n⚙️  Testing test generation...');
    // Test the private method through a simple call
    const testYaml = (server as any).generateTestFromPrompt('Test user API', 'api');
    if (testYaml.includes('name:') && testYaml.includes('tests:')) {
      console.log('✅ Test generation works');
    } else {
      throw new Error('Generated test YAML is invalid');
    }
    
    console.log('\n✅ All tests passed! MCP server is ready to use.');
    process.exit(0);
    
  } catch (error) {
    console.error(`❌ Test failed: ${error instanceof Error ? error.message : String(error)}`);
    process.exit(1);
  }
}

if (require.main === module) {
  runTests();
}