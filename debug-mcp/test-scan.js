import { spawn } from 'child_process';
import fs from 'fs';

const server = spawn('rocketship-mcp', [], { stdio: ['pipe', 'pipe', 'pipe'] });

const request = {
  jsonrpc: '2.0',
  id: 1,
  method: 'tools/call',
  params: {
    name: 'scan_and_generate_test_suite',
    arguments: {
      project_root: '.',
      environments: ['staging'],
      codebase_analysis: JSON.parse(fs.readFileSync('mock-analysis.json', 'utf8'))
    }
  }
};

let output = '';
server.stdout.on('data', (data) => { output += data.toString(); });

server.stdin.write(JSON.stringify(request) + '\n');

setTimeout(() => {
  server.kill();
  
  // Show generated files
  if (fs.existsSync('.rocketship/api-tests/rocketship.yaml')) {
    console.log('=== Generated API test file content ===');
    console.log(fs.readFileSync('.rocketship/api-tests/rocketship.yaml', 'utf8'));
  }
}, 2000);
EOF < /dev/null