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
    
    console.log('\n⚙️  Testing dynamic knowledge loader...');
    // Test that the dynamic loader works
    const exampleResponse = await (server as any).handleGetExamples({ 
      feature_type: 'http',
      use_case: 'API testing' 
    });
    if (exampleResponse.content && exampleResponse.content[0].text.includes('Real Rocketship Examples')) {
      console.log('✅ Dynamic knowledge loader works');
    } else {
      throw new Error('Dynamic knowledge loader failed');
    }
    
    console.log('\n⚙️  Testing test structure suggestions...');
    const structureResponse = await (server as any).handleSuggestStructure({ 
      project_type: 'frontend',
      user_flows: ['login', 'dashboard']
    });
    if (structureResponse.content && structureResponse.content[0].text.includes('.rocketship/')) {
      console.log('✅ Test structure suggestions work');
    } else {
      throw new Error('Test structure suggestions failed');
    }

    console.log('\n⚙️  Testing schema info...');
    const schemaResponse = await (server as any).handleGetSchemaInfo({ 
      section: 'plugins'
    });
    if (schemaResponse.content && schemaResponse.content[0].text.includes('Available Plugins')) {
      console.log('✅ Schema info retrieval works');
    } else {
      throw new Error('Schema info retrieval failed');
    }
    
    console.log('\n✅ All tests passed! MCP server is ready to use.');
    process.exit(0);
    
  } catch (error) {
    console.error(`❌ Test failed: ${error instanceof Error ? error.message : String(error)}`);
    if (error instanceof Error) {
      console.error('Stack trace:', error.stack);
    }
    process.exit(1);
  }
}

if (require.main === module) {
  runTests();
}