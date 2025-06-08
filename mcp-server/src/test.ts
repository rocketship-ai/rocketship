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
    
    console.log('\n⚙️  Testing knowledge base access...');
    // Test that the knowledge base is accessible
    const exampleResponse = await (server as any).handleGetExamples({ feature: 'api_testing' });
    if (exampleResponse.content && exampleResponse.content[0].text.includes('API testing patterns')) {
      console.log('✅ Knowledge base access works');
    } else {
      throw new Error('Knowledge base access failed');
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