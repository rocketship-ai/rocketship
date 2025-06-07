# Local Testing Guide

## Quick Test Steps

```bash
# 1. Navigate to MCP server directory
cd mcp-server

# 2. Install dependencies
npm install

# 3. Build TypeScript
npm run build

# 4. Run basic tests
npm run test

# 5. Test package creation
npm pack

# 6. Verify generated files
ls -la dist/
ls -la *.tgz
```

## Expected Output

After running the tests, you should see:
```
🚀 Testing Rocketship MCP Server...

📦 Testing server initialization...
✅ Server initialized successfully

⚙️  Testing test generation...
✅ Test generation works

✅ All tests passed! MCP server is ready to use.
```

## Verify Build Artifacts

The `dist/` directory should contain:
- `index.js` - Compiled server
- `index.d.ts` - TypeScript declarations  
- `test.js` - Compiled test file

## Local Installation Test

```bash
# Install the package locally
npm install -g ./rocketshipai-mcp-server-0.1.0.tgz

# Test the binary
rocketship-mcp --help

# Should show MCP server startup (will wait for stdio input)
# Press Ctrl+C to exit

# Clean up
npm uninstall -g @rocketshipai/mcp-server
rm *.tgz
```

## Integration Test

To test with an actual MCP client:

1. Build and pack the package locally
2. Install globally from the .tgz file
3. Add to your MCP configuration:
   ```json
   {
     "mcpServers": {
       "rocketship-local": {
         "command": "rocketship-mcp"
       }
     }
   }
   ```
4. Test with your coding agent

## Troubleshooting

- **Build fails**: Check TypeScript errors with `npx tsc --noEmit`
- **Import errors**: Verify all dependencies are installed
- **Binary not found**: Ensure global npm bin directory is in PATH
- **MCP connection issues**: Check that the binary runs without errors