#!/usr/bin/env node

/**
 * Comprehensive test for CLI knowledge embedding
 * Tests that our CLI introspection captures the expected data accurately
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

// Colors for output
const GREEN = '\x1b[32m';
const RED = '\x1b[31m';
const YELLOW = '\x1b[33m';
const BLUE = '\x1b[36m';
const RESET = '\x1b[0m';

function log(color, message) {
  console.log(`${color}${message}${RESET}`);
}

async function runKnowledgeEmbeddingTest() {
  log(BLUE, '🧪 Testing CLI Knowledge Embedding System\n');

  try {
    // Run the embed-knowledge script
    log(YELLOW, '📦 Running embed-knowledge script...');
    execSync('npm run embed-knowledge', { stdio: 'pipe' });
    log(GREEN, '✅ Knowledge embedding completed successfully\n');

    // Load the embedded knowledge
    const embeddedKnowledge = await import('./src/embedded-knowledge.js');
    const { EMBEDDED_CLI_DATA, EMBEDDED_SCHEMA, EMBEDDED_EXAMPLES, EMBEDDED_DOCS } = embeddedKnowledge;

    // Test 1: CLI Version Detection
    log(BLUE, '1️⃣ Testing CLI Version Detection');
    if (EMBEDDED_CLI_DATA?.version?.version) {
      log(GREEN, `   ✅ CLI Version detected: ${EMBEDDED_CLI_DATA.version.version}`);
      if (EMBEDDED_CLI_DATA.version.version.includes('v0.5.6')) {
        log(GREEN, '   ✅ Version matches expected format');
      } else {
        log(YELLOW, '   ⚠️  Version format may have changed');
      }
    } else {
      log(RED, '   ❌ CLI version not detected');
    }

    // Test 2: CLI Commands and Help Text
    log(BLUE, '\n2️⃣ Testing CLI Commands and Help Text');
    const expectedCommands = ['run', 'validate', 'start', 'stop', 'version'];
    let commandsFound = 0;
    
    if (EMBEDDED_CLI_DATA?.help) {
      for (const cmd of expectedCommands) {
        if (EMBEDDED_CLI_DATA.help[cmd]) {
          log(GREEN, `   ✅ Command '${cmd}' help captured`);
          commandsFound++;
          
          // Check for help content
          if (EMBEDDED_CLI_DATA.help[cmd].help && EMBEDDED_CLI_DATA.help[cmd].help.length > 50) {
            log(GREEN, `   ✅ Command '${cmd}' has detailed help (${EMBEDDED_CLI_DATA.help[cmd].help.length} chars)`);
          }
        } else {
          log(RED, `   ❌ Command '${cmd}' help missing`);
        }
      }
      log(GREEN, `   ✅ ${commandsFound}/${expectedCommands.length} expected commands found`);
    } else {
      log(RED, '   ❌ No CLI help data found');
    }

    // Test 3: Flag Documentation
    log(BLUE, '\n3️⃣ Testing Flag Documentation');
    if (EMBEDDED_CLI_DATA?.flags && Object.keys(EMBEDDED_CLI_DATA.flags).length > 0) {
      const flags = Object.keys(EMBEDDED_CLI_DATA.flags);
      log(GREEN, `   ✅ ${flags.length} flags documented: ${flags.join(', ')}`);
      
      // Check for specific important flags
      const importantFlags = ['auto', 'file', 'dir', 'var-file'];
      for (const flag of importantFlags) {
        if (EMBEDDED_CLI_DATA.flags[flag]) {
          log(GREEN, `   ✅ Important flag '${flag}' documented`);
        } else {
          log(YELLOW, `   ⚠️  Important flag '${flag}' not found`);
        }
      }
    } else {
      log(RED, '   ❌ No flag documentation found');
    }

    // Test 4: Installation Methods
    log(BLUE, '\n4️⃣ Testing Installation Methods');
    if (EMBEDDED_CLI_DATA?.installation) {
      const installation = EMBEDDED_CLI_DATA.installation;
      
      if (installation.methods && installation.methods.length > 0) {
        log(GREEN, `   ✅ ${installation.methods.length} installation methods found`);
        installation.methods.forEach(method => {
          log(GREEN, `   ✅ Method: ${method.name} - ${method.description}`);
        });
      }
      
      if (installation.notAvailable && installation.notAvailable.length > 0) {
        log(GREEN, `   ✅ ${installation.notAvailable.length} unavailable methods documented`);
        installation.notAvailable.forEach(na => {
          if (na.toLowerCase().includes('homebrew')) {
            log(GREEN, '   ✅ Homebrew correctly marked as unavailable');
          }
        });
      }
      
      if (installation.fromReadme) {
        log(GREEN, `   ✅ Installation info extracted from README (${installation.fromReadme.length} chars)`);
      }
    } else {
      log(RED, '   ❌ No installation data found');
    }

    // Test 5: Usage Patterns
    log(BLUE, '\n5️⃣ Testing Usage Patterns');
    if (EMBEDDED_CLI_DATA?.usage?.common_patterns) {
      const patterns = EMBEDDED_CLI_DATA.usage.common_patterns;
      log(GREEN, `   ✅ ${patterns.length} usage patterns captured`);
      
      // Check for important patterns
      const runPatterns = patterns.filter(p => p.command.includes('run'));
      const validatePatterns = patterns.filter(p => p.command.includes('validate'));
      
      log(GREEN, `   ✅ ${runPatterns.length} 'run' command patterns`);
      log(GREEN, `   ✅ ${validatePatterns.length} 'validate' command patterns`);
      
      // Check for correct flags
      const hasVarFile = patterns.some(p => p.command.includes('--var-file'));
      const hasIncorrectVars = patterns.some(p => p.command.includes('--vars'));
      
      if (hasVarFile) {
        log(GREEN, '   ✅ Correct --var-file flag found in patterns');
      } else {
        log(YELLOW, '   ⚠️  --var-file flag not found in patterns');
      }
      
      if (hasIncorrectVars) {
        log(RED, '   ❌ Incorrect --vars flag found in patterns (should be --var-file)');
      } else {
        log(GREEN, '   ✅ No incorrect --vars flag found');
      }
    } else {
      log(RED, '   ❌ No usage patterns found');
    }

    // Test 6: File Structure Extraction
    log(BLUE, '\n6️⃣ Testing File Structure Extraction');
    if (EMBEDDED_CLI_DATA?.usage?.file_structure) {
      const structure = EMBEDDED_CLI_DATA.usage.file_structure;
      
      if (structure.pattern) {
        log(GREEN, `   ✅ File pattern extracted: ${structure.pattern}`);
      }
      
      if (structure.examples && Object.keys(structure.examples).length > 0) {
        log(GREEN, `   ✅ ${Object.keys(structure.examples).length} structure examples found`);
      }
      
      if (structure.real_examples && Object.keys(structure.real_examples).length > 0) {
        log(GREEN, `   ✅ ${Object.keys(structure.real_examples).length} real examples found in codebase`);
        Object.entries(structure.real_examples).forEach(([name, info]) => {
          log(GREEN, `   ✅ Real example: ${info.path}`);
        });
      }
    } else {
      log(RED, '   ❌ No file structure data found');
    }

    // Test 7: Variable Syntax Extraction
    log(BLUE, '\n7️⃣ Testing Variable Syntax Extraction');
    if (EMBEDDED_CLI_DATA?.usage?.syntax_patterns?.variables) {
      const vars = EMBEDDED_CLI_DATA.usage.syntax_patterns.variables;
      
      if (vars.config && vars.config.length > 0) {
        log(GREEN, `   ✅ ${vars.config.length} config variable examples: ${vars.config.slice(0, 3).join(', ')}`);
      }
      
      if (vars.environment && vars.environment.length > 0) {
        log(GREEN, `   ✅ ${vars.environment.length} environment variable examples: ${vars.environment.slice(0, 3).join(', ')}`);
      }
      
      if (vars.runtime && vars.runtime.length > 0) {
        log(GREEN, `   ✅ ${vars.runtime.length} runtime variable examples: ${vars.runtime.slice(0, 3).join(', ')}`);
      }
    } else {
      log(RED, '   ❌ No variable syntax patterns found');
    }

    // Test 8: Schema Validation
    log(BLUE, '\n8️⃣ Testing Schema Validation');
    if (EMBEDDED_SCHEMA) {
      log(GREEN, '   ✅ Schema loaded successfully');
      
      if (EMBEDDED_SCHEMA.properties?.tests?.items?.properties?.steps?.items?.properties?.plugin?.enum) {
        const plugins = EMBEDDED_SCHEMA.properties.tests.items.properties.steps.items.properties.plugin.enum;
        log(GREEN, `   ✅ ${plugins.length} plugins in schema: ${plugins.join(', ')}`);
        
        // Check for important plugins
        const importantPlugins = ['http', 'browser', 'sql', 'delay'];
        importantPlugins.forEach(plugin => {
          if (plugins.includes(plugin)) {
            log(GREEN, `   ✅ Important plugin '${plugin}' available`);
          } else {
            log(YELLOW, `   ⚠️  Plugin '${plugin}' not found`);
          }
        });
      }
    } else {
      log(RED, '   ❌ Schema not loaded');
    }

    // Test 9: Examples Loading
    log(BLUE, '\n9️⃣ Testing Examples Loading');
    if (EMBEDDED_EXAMPLES && EMBEDDED_EXAMPLES.size > 0) {
      log(GREEN, `   ✅ ${EMBEDDED_EXAMPLES.size} examples loaded`);
      
      const exampleNames = Array.from(EMBEDDED_EXAMPLES.keys());
      log(GREEN, `   ✅ Example types: ${exampleNames.slice(0, 5).join(', ')}${exampleNames.length > 5 ? '...' : ''}`);
      
      // Check for important example types
      const importantTypes = ['http', 'browser', 'sql'];
      importantTypes.forEach(type => {
        const hasExample = exampleNames.some(name => name.includes(type));
        if (hasExample) {
          log(GREEN, `   ✅ ${type} examples available`);
        } else {
          log(YELLOW, `   ⚠️  No ${type} examples found`);
        }
      });
    } else {
      log(RED, '   ❌ No examples loaded');
    }

    // Test 10: Documentation Loading
    log(BLUE, '\n🔟 Testing Documentation Loading');
    if (EMBEDDED_DOCS && EMBEDDED_DOCS.size > 0) {
      log(GREEN, `   ✅ ${EMBEDDED_DOCS.size} documentation files loaded`);
      
      const docPaths = Array.from(EMBEDDED_DOCS.keys());
      
      // Check for important docs
      const importantDocs = ['README.md', 'CLAUDE.md'];
      importantDocs.forEach(doc => {
        const hasDoc = docPaths.some(path => path.includes(doc));
        if (hasDoc) {
          log(GREEN, `   ✅ ${doc} loaded`);
        } else {
          log(YELLOW, `   ⚠️  ${doc} not found`);
        }
      });
      
      log(GREEN, `   ✅ Doc types: ${docPaths.slice(0, 5).join(', ')}${docPaths.length > 5 ? '...' : ''}`);
    } else {
      log(RED, '   ❌ No documentation loaded');
    }

    // Test 11: Compare with Known Issues
    log(BLUE, '\n1️⃣1️⃣ Testing Known Issue Fixes');
    
    // Issue 1: Wrong installation method
    let installationCorrect = false;
    if (EMBEDDED_CLI_DATA?.installation?.notAvailable) {
      const brewNotAvailable = EMBEDDED_CLI_DATA.installation.notAvailable.some(na => 
        na.toLowerCase().includes('homebrew') || na.toLowerCase().includes('brew')
      );
      if (brewNotAvailable) {
        log(GREEN, '   ✅ Homebrew correctly marked as unavailable (fixes brew upgrade issue)');
        installationCorrect = true;
      }
    }
    if (!installationCorrect) {
      log(RED, '   ❌ Homebrew availability not properly documented');
    }
    
    // Issue 2: Flag documentation
    let flagsCorrect = false;
    if (EMBEDDED_CLI_DATA?.flags) {
      const hasVarFile = EMBEDDED_CLI_DATA.flags['var-file'] !== undefined;
      const hasIncorrectVars = EMBEDDED_CLI_DATA.flags['vars'] !== undefined;
      
      if (hasVarFile && !hasIncorrectVars) {
        log(GREEN, '   ✅ Correct flag documentation (--var-file, not --vars)');
        flagsCorrect = true;
      } else if (!hasVarFile && hasIncorrectVars) {
        log(RED, '   ❌ Still has incorrect --vars flag');
      } else if (!hasVarFile && !hasIncorrectVars) {
        log(YELLOW, '   ⚠️  Flag documentation might be incomplete');
      }
    }
    
    // Issue 3: Current capabilities
    let versionCorrect = false;
    if (EMBEDDED_CLI_DATA?.version?.version && EMBEDDED_CLI_DATA.version.version.includes('0.5.6')) {
      log(GREEN, '   ✅ Current CLI version (0.5.6) properly captured');
      versionCorrect = true;
    } else {
      log(YELLOW, '   ⚠️  CLI version might not be current');
    }

    // Final Summary
    log(BLUE, '\n📊 Knowledge Embedding Test Summary\n');
    
    const testResults = [
      { name: 'CLI Version Detection', passed: versionCorrect },
      { name: 'Command Help Extraction', passed: commandsFound >= 4 },
      { name: 'Flag Documentation', passed: flagsCorrect },
      { name: 'Installation Methods', passed: installationCorrect },
      { name: 'Schema Loading', passed: !!EMBEDDED_SCHEMA },
      { name: 'Examples Loading', passed: EMBEDDED_EXAMPLES && EMBEDDED_EXAMPLES.size > 0 },
      { name: 'Documentation Loading', passed: EMBEDDED_DOCS && EMBEDDED_DOCS.size > 0 }
    ];
    
    const passedTests = testResults.filter(t => t.passed).length;
    const totalTests = testResults.length;
    
    testResults.forEach(test => {
      const status = test.passed ? GREEN + '✅' : RED + '❌';
      log('', `${status} ${test.name}${RESET}`);
    });
    
    log('', '');
    if (passedTests === totalTests) {
      log(GREEN, `🎉 All ${totalTests} tests passed! Knowledge embedding is working perfectly.`);
    } else {
      log(YELLOW, `⚠️  ${passedTests}/${totalTests} tests passed. Some issues may need attention.`);
    }
    
    // Output introspection data size for reference
    const dataSize = JSON.stringify(EMBEDDED_CLI_DATA).length;
    log(BLUE, `\n📈 Introspection Data Size: ${dataSize.toLocaleString()} characters`);
    
  } catch (error) {
    log(RED, `❌ Test failed: ${error.message}`);
    process.exit(1);
  }
}

// Run the test
runKnowledgeEmbeddingTest().catch(console.error);