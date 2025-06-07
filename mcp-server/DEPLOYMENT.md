# Deployment Guide for Rocketship MCP Server

## Publishing to NPM

### Prerequisites

1. **NPM Account**: Create an account at [npmjs.com](https://npmjs.com)
2. **NPM Token**: Generate an automation token for CI/CD
3. **GitHub Secrets**: Add `NPM_TOKEN` to repository secrets

### Manual Deployment

```bash
# 1. Navigate to the TypeScript MCP server directory
cd mcp-server-js

# 2. Install dependencies
npm install

# 3. Build the project
npm run build

# 4. Test the build
npm run test

# 5. Update version
npm version patch  # or minor/major

# 6. Publish to NPM
npm publish --access public
```

### Automated Deployment (Recommended)

The repository includes a GitHub Actions workflow for automated publishing:

#### Option 1: Tag-based Release
```bash
# Create and push a version tag
git tag mcp-v1.0.0
git push origin mcp-v1.0.0

# This triggers automatic build and publish
```

#### Option 2: Manual Trigger
1. Go to GitHub Actions in your repository
2. Select "Publish MCP Server to NPM" workflow  
3. Click "Run workflow"
4. Enter the version number (e.g., 1.0.1)
5. Click "Run workflow"

## User Installation

Once published, users can install via:

### Zero-Install with npx (Recommended)
```json
{
  "mcpServers": {
    "rocketship": {
      "command": "npx",
      "args": ["-y", "@rocketship/mcp-server@latest"]
    }
  }
}
```

### Global Installation
```bash
npm install -g @rocketship/mcp-server

# Then in MCP config:
{
  "mcpServers": {
    "rocketship": {
      "command": "rocketship-mcp"
    }
  }
}
```

## Package Information

- **Package Name**: `@rocketship/mcp-server`
- **Registry**: [npmjs.com](https://www.npmjs.com/package/@rocketship/mcp-server)
- **Documentation**: Included in package README
- **License**: MIT

## Version Management

The package follows semantic versioning:

- **Patch** (1.0.X): Bug fixes, small improvements
- **Minor** (1.X.0): New features, backward compatible
- **Major** (X.0.0): Breaking changes

## Monitoring

After deployment:

1. **NPM Stats**: Monitor downloads at npmjs.com
2. **GitHub Releases**: Track versions and release notes
3. **User Feedback**: Monitor issues and discussions
4. **Usage Analytics**: Consider adding telemetry (opt-in)

## Rollback Procedure

If a release has issues:

```bash
# Deprecate the problematic version
npm deprecate @rocketship/mcp-server@1.0.1 "This version has known issues, use 1.0.0"

# Publish a fixed version
npm version patch
npm publish --access public
```

## Testing Before Release

Always test the package before publishing:

```bash
# 1. Build and test locally
npm run build
npm run test

# 2. Test the package installation
npm pack
npm install -g ./rocketship-mcp-server-1.0.0.tgz

# 3. Test MCP server functionality
rocketship-mcp --help

# 4. Clean up test installation
npm uninstall -g @rocketship/mcp-server
```

## Distribution Statistics

The package will be available via:

- **npm**: Primary distribution channel
- **GitHub Packages**: Mirror for enterprise users
- **CDN**: Automatic via npmjs.com CDN (unpkg, jsdelivr)

Users worldwide can access with zero installation using npx, making it as easy as the Supabase MCP server.