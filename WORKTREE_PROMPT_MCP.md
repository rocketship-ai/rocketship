# Prompt for Claude Code Instance - MCP Server Auto-Generation

You are working in a git worktree called `rocketship-mcp-server` on branch `auto-gen-mcp`. 

## Your Task
Improve the Rocketship MCP server's auto-generation process to ensure it stays up-to-date with CLI capabilities on every PR merge.

**Problem Statement:**
The Rocketship MCP server currently has stale information about CLI capabilities, leading to incorrect suggestions. Examples of issues found:

1. **Wrong installation method**: Suggested `brew upgrade rocketship` when Rocketship isn't available via Homebrew
2. **Outdated flags**: Suggested `--vars` flag when it should be `--var-file`
3. **Missing features**: MCP server knowledge doesn't reflect current CLI capabilities

**Requirements:**
1. Analyze the current MCP server update mechanism (`.github/workflows/release.yml`)
2. Create automation that updates MCP server knowledge on every PR merge (not just releases)
3. Ensure the MCP server accurately reflects current CLI help text, flags, and capabilities
4. Fix the specific issues mentioned above
5. Create a robust system that auto-extracts CLI information and embeds it in the MCP server
6. Test the updated MCP server to ensure it provides accurate information

## Initial Setup
Before starting work, you MUST set up your isolated Docker environment:

```bash
# Run this first to create your isolated environment
./docker/setup-worktree-env.sh

# Then start your services
cd .docker && docker-compose up -d

# Verify everything is running
docker ps | grep rocketship-mcp-server
```

After setup, you'll have your own isolated:
- Temporal server
- Rocketship engine and worker
- Test databases
- Unique ports that won't conflict with other instances

## Development Workflow
1. Build and test the current MCP server:
   ```bash
   cd mcp-server
   npm install
   npm run build
   npm test
   ```

2. Test your worktree-specific CLI for extracting current capabilities:
   ```bash
   .docker/docker-rocketship-local.sh --help
   .docker/docker-rocketship-local.sh run --help
   .docker/docker-rocketship-local.sh version
   ```

3. After making changes to Go code, rebuild:
   ```bash
   cd .docker
   docker-compose build engine worker
   docker build -f Dockerfile.cli -t rocketship-mcp-server-cli:latest ..
   docker-compose restart engine worker
   ```

4. Test the updated MCP server:
   ```bash
   cd mcp-server
   npm run build
   npm test
   ```

## Key Areas to Investigate

### Current MCP Server Structure:
- **`mcp-server/src/embedded-knowledge.ts`** - Current knowledge base
- **`mcp-server/scripts/embed-knowledge.js`** - Knowledge embedding script
- **`.github/workflows/release.yml`** - Current update mechanism
- **`mcp-server/src/index.ts`** - Main MCP server implementation

### CLI Information Sources:
- **Help text extraction**: `rocketship --help`, `rocketship [command] --help`
- **Command structure**: Available commands and subcommands
- **Flag documentation**: Current flags and their usage
- **Installation methods**: How users actually install/update Rocketship
- **Version information**: Current version and release process

### Automation Points:
- **PR merge triggers**: When to update MCP server knowledge
- **CLI introspection**: Automatically extract current CLI capabilities
- **Knowledge packaging**: How to embed fresh knowledge into MCP server
- **Deployment**: How to deploy updated MCP server

## Technical Implementation Strategy

1. **Analyze Current Process**:
   - Examine `.github/workflows/release.yml`
   - Understand how `embed-knowledge.js` works
   - Map out the current knowledge update flow

2. **Create CLI Introspection**:
   - Build scripts to automatically extract help text
   - Generate structured data about commands, flags, examples
   - Capture installation/update methods
   - Extract version and capability information

3. **Enhance Automation**:
   - Create GitHub workflow that runs on PR merge
   - Automatically regenerate MCP server knowledge
   - Deploy updated MCP server
   - Ensure knowledge stays current

4. **Fix Specific Issues**:
   - Correct installation instructions (no Homebrew)
   - Update flag documentation (`--var-file` not `--vars`)
   - Add any missing commands or capabilities

## Testing Requirements

1. **MCP Server Testing**:
   - Test that MCP server provides correct CLI information
   - Verify installation instructions are accurate
   - Check that all current flags are documented correctly

2. **Integration Testing**:
   - Test the auto-generation workflow
   - Verify knowledge updates on CLI changes
   - Ensure deployment process works

3. **User Experience Testing**:
   - Simulate the problematic interactions from the examples
   - Verify they now work correctly
   - Test with various CLI scenarios

## Submission Process
After implementing your changes and verifying they work:

1. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: implement auto-generation for MCP server knowledge

   - Add automated CLI introspection for current capabilities
   - Create workflow to update MCP server on every PR merge
   - Fix installation instructions and flag documentation
   - Ensure MCP server knowledge stays current with CLI
   - Add comprehensive testing for accuracy"
   git push origin auto-gen-mcp
   ```

2. **Create a Pull Request**:
   ```bash
   # Use GitHub MCP to create the PR
   gh pr create --title "feat: Auto-generate MCP server knowledge from CLI" --body "Implements automated knowledge extraction and updating for the Rocketship MCP server.

   **Problem Solved:**
   - MCP server had stale information about CLI capabilities
   - Wrong installation methods and outdated flags
   - Manual knowledge updates were inconsistent

   **Solution:**
   - Automated CLI introspection to extract current capabilities
   - GitHub workflow to update MCP server on every PR merge
   - Accurate installation and usage information
   - Robust testing to ensure accuracy

   **Key Changes:**
   - Enhanced embed-knowledge script with CLI introspection
   - New GitHub workflow for automated updates
   - Fixed specific issues: installation methods, --var-file flag
   - Comprehensive testing for MCP server accuracy"
   ```

3. **Monitor CI Status**:
   You MUST monitor the GitHub PR check workflow and ensure it passes. Use the GitHub MCP server to:
   
   - Poll workflow status every minute until completion
   - Check that your new automation workflows work correctly
   - Verify the MCP server tests pass
   - Ensure CLI integration tests still work
   - Test the deployed MCP server for accuracy

   Keep polling until:
   - ✅ All checks pass (CI is green)
   - ✅ MCP server automation works correctly
   - ✅ Knowledge extraction and embedding succeeds
   - ❌ If any checks fail, investigate logs, fix issues, and push again

## Important Notes
- You're in an isolated environment - your containers won't interfere with other instances
- Your Temporal UI will be on a unique port (shown after setup script)
- **CRITICAL**: The MCP server must provide accurate, current CLI information
- Test thoroughly with the specific problem scenarios provided
- Consider backward compatibility and deployment implications
- The automation should be robust and handle CLI changes gracefully
- Focus on making the MCP server a reliable source of current Rocketship information

## Success Criteria
1. MCP server provides correct installation instructions
2. All CLI flags and commands are accurately documented
3. Automation updates MCP server knowledge on every relevant change
4. The specific issues from the examples are resolved
5. System is robust and maintainable for future CLI evolution

Please investigate the current system, implement the auto-generation improvements, and ensure the MCP server becomes a reliable, always-current source of Rocketship CLI information.