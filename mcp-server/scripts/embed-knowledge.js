#!/usr/bin/env node

/**
 * Build script to embed Rocketship knowledge into the MCP server
 * This replaces the static embedded content with real examples, docs, and current CLI capabilities
 */

import * as fs from 'fs';
import * as path from 'path';
import { performCLIIntrospection } from './cli-introspection.js';

const PROJECT_ROOT = path.resolve(process.cwd(), '..');
const MCP_SRC = path.resolve(process.cwd(), 'src');

function loadRealSchema() {
  const schemaPath = path.join(PROJECT_ROOT, 'internal', 'dsl', 'schema.json');
  if (!fs.existsSync(schemaPath)) {
    throw new Error(`Schema not found at ${schemaPath}`);
  }
  return JSON.parse(fs.readFileSync(schemaPath, 'utf-8'));
}

function loadRealExamples() {
  const examplesDir = path.join(PROJECT_ROOT, 'examples');
  const examples = new Map();
  
  if (!fs.existsSync(examplesDir)) {
    throw new Error(`Examples directory not found at ${examplesDir}`);
  }

  const subdirs = fs.readdirSync(examplesDir, { withFileTypes: true })
    .filter(dirent => dirent.isDirectory())
    .map(dirent => dirent.name);

  for (const subdir of subdirs) {
    const yamlPath = path.join(examplesDir, subdir, 'rocketship.yaml');
    if (fs.existsSync(yamlPath)) {
      const content = fs.readFileSync(yamlPath, 'utf-8');
      examples.set(subdir, content);
    }
  }

  return examples;
}

function loadRealDocs() {
  const docs = new Map();
  
  // Load reference docs
  const refDir = path.join(PROJECT_ROOT, 'docs', 'src', 'reference');
  if (fs.existsSync(refDir)) {
    const refFiles = fs.readdirSync(refDir).filter(f => f.endsWith('.md'));
    for (const file of refFiles) {
      const content = fs.readFileSync(path.join(refDir, file), 'utf-8');
      docs.set(`reference/${file}`, content);
    }
  }

  // Load example docs  
  const exampleDocsDir = path.join(PROJECT_ROOT, 'docs', 'src', 'examples');
  if (fs.existsSync(exampleDocsDir)) {
    const exampleFiles = fs.readdirSync(exampleDocsDir).filter(f => f.endsWith('.md'));
    for (const file of exampleFiles) {
      const content = fs.readFileSync(path.join(exampleDocsDir, file), 'utf-8');
      docs.set(`examples/${file}`, content);
    }
  }

  return docs;
}

function generateKnowledgeModule() {
  console.log('🔄 Loading real Rocketship knowledge...');
  
  const schema = loadRealSchema();
  const examples = loadRealExamples();
  const docs = loadRealDocs();
  const cliData = performCLIIntrospection();
  
  console.log(`✓ Loaded schema`);
  console.log(`✓ Loaded ${examples.size} examples`);
  console.log(`✓ Loaded ${docs.size} documentation files`);
  console.log(`✓ Loaded current CLI capabilities (${cliData.version.version})`);

  // Generate TypeScript module
  let moduleContent = `// Auto-generated embedded knowledge - DO NOT EDIT MANUALLY
// Generated on ${new Date().toISOString()}
// CLI Version: ${cliData.version.version}
// Git Commit: ${cliData.version.gitCommit}

export const EMBEDDED_SCHEMA = ${JSON.stringify(schema, null, 2)};

export const EMBEDDED_CLI_DATA: any = ${JSON.stringify(cliData, null, 2)};

export const EMBEDDED_EXAMPLES = new Map([
`;

  for (const [name, content] of examples) {
    moduleContent += `  ["${name}", ${JSON.stringify({ content, path: "embedded" }, null, 2)}],\n`;
  }

  moduleContent += `]);

export const EMBEDDED_DOCS = new Map([
`;

  for (const [name, content] of docs) {
    moduleContent += `  ["${name}", ${JSON.stringify(content, null, 2)}],\n`;
  }

  moduleContent += `]);
`;

  return moduleContent;
}

function main() {
  try {
    console.log('🚀 Embedding Rocketship knowledge into MCP server...');
    
    const knowledgeModule = generateKnowledgeModule();
    const outputPath = path.join(MCP_SRC, 'embedded-knowledge.ts');
    
    fs.writeFileSync(outputPath, knowledgeModule);
    console.log(`✅ Generated ${outputPath}`);
    
    console.log('🎉 Knowledge embedding complete!');
  } catch (error) {
    console.error('❌ Failed to embed knowledge:', error.message);
    process.exit(1);
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main();
}