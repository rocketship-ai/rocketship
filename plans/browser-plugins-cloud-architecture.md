# Browser Plugins Cloud Architecture Plan

**Status:** Planning
**Created:** 2025-10-16
**Context:** Current browser plugin architecture assumes single-worker execution and will not work in cloud deployment with multiple Temporal workers.

---

## Problem Statement

The current browser plugin architecture (playwright and browser_use) will **FAIL** in a cloud environment where Temporal distributes activities across multiple workers.

### Why Current Architecture Fails

**Assumptions that break in cloud:**

1. **Local filesystem session storage** - Sessions saved to `/tmp/` on one worker cannot be accessed by another worker
2. **Local process management** - Browser processes launched on Worker A cannot be accessed from Worker B
3. **Localhost CDP endpoints** - `localhost:9222` on Worker A ≠ `localhost:9222` on Worker B
4. **Activity affinity** - Temporal makes no guarantees that sequential activities run on the same worker

### Example Failure Scenario

```yaml
steps:
  - name: "start browser"        # Temporal assigns to Worker A
    plugin: playwright
    config:
      role: start
      session_id: "test-session"

  - name: "navigate"              # Temporal assigns to Worker B
    plugin: playwright
    config:
      role: script
      session_id: "test-session"  # ❌ Session file not found
      script: "page.goto('https://example.com')"
```

**What happens:**
1. Worker A launches browser, saves session to `/tmp/playwright-session-test-session.json`
2. Worker B tries to load session from its own `/tmp/` - file doesn't exist
3. Even if session data was passed, `localhost:9222` on Worker B has no browser listening
4. Test fails with "session not found" or "connection refused"

---

## Code That Won't Work

### 1. Local File Storage

**File:** `internal/plugins/playwright/playwright.go:471-480`

```go
func (p *PlaywrightPlugin) saveSession(sessionID string, data SessionData) error {
    // ❌ BROKEN: os.TempDir() is local to each worker machine
    sessionFile := filepath.Join(os.TempDir(),
        fmt.Sprintf("playwright-session-%s.json", sessionID))

    // ❌ BROKEN: File only exists on THIS worker's filesystem
    if err := os.WriteFile(sessionFile, sessionJSON, 0600); err != nil {
        return fmt.Errorf("failed to write session file: %w", err)
    }
    return nil
}

func (p *PlaywrightPlugin) loadSession(sessionID string) (SessionData, error) {
    sessionFile := filepath.Join(os.TempDir(),
        fmt.Sprintf("playwright-session-%s.json", sessionID))

    // ❌ BROKEN: File doesn't exist if different worker
    data, err := os.ReadFile(sessionFile)
    if err != nil {
        return SessionData{}, fmt.Errorf("session not found: %w", err)
    }
    // ...
}
```

### 2. Local Process Management

**File:** `internal/plugins/playwright/playwright.go:504-530`

```go
case "stop":
    sessionData, err := p.loadSession(sessionID)

    if sessionData.PID > 0 {
        // ❌ BROKEN: Process exists on different worker machine
        process, err := os.FindProcess(sessionData.PID)
        if err == nil {
            // ❌ BROKEN: Can't kill process on remote machine
            process.Signal(syscall.SIGTERM)
        }
    }
```

### 3. Localhost CDP Connections

**Pattern in both `playwright.go` and `browser_use.go`:**

```go
// Browser launched with localhost endpoint
cdpEndpoint := fmt.Sprintf("http://localhost:%d", port)

// Saved to session
sessionData := SessionData{
    CDPEndpoint: cdpEndpoint,  // ❌ BROKEN: Only valid on THIS worker
    UserDataDir: userDataDir,  // ❌ BROKEN: Local filesystem path
    PID:         cmd.Process.Pid,  // ❌ BROKEN: Local process
}

// Later activity on different worker
playwright.connect(sessionData.CDPEndpoint)  // ❌ BROKEN: Connection refused
```

---

## Solution Options

### Option 1: Temporal Session Affinity (Quick Fix)

**Concept:** Use Temporal's session framework to ensure all browser activities for a session run on the same worker.

**Pros:**
- Minimal code changes
- Gets cloud working quickly
- Browser processes stay local (simple)
- Can keep file-based session as fallback

**Cons:**
- Reduces parallelism (all browser steps serialized on one worker)
- Worker failure kills all sessions on that worker
- Doesn't scale well for heavy browser workloads
- Still has single point of failure per session

**Implementation Complexity:** Low (1-2 days)

---

### Option 2: Remote Browser Service (Production-Grade)

**Concept:** Separate browser infrastructure from workers. Workers connect to remote browser pool via network.

**Architecture:**

```
┌─────────────┐
│  Worker A   │──┐
└─────────────┘  │
                 │    ┌──────────────────────────┐
┌─────────────┐  ├───→│  Browser Pool Service    │
│  Worker B   │──┤    │  (Browserless/Custom)    │
└─────────────┘  │    │                          │
                 │    │  Browser 1 (chromium)    │
┌─────────────┐  │    │  Browser 2 (chromium)    │
│  Worker C   │──┘    │  Browser 3 (chromium)    │
└─────────────┘       └──────────────────────────┘
                                 │
                      Accessible via ws://browser-pool:3000/session/{id}
```

**Pros:**
- Workers are completely stateless
- Activities can run on any worker (better parallelism)
- Browser resource management separate from worker capacity
- Better isolation and resource limits
- Easier to scale browsers independently
- Industry standard pattern

**Cons:**
- More infrastructure to manage (browser pool service)
- Network latency for CDP commands (usually negligible)
- Need to handle browser service failures
- More complex deployment

**Implementation Complexity:** Medium (1 week)

---

### Option 3: Hybrid Migration Path (Recommended)

**Concept:** Incremental migration from local to remote browsers.

**Phase 1: Add Session Affinity (1-2 days)**
- Use Temporal sessions to route activities
- Keep current file-based approach
- Minimal changes to get cloud working

**Phase 2: Migrate Session State to Workflow (2-3 days)**
- Store session metadata in Temporal workflow state
- Remove dependency on local files
- Still uses worker affinity

**Phase 3: Add Remote Browser Support (1 week)**
- Deploy browser pool service
- Update plugins to support both local and remote modes
- Gradual migration based on config flag

**Total Timeline:** 2-3 weeks for full migration

---

## Recommended Implementation: Phase 1 (Session Affinity)

### Step 1: Update Worker Registration

**File:** `cmd/worker/main.go`

```go
func main() {
    // ... existing setup ...

    w := worker.New(temporalClient, taskQueue, worker.Options{
        // Enable session support
        EnableSessionWorker: true,
        MaxConcurrentSessionExecutionSize: 10,

        // Existing options
        MaxConcurrentActivityExecutionSize: maxActivities,
    })

    // ... register activities ...
}
```

### Step 2: Create Session-Aware Workflow Wrapper

**File:** `internal/orchestrator/browser_session.go` (new file)

```go
package orchestrator

import (
    "fmt"
    "go.temporal.io/sdk/workflow"
)

// BrowserSessionState holds session metadata in workflow state
type BrowserSessionState struct {
    SessionID   string
    TaskQueue   string  // Which worker has this session
    CDPEndpoint string  // For future remote browser support
    WorkerID    string  // Which specific worker
}

// ExecuteWithBrowserSession ensures all activities for a browser session
// run on the same worker
func ExecuteWithBrowserSession(
    ctx workflow.Context,
    sessionID string,
    activities []ActivityConfig,
) error {
    // Create session-specific task queue
    sessionTaskQueue := fmt.Sprintf("browser-session-%s", sessionID)

    // Configure activities to use session task queue
    activityOptions := workflow.ActivityOptions{
        TaskQueue:           sessionTaskQueue,
        StartToCloseTimeout: time.Minute * 5,
    }
    ctx = workflow.WithActivityOptions(ctx, activityOptions)

    // Track session state in workflow
    var sessionState BrowserSessionState

    // Execute activities sequentially on same worker
    for _, activity := range activities {
        if activity.Plugin == "playwright" || activity.Plugin == "browser_use" {
            // First browser activity starts the session
            if activity.Config["role"] == "start" {
                err := workflow.ExecuteActivity(ctx, "PlaywrightActivity", activity.Config).
                    Get(ctx, &sessionState)
                if err != nil {
                    return fmt.Errorf("failed to start browser session: %w", err)
                }
            } else {
                // Pass session state to subsequent activities
                activity.Config["session_state"] = sessionState
                err := workflow.ExecuteActivity(ctx, "PlaywrightActivity", activity.Config).
                    Get(ctx, nil)
                if err != nil {
                    return fmt.Errorf("browser activity failed: %w", err)
                }
            }
        } else {
            // Non-browser activities can run anywhere
            err := workflow.ExecuteActivity(ctx, activity.Plugin+"Activity", activity.Config).
                Get(ctx, nil)
            if err != nil {
                return err
            }
        }
    }

    return nil
}
```

### Step 3: Update Plugin to Return Session State

**File:** `internal/plugins/playwright/playwright.go`

```go
// Update Execute to return session state for "start" role
func (p *PlaywrightPlugin) Execute(ctx context.Context, stepConfig interface{}, state map[string]interface{}) (interface{}, error) {
    // ... existing parsing ...

    switch role {
    case "start":
        // ... existing browser launch code ...

        // Save session data locally (still needed for this worker)
        sessionData := SessionData{
            CDPEndpoint: cdpEndpoint,
            UserDataDir: userDataDir,
            PID:         cmd.Process.Pid,
        }
        if err := p.saveSession(sessionID, sessionData); err != nil {
            return nil, err
        }

        // ALSO return session state to workflow
        // This allows workflow to track which worker has the session
        return map[string]interface{}{
            "session_id":    sessionID,
            "cdp_endpoint":  cdpEndpoint,
            "worker_id":     os.Getenv("HOSTNAME"), // Pod name in K8s
            "task_queue":    os.Getenv("TEMPORAL_TASK_QUEUE"),
        }, nil

    case "script", "stop":
        // Still try to load from local file first (for same worker)
        sessionData, err := p.loadSession(sessionID)
        if err != nil {
            // Fallback: check if session state was passed from workflow
            if sessionState, ok := stepConfig.(map[string]interface{})["session_state"]; ok {
                // This means we're on a different worker - should not happen with session affinity!
                return nil, fmt.Errorf("session affinity broken: session started on different worker")
            }
            return nil, fmt.Errorf("session not found: %w", err)
        }

        // ... rest of existing code ...
    }
}
```

### Step 4: Update Orchestrator to Use Session Workflow

**File:** `internal/orchestrator/orchestrator.go`

```go
func (o *Orchestrator) executeTestWorkflow(ctx workflow.Context, testSpec TestSpec) error {
    // Group steps by browser session
    sessionGroups := groupStepsByBrowserSession(testSpec.Steps)

    for sessionID, steps := range sessionGroups {
        if sessionID != "" {
            // Browser steps - use session affinity
            err := ExecuteWithBrowserSession(ctx, sessionID, steps)
            if err != nil {
                return err
            }
        } else {
            // Non-browser steps - execute normally
            for _, step := range steps {
                err := workflow.ExecuteActivity(ctx, step.Plugin+"Activity", step.Config).
                    Get(ctx, nil)
                if err != nil {
                    return err
                }
            }
        }
    }

    return nil
}

func groupStepsByBrowserSession(steps []Step) map[string][]ActivityConfig {
    // Extract browser session IDs and group related steps
    groups := make(map[string][]ActivityConfig)

    for _, step := range steps {
        if step.Plugin == "playwright" || step.Plugin == "browser_use" {
            sessionID := step.Config["session_id"].(string)
            groups[sessionID] = append(groups[sessionID], ActivityConfig{
                Plugin: step.Plugin,
                Config: step.Config,
            })
        } else {
            // Non-browser steps go in empty session group
            groups[""] = append(groups[""], ActivityConfig{
                Plugin: step.Plugin,
                Config: step.Config,
            })
        }
    }

    return groups
}
```

---

## Phase 2: Remote Browser Service (Future)

### Browser Pool Service Options

**Option A: Use Browserless (SaaS or self-hosted)**

```yaml
# docker-compose.yml or K8s deployment
services:
  browserless:
    image: browserless/chrome:latest
    ports:
      - "3000:3000"
    environment:
      MAX_CONCURRENT_SESSIONS: 10
      CONNECTION_TIMEOUT: 60000
      PREBOOT_CHROME: "true"
```

**Option B: Custom Service (More Control)**

Build a simple Go service that manages browser lifecycle:

```go
// Browser pool service
type BrowserPool struct {
    sessions map[string]*BrowserSession
    mu       sync.Mutex
}

type BrowserSession struct {
    ID          string
    CDPEndpoint string
    CreatedAt   time.Time
}

func (p *BrowserPool) CreateSession(opts SessionOptions) (*BrowserSession, error) {
    // Launch browser container/process
    // Return WebSocket endpoint accessible from any worker
}

func (p *BrowserPool) DeleteSession(id string) error {
    // Terminate browser and cleanup
}
```

### Update Plugins to Use Remote Browsers

```go
// Add config option for browser mode
type PlaywrightConfig struct {
    // ... existing fields ...

    BrowserMode string // "local" or "remote"
    BrowserPoolURL string // "http://browser-pool:3000" (only for remote mode)
}

func (p *PlaywrightPlugin) Execute(ctx context.Context, stepConfig interface{}, state map[string]interface{}) (interface{}, error) {
    // ... parse config ...

    switch role {
    case "start":
        if cfg.BrowserMode == "remote" {
            // Request browser from pool
            resp, err := http.Post(
                cfg.BrowserPoolURL+"/sessions",
                "application/json",
                sessionRequest,
            )

            var result struct {
                SessionID   string `json:"session_id"`
                CDPEndpoint string `json:"cdp_endpoint"` // ws://browser-pool:3000/session/abc
            }
            json.NewDecoder(resp.Body).Decode(&result)

            // Return remote endpoint
            return map[string]interface{}{
                "session_id":    result.SessionID,
                "cdp_endpoint":  result.CDPEndpoint,
                "browser_mode":  "remote",
            }, nil

        } else {
            // Existing local browser launch
            // ... current code ...
        }

    case "script":
        if sessionData.BrowserMode == "remote" {
            // Connect to remote browser via WebSocket
            playwright.connect(sessionData.CDPEndpoint)
        } else {
            // Connect to local browser
            playwright.connect("http://localhost:9222")
        }
    }
}
```

---

## Testing Strategy

### Phase 1 Testing (Session Affinity)

1. **Unit tests:** Verify session grouping logic
2. **Integration tests:** Deploy to minikube with 2+ workers, run browser tests
3. **Failure tests:** Kill a worker mid-session, verify error handling

### Phase 2 Testing (Remote Browsers)

1. **Local docker-compose:** Run browser pool + workers locally
2. **Load testing:** 10+ concurrent browser sessions
3. **Network failure tests:** Simulate browser pool downtime

---

## Configuration Migration

### Current (local-only)

```yaml
# rocketship.yaml
tests:
  - name: "browser test"
    steps:
      - plugin: playwright
        config:
          role: start
          session_id: "test-{{ .run.id }}"
```

### Phase 1 (session affinity)

```yaml
# No config changes needed!
# Session affinity handled automatically by orchestrator
tests:
  - name: "browser test"
    steps:
      - plugin: playwright
        config:
          role: start
          session_id: "test-{{ .run.id }}"
```

### Phase 2 (remote browsers)

```yaml
# Optional browser_mode config
tests:
  - name: "browser test"
    steps:
      - plugin: playwright
        config:
          role: start
          session_id: "test-{{ .run.id }}"
          browser_mode: "remote"  # NEW: Defaults to "local" for backwards compat
          browser_pool_url: "http://browser-pool:3000"  # NEW: Optional override
```

---

## Deployment Considerations

### Minikube Environment

Phase 1 works out of the box:
- Deploy multiple worker replicas
- Temporal handles session routing
- Existing `scripts/install-minikube.sh` needs replica count adjustment

```bash
# Update Helm values
helm upgrade rocketship ./charts/rocketship \
  --set worker.replicas=3 \
  --set worker.enableSessions=true
```

### Production Kubernetes

Phase 2 requires additional resources:

```yaml
# Browser pool deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: browser-pool
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: browserless
        image: browserless/chrome:latest
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
---
apiVersion: v1
kind: Service
metadata:
  name: browser-pool
spec:
  selector:
    app: browser-pool
  ports:
  - port: 3000
    targetPort: 3000
```

---

## Rollback Plan

If Phase 1 (session affinity) causes issues:

1. Revert worker registration changes
2. Document that browser tests require local-only mode
3. Add warning when running browser tests in cloud

If Phase 2 (remote browsers) causes issues:

1. Set `browser_mode: local` in config
2. Scale down browser pool service
3. Workers continue using session affinity

---

## Success Metrics

### Phase 1 Success Criteria

- ✅ Browser tests pass with 3+ worker replicas
- ✅ No "session not found" errors
- ✅ Worker failure doesn't hang workflows
- ✅ Session cleanup works (no zombie browsers)

### Phase 2 Success Criteria

- ✅ 10+ concurrent browser sessions
- ✅ Workers are stateless (no local browser processes)
- ✅ Browser pool scales independently
- ✅ Network failures handled gracefully
- ✅ CDP latency < 100ms p99

---

## Open Questions

1. **Session timeout handling:** What happens if a browser session outlives the workflow? Need cleanup job?
2. **Browser resource limits:** How many concurrent browsers per worker? Per pool?
3. **Browser version management:** How to update Chrome/Firefox without breaking tests?
4. **Headless vs headed:** Should remote browsers support headed mode for debugging?
5. **Screenshot storage:** Currently saved locally - need S3/GCS for remote browsers?

---

## References

- [Temporal Sessions Documentation](https://docs.temporal.io/dev-guide/go/features#sessions)
- [Browserless Documentation](https://docs.browserless.io/)
- [Playwright Docker Guide](https://playwright.dev/docs/docker)
- Chrome DevTools Protocol: https://chromedevtools.github.io/devtools-protocol/

---

## Next Steps

1. **Immediate:** Review this plan with team
2. **This week:** Implement Phase 1 (session affinity)
3. **Next sprint:** Deploy to staging with multiple workers
4. **Future:** Plan Phase 2 (remote browsers) based on usage patterns
