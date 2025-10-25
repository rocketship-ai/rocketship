# UX Improvement Plans

## Priority UX Improvements

### 1. ✅ Auto Browser Session Management

**Goal:** Users shouldn't need explicit start/stop steps. Framework auto-detects browser usage and manages sessions.

**Current State:**

- Playwright plugin requires explicit `role: start/stop` steps
- Each step must specify `session_id: "run-test-{{ .run.id }}"`
- Browser_use requires pre-existing session via `session_id`
- Agent requires `session_id` and MCP server config to use browser

**Implementation:**

- **DSL Parser Enhancement:**

  - Scan all test steps before execution
  - Detect browser-using plugins: `playwright`, `browser_use`, or `agent` with `capabilities: ["browser"]`
  - Auto-inject browser start at test beginning (before first step)
  - Auto-inject browser stop in `cleanup.always`
  - Generate session ID: `test-{{ .run.id }}` (stable per test run)

- **Plugin Changes:**

  - Make `session_id` optional in all browser plugins
  - If not provided, use auto-generated session ID from test context
  - **REMOVE** `role: start/stop` from playwright plugin entirely (breaking change, cleaner UX)
  - Keep `role: script` for running custom scripts against sessions

- **Headless Behavior:**
  - Default: `headless: true`
  - Scan ALL steps in test for `headless` config
  - If ANY step has `headless: false`, entire session is non-headless
  - Support template variables: `headless: "{{ .env.HEADLESS }}"`

**Implementation:**

- Remove `role: start/stop` from playwright plugin
- Keep `role: script` for custom browser scripts

---

### 2. ✅ Capabilities Array for Agent Plugin

**Goal:** Hide MCP server configuration complexity. Use simple capability strings instead.

**Current State:**

```yaml
# Agent plugin currently requires:
mcp_servers:
  playwright:
    type: stdio
    command: npx
    args: ["@playwright/mcp@0.0.43"]
```

**New Design:**

```yaml
# Simple capabilities array:
capabilities: ["browser"] # Auto-maps to playwright MCP v0.0.43
```

**Implementation:**

- **Agent Plugin Config:**

  - Add `Capabilities []string` field to `Config` struct
  - **REMOVE** `MCPServers` field entirely (breaking change, cleaner UX)
  - Map capability names to MCP server configs internally:
    - `"browser"` → Playwright MCP v0.0.43 (stdio, npx, pinned version)
    - `"supabase"` → Future Supabase MCP (when available)
  - Version pinning: Bundle specific MCP versions with Rocketship releases
  - No escape hatch for custom MCP servers (can add back later if needed)

- **Capability Resolution Logic:**

  ```go
  func resolveCapabilities(capabilities []string) (map[string]MCPServerConfig, error) {
      servers := make(map[string]MCPServerConfig)
      for _, cap := range capabilities {
          switch cap {
          case "browser":
              servers["playwright"] = MCPServerConfig{
                  Type: MCPServerTypeStdio,
                  Command: "npx",
                  Args: []string{"@playwright/mcp@0.0.43"},
              }
          case "supabase":
              // Future: Supabase MCP config
              return nil, fmt.Errorf("supabase capability not yet supported")
          default:
              return nil, fmt.Errorf("unknown capability: %s", cap)
          }
      }
      return servers, nil
  }
  ```

- **Migration Path:**
  - Remove `mcp_servers` field entirely
  - Users must migrate to `capabilities` array
  - Custom MCP servers not supported (simplified UX trade-off)

**Implementation:**

- Remove `mcp_servers` field from agent plugin
- Users will use `capabilities: ["browser"]` instead

---

### 3. ✅ Supabase Auto-Credentials from Environment

**Goal:** Stop repeating `url` and `key` in every Supabase step. Auto-detect from env vars.

**Current State:**

```yaml
# Every supabase step requires:
plugin: supabase
config:
  url: "{{ .env.SUPABASE_URL }}"
  key: "{{ .env.SUPABASE_SERVICE_KEY }}"
  operation: select
  # ... rest of config
```

**New Design:**

```yaml
# Just omit url/key - auto-detected from env:
plugin: supabase
config:
  operation: select
  # url and key auto-loaded from environment
  # ... rest of config
```

**Implementation:**

- **Supabase Plugin Enhancement:**

  - Make `URL` and `Key` optional in `SupabaseConfig`
  - Add environment auto-detection in `Activity()` method:

    ```go
    // Auto-detect URL if not provided
    if config.URL == "" {
        config.URL = os.Getenv("SUPABASE_URL")
        if config.URL == "" {
            return nil, fmt.Errorf("url is required (either in config or SUPABASE_URL env var)")
        }
    }

    // Auto-detect Key with precedence order
    if config.Key == "" {
        // Service keys (full access) take precedence
        if key := os.Getenv("SUPABASE_SECRET_KEY"); key != "" {
            config.Key = key
        } else if key := os.Getenv("SUPABASE_SERVICE_KEY"); key != "" {
            config.Key = key
        } else if key := os.Getenv("SUPABASE_PUBLISHABLE_KEY"); key != "" {
            config.Key = key
        } else if key := os.Getenv("SUPABASE_ANON_KEY"); key != "" {
            config.Key = key
        } else {
            return nil, fmt.Errorf("key is required (either in config or SUPABASE_*_KEY env var)")
        }
    }
    ```

- **Environment Variable Precedence:**

  - **Service keys (full database access):**
    1. `SUPABASE_SECRET_KEY` (newest naming)
    2. `SUPABASE_SERVICE_KEY` (legacy naming)
  - **Anon keys (RLS-restricted access):** 3. `SUPABASE_PUBLISHABLE_KEY` (newest naming) 4. `SUPABASE_ANON_KEY` (legacy naming)
  - **Rationale:** Prefer newer naming, but support all variants for backwards compat

- **Override Behavior:**
  - Explicit config values ALWAYS override environment variables
  - Template variables still work: `url: "{{ .env.CUSTOM_SUPABASE_URL }}"`

**Docs Update:** Document supported env var names and precedence

---

### 4. ⏸️ Headless Configuration (Covered by #1)

Headless is handled as part of auto browser session management.

**Key Details:**

- Playwright plugin already supports `headless: bool` in start config
- Browser_use doesn't need it (uses existing session)
- Agent doesn't need it (uses MCP to existing session)
- Auto-session logic will:
  - Default `headless: true`
  - Scan all test steps for any `headless: false`
  - If found, start browser as non-headless
  - Support template variables for CI vs local detection

---

## Implementation Phases

### Phase 1: Supabase Auto-Credentials (Easiest, High Impact)

- ✅ Isolated change to supabase plugin
- ✅ No breaking changes
- ✅ Immediate UX improvement in all Supabase tests
- **Files to modify:**
  - `internal/plugins/supabase/supabase.go` - Add env detection logic
  - `docs/` - Update Supabase plugin documentation

### Phase 2: Agent Capabilities Array (Medium Complexity)

- ✅ Agent plugin only
- ✅ No breaking changes (additive feature)
- ✅ Significant DX improvement for agent-based tests
- **Files to modify:**
  - `internal/plugins/agent/types.go` - Add Capabilities field
  - `internal/plugins/agent/agent.go` - Add capability resolution
  - `examples/agent-browser-testing/` - Simplify examples
  - `docs/` - Update agent plugin documentation

### Phase 3: Auto Browser Session Management (Most Complex)

- ⚠️ Touches DSL parser and multiple plugins
- ⚠️ Potential breaking changes if we remove explicit start/stop
- ✅ Largest UX impact - eliminates most browser boilerplate
- **Files to modify:**
  - `internal/dsl/parser.go` - Add browser detection and auto-injection
  - `internal/plugins/playwright/playwright.go` - Make session_id optional
  - `internal/plugins/browser_use/browser_use.go` - Make session_id optional
  - `internal/plugins/agent/agent.go` - Make session_id optional with browser capability
  - `internal/orchestrator/` - Pass test context with auto-session ID
  - `examples/` - Simplify all browser examples
  - `docs/` - Update browser testing documentation

---

## File Impact Summary

### High Priority Files (Phase 1-2):

1. `internal/plugins/supabase/supabase.go` - Env auto-detection
2. `internal/plugins/agent/types.go` - Capabilities field
3. `internal/plugins/agent/agent.go` - Capability resolution

### Medium Priority Files (Phase 3):

4. `internal/dsl/parser.go` - Browser auto-injection
5. `internal/plugins/playwright/playwright.go` - Optional session_id
6. `internal/plugins/browser_use/browser_use.go` - Optional session_id

### Documentation Updates:

- `docs/plugins/supabase.md` - Environment variables
- `docs/plugins/agent.md` - Capabilities array
- `docs/examples/browser-testing.md` - Simplified examples
- All example YAML files

---

## Edge Cases & Testing Strategy

### Feature 1: Auto Browser Session Management

**Edge Cases to Test:**

1. **Multiple browser-using plugins in same test**

   - ✅ Test: playwright script + browser_use + agent with browser capability
   - ✅ Expected: All share the same session
   - ⚠️ Edge case: What if steps have conflicting `headless` values?
   - ✅ Solution: ANY `headless: false` makes entire session non-headless

2. **Explicit session_id override**

   - ✅ Test: User provides `session_id: "custom-{{ .run.id }}"` in one step
   - ⚠️ Edge case: Should we allow overriding auto-session?
   - ✅ Solution: Explicit session_id takes precedence (advanced users)
   - ⚠️ Warning: Mixing auto and explicit session IDs could break things

3. **Cleanup failure scenarios**

   - ✅ Test: Browser crashes mid-test
   - ✅ Test: User kills rocketship process
   - ⚠️ Edge case: Orphaned browser processes
   - ✅ Solution: cleanup.always should handle session file cleanup
   - ⚠️ TODO: Verify lifecycle hooks work when process is killed

4. **Parallel test execution**

   - ✅ Test: Multiple tests running in parallel, each needs isolated session
   - ⚠️ Edge case: Session ID collision
   - ✅ Solution: Include `.run.id` in session ID (unique per run)
   - ✅ Test: Verify session file isolation

5. **No browser plugins in test**

   - ✅ Test: Test with only HTTP, SQL plugins
   - ✅ Expected: No browser start/stop injected
   - ✅ Test: Verify no performance overhead from detection

6. **Template variable errors in headless config**

   - ✅ Test: `headless: "{{ .env.MISSING_VAR }}"`
   - ⚠️ Edge case: Undefined variable
   - ✅ Solution: DSL template system should error with clear message

7. **Explicit start/stop steps**
   - ⚠️ Edge case: User tries to use `role: start/stop`
   - ✅ Solution: Error message
   - ✅ Error: "Invalid role. Only 'script' is supported. Browser sessions are auto-managed."

---

### Feature 2: Agent Capabilities Array

**Edge Cases to Test:**

1. **Unknown capability name**

   - ✅ Test: `capabilities: ["invalid"]`
   - ✅ Expected: Clear error message listing valid capabilities
   - ✅ Test: Error message quality

2. **Empty capabilities array**

   - ✅ Test: `capabilities: []`
   - ✅ Expected: No MCP servers configured (agent runs without tools)
   - ✅ Test: Agent still works for prompt-only tasks

3. **Duplicate capabilities**

   - ✅ Test: `capabilities: ["browser", "browser"]`
   - ✅ Expected: Deduplicated to single MCP server
   - ✅ Test: No duplicate server processes

4. **Version conflicts**

   - ⚠️ Edge case: User has different playwright MCP version installed globally
   - ✅ Solution: Pin version in command: `npx @playwright/mcp@0.0.43`
   - ✅ Test: Verify correct version is used

5. **MCP server startup failure**

   - ✅ Test: playwright MCP binary not available (npx fails)
   - ✅ Expected: Clear error with installation instructions
   - ✅ Test: Error message clarity

6. **Capability with session_id interaction**

   - ✅ Test: `capabilities: ["browser"]` + `session_id: "test"`
   - ✅ Expected: Agent connects to existing browser session via CDP
   - ✅ Test: Verify CDP connection works
   - ⚠️ Edge case: Session doesn't exist yet
   - ✅ Solution: Error with clear message about session lifecycle

7. **Unknown mcp_servers field**
   - ⚠️ Edge case: User tries to use `mcp_servers` field
   - ✅ Solution: Error message
   - ✅ Error: "Unknown field 'mcp_servers'. Use 'capabilities' instead."

---

### Feature 3: Supabase Auto-Credentials

**Edge Cases to Test:**

1. **No environment variables set**

   - ✅ Test: Run with empty env
   - ✅ Expected: Clear error listing all supported env vars
   - ✅ Test: Error message quality

2. **Multiple key types set simultaneously**

   - ✅ Test: Set all 4 key env vars (SECRET, SERVICE, PUBLISHABLE, ANON)
   - ✅ Expected: Uses SECRET_KEY (highest precedence)
   - ✅ Test: Log which key was selected (debug mode)

3. **Explicit config overrides env**

   - ✅ Test: Set `SUPABASE_URL` env but provide explicit `url: "custom"`
   - ✅ Expected: Uses explicit config value
   - ✅ Test: Verify override behavior

4. **Template variables in config**

   - ✅ Test: `url: "{{ .env.CUSTOM_SUPABASE_URL }}"`
   - ✅ Expected: Template processing happens first, then env fallback
   - ✅ Test: Order of operations

5. **Invalid URL/Key values from env**

   - ✅ Test: `SUPABASE_URL=not-a-url`
   - ✅ Expected: Supabase API returns clear error
   - ✅ Decision: No pre-validation, let API handle it

6. **Sensitive data in logs**

   - ⚠️ Security edge case: API keys in debug logs
   - ✅ Solution: Redact keys in log messages (show first 4 chars only)
   - ✅ Test: Verify no full keys in logs even with --debug

7. **Different key types mid-test**

   - ✅ Test: Step 1 uses SERVICE_KEY, Step 2 sets different key in config
   - ✅ Expected: Each step independent, uses its own config
   - ✅ Test: Verify step isolation

8. **Race conditions with env var changes**
   - ⚠️ Edge case: User modifies .env file during test execution
   - ✅ Solution: Env vars read once at process start (Go os.Getenv)
   - ✅ Test: Verify env changes don't affect running tests

---

## Integration Testing Matrix

**Test combinations across all features:**

| Test Scenario            | Browser Auto                  | Capabilities                     | Supabase Auto-Creds                |
| ------------------------ | ----------------------------- | -------------------------------- | ---------------------------------- |
| All features together    | ✅ Auto browser session       | ✅ Agent with browser capability | ✅ Auto-detect SUPABASE_SECRET_KEY |
| Browser + Supabase       | ✅ Playwright script          | N/A                              | ✅ Auto-detect from env            |
| Agent only               | ✅ Auto-start for browser cap | ✅ Browser capability            | N/A                                |
| Explicit config override | N/A (auto-only)               | N/A (auto-only)                  | ✅ Explicit overrides env          |

**Cross-feature edge cases:**

1. **Agent with browser capability + auto-session**

   - ✅ Test: Agent step with `capabilities: ["browser"]`
   - ✅ Expected: Auto-started browser session via session_id
   - ✅ Expected: Agent MCP connects via CDP to same session
   - ✅ Test: Verify both use same browser instance

2. **Headless detection with capabilities**

   - ✅ Test: Agent step with `capabilities: ["browser"]` and `headless: false`
   - ⚠️ Edge case: How does agent plugin handle headless config?
   - ✅ Solution: Agent doesn't configure browser, only auto-session does
   - ✅ Test: Verify agent ignores headless, auto-session uses it

3. **Template variable precedence**

   - ✅ Test: `session_id: "{{ .env.SESSION }}"` with auto-session enabled
   - ✅ Expected: Template variable resolved first
   - ✅ Test: If env var exists, uses it; else uses auto-generated

4. **Error propagation across layers**
   - ✅ Test: Supabase auto-cred fails + browser session auto-start fails
   - ✅ Expected: Clear error message indicating which feature failed
   - ✅ Test: Error messages don't confuse users

---

## Regression Testing Checklist

**Ensure existing functionality still works:**

- [ ] Explicit Supabase url/key configs (still supported as override)
- [ ] Template variables in all config fields
- [ ] Environment variable substitution via DSL
- [ ] Save configs and assertions
- [ ] Parallel test execution
- [ ] Cleanup hooks (always/on_success/on_failure)
- [ ] Multi-step workflows with state passing
- [ ] All existing example tests pass

---

## Implementation Complexity

### Phase 1: Supabase Auto-Credentials

**Complexity:** LOW
**Risk:** LOW
**Impact:** HIGH

**Why low complexity:**

- Single file change (supabase.go)
- No breaking changes
- Simple env var lookup logic

**Implementation checklist:**

- [ ] Add env detection in `Activity()` method
- [ ] Test all 4 key types (precedence order)
- [ ] Test explicit override behavior
- [ ] Add key redaction in debug logs
- [ ] Update docs

---

### Phase 2: Agent Capabilities Array

**Complexity:** MEDIUM
**Risk:** LOW
**Impact:** HIGH

**Why medium complexity:**

- Remove `MCPServers` field
- Add `Capabilities` field
- Capability resolution logic
- Simple error handling

**Implementation checklist:**

- [ ] Add `Capabilities []string` to Config struct
- [ ] Remove `MCPServers` field
- [ ] Add capability resolution function
- [ ] Map "browser" → Playwright MCP v0.0.43
- [ ] Deduplicate capabilities
- [ ] Error if unknown capability
- [ ] Error if unknown field mcp_servers
- [ ] Update all examples
- [ ] Update docs

---

### Phase 3: Auto Browser Session Management

**Complexity:** HIGH
**Risk:** MEDIUM
**Impact:** VERY HIGH

**Why high complexity:**

- DSL parser changes (test scanning/injection)
- Multiple plugin changes (playwright, browser_use, agent)
- Headless aggregation logic
- Session ID generation and passing
- Cleanup injection
- Simple error handling

**Implementation checklist:**

- [ ] DSL: Scan test steps for browser-using plugins
- [ ] DSL: Detect `capabilities: ["browser"]` in agent steps
- [ ] DSL: Generate session ID `test-{{ .run.id }}`
- [ ] DSL: Scan all steps for `headless` config
- [ ] DSL: Aggregate headless (ANY false = all false)
- [ ] DSL: Inject browser start before first step
- [ ] DSL: Inject browser stop in cleanup.always
- [ ] DSL: Pass session ID to all browser steps via context
- [ ] Playwright: Make session_id optional (use from context)
- [ ] Playwright: Error if role is start/stop
- [ ] Playwright: Keep role=script working
- [ ] Browser_use: Make session_id optional (use from context)
- [ ] Agent: Make session_id optional when using browser capability
- [ ] Agent: Auto-connect to session via CDP when capability=browser
- [ ] Test: Playwright script + browser_use + agent all share session
- [ ] Test: Headless aggregation across steps
- [ ] Test: Parallel tests have isolated sessions
- [ ] Test: Cleanup works even if test fails
- [ ] Update all browser examples
- [ ] Update docs

---

## Other UX Issues (Lower Priority)

- why is generate-plugin-reference.py not working? I dont see the browser_use plugin. ✅
- lifecycle clean up hook does not seem to work if i kill the run (need to test this to confirm)
- can we have the agent plugin have the same env override that the supabase plugin has? Meaning that it by default inherits the ANTHROPIC_API_KEY from the environment, but it should be possible to override it in the config of the plugin.
- can we have the agent plugin use a CC plan instead of the anthropic API key?
- I wanna re organize the docs. A plugins section should exist. And fix up this doc page https://docs.rocketship.sh/examples/ai/browser-testing/
- create ROCKETSHIP_QUICKSTART.md
