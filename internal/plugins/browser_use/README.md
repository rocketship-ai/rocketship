# browser_use Plugin

⚠️ **PERFORMANCE WARNING**: This plugin is **not recommended** for most browser testing use cases.

## Recommendation: Use the Agent Plugin Instead

The **`agent` plugin** provides superior browser testing capabilities with better performance and flexibility:

**Why Agent Plugin is Better:**
- ✅ **Faster execution** - Direct Claude API calls vs. browser-use library overhead
- ✅ **More capable** - Claude Sonnet 4.5 (256k context) vs. GPT-4o/Claude via browser-use
- ✅ **Better tool integration** - Native MCP server support (Playwright MCP, filesystem, APIs, etc.)
- ✅ **Flexible workflows** - Combine browser testing with database queries, API calls, file operations
- ✅ **Lower latency** - Single-step inference vs. multi-step browser-use agent loop
- ✅ **Better reliability** - Fewer moving parts, cleaner architecture

**Example with Agent Plugin:**
```yaml
steps:
  - name: "Start browser session"
    plugin: playwright
    config:
      role: start
      session_id: "test-{{ .run.id }}"
      headless: false

  - name: "Test with Claude agent"
    plugin: agent
    config:
      prompt: |
        Navigate to https://example.com and verify:
        1. Page title is "Example Domain"
        2. Heading contains "Example Domain"
        3. Paragraph has documentation link
      session_id: "test-{{ .run.id }}"
      mcp_servers:
        playwright:
          type: stdio
          command: npx
          args: ["@playwright/mcp@0.0.43"]
```

## When to Use browser_use Plugin

Only use this plugin if you specifically need:
- **Multi-modal vision** for complex visual understanding tasks
- **OpenAI GPT-4o** specifically (though agent plugin supports Claude which is often better)
- **Legacy compatibility** with existing browser-use workflows

## Performance Comparison

| Feature | browser_use | agent (Playwright MCP) |
|---------|------------|----------------------|
| **Speed** | Slow (multi-step agent loop) | Fast (single inference) |
| **Context** | Limited by model | 256k tokens (Claude Sonnet 4.5) |
| **Reliability** | Lower (more dependencies) | Higher (simpler stack) |
| **Capabilities** | Browser only | Browser + files + APIs + more |
| **Cost** | Higher (multiple LLM calls) | Lower (single call) |

## Migration Guide

If you have existing `browser_use` tests, migrating to `agent` is straightforward:

**Before (browser_use):**
```yaml
- plugin: browser_use
  config:
    session_id: "test"
    task: "Find the login button and click it"
    max_steps: 5
    llm:
      provider: "openai"
      model: "gpt-4o"
```

**After (agent with Playwright MCP):**
```yaml
- plugin: agent
  config:
    session_id: "test"
    prompt: "Find the login button and click it"
    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@0.0.43"]
```

## See Also

- [Agent Plugin Documentation](../agent/) - Recommended alternative
- [Browser Testing Guide](../../../docs/src/examples/ai/browser-testing.md) - Usage examples
- [Playwright Plugin](../playwright/) - For deterministic browser scripting
