#!/usr/bin/env node

/**
 * Debug script to examine the CLI data structure and installation data
 */

const { RocketshipMCPServer } = require('./dist/index.js');

async function debugCLIData() {
  console.log('🔍 Debugging CLI data structure...\n');
  
  // Create server instance
  const server = new RocketshipMCPServer();
  
  // Access the private cliData property for debugging
  const cliData = server.cliData;
  
  console.log('📊 CLI Data Structure:');
  console.log('─'.repeat(50));
  console.log('Type:', typeof cliData);
  console.log('Keys:', Object.keys(cliData || {}));
  
  if (cliData) {
    console.log('\n📅 Timestamp:', cliData.timestamp);
    console.log('📦 Version:', cliData.version?.version);
    console.log('🔧 Git Commit:', cliData.version?.gitCommit);
    
    // Check for installation data
    console.log('\n🔍 Installation Data Analysis:');
    console.log('─'.repeat(30));
    console.log('Has installation property:', 'installation' in cliData);
    console.log('Installation data type:', typeof cliData.installation);
    console.log('Installation data value:', cliData.installation);
    
    // Check top-level properties
    console.log('\n📋 Top-level properties:');
    for (const key of Object.keys(cliData)) {
      console.log(`  - ${key}: ${typeof cliData[key]}`);
    }
    
    // Look for any properties that might contain installation info
    console.log('\n🔎 Searching for installation-related data:');
    for (const [key, value] of Object.entries(cliData)) {
      if (typeof value === 'object' && value !== null) {
        const subKeys = Object.keys(value);
        const hasInstallationKeys = subKeys.some(k => 
          k.toLowerCase().includes('install') || 
          k.toLowerCase().includes('download') ||
          k.toLowerCase().includes('setup')
        );
        if (hasInstallationKeys) {
          console.log(`  Found installation-related keys in ${key}:`, subKeys.filter(k => 
            k.toLowerCase().includes('install') || 
            k.toLowerCase().includes('download') ||
            k.toLowerCase().includes('setup')
          ));
        }
      }
    }
  }
  
  // Test the actual CLI guidance method and see what's returned
  console.log('\n🧪 Testing CLI guidance method:');
  console.log('─'.repeat(40));
  
  try {
    // This should trigger the debug logging in the source code
    console.log('Calling handleGetCLIGuidance with command "run"...');
    const guidance = await server.handleGetCLIGuidance({ command: 'run' });
    const text = guidance.content[0].text;
    
    console.log('✅ CLI guidance method executed successfully');
    console.log('📝 Response length:', text.length);
    console.log('🔍 Contains "installation":', text.toLowerCase().includes('installation'));
    console.log('🔍 Contains "download":', text.toLowerCase().includes('download'));
    console.log('🔍 Contains "install":', text.toLowerCase().includes('install'));
    
    // Check for specific installation guidance patterns
    const patterns = [
      'wget',
      'curl',
      'github.com/releases',
      'binary',
      'executable',
      'chmod',
      'PATH'
    ];
    
    console.log('\n🔍 Installation-related content analysis:');
    for (const pattern of patterns) {
      const found = text.toLowerCase().includes(pattern.toLowerCase());
      console.log(`  ${pattern}: ${found ? '✅' : '❌'}`);
    }
    
  } catch (error) {
    console.error('❌ Error testing CLI guidance:', error.message);
  }
  
  console.log('\n🎉 Debug analysis complete!');
}

// Run the debug
debugCLIData().catch(console.error);