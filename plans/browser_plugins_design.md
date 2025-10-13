# Rocketship — Persistent Browser Sessions via Playwright CDP (Coding Agent Brief)

> Objective: Split Rocketship’s browser testing into **two plugins** that can share a **single persistent browser session** using **Playwright’s BrowserServer wsEndpoint (CDP)**. No backward compatibility required.

- **Plugins**
  - **`playwright`**: owns the browser lifecycle and deterministic code steps.
  - **`browser_use`**: agentic steps powered by `browser-use`, attaching to the same browser via CDP.
- **Persistence**: The **first `playwright.start` step** launches a BrowserServer and writes a **session file** containing the `wsEndpoint` (and PID). Subsequent steps reattach using that file; **`playwright.stop`** terminates.
- **Docs to read first (required)**
  - **browser-use – Follow-up tasks**: keep session alive and chain tasks. https://docs.browser-use.com/examples/templates/follow-up-tasks
  - **browser-use – Playwright integration**: share Chrome via CDP between Playwright and browser-use. https://docs.browser-use.com/examples/templates/playwright-integration
  - **browser-use – Attach a CDP URL**: https://docs.browser-use.com/customize/browser/remote
  - **Playwright – connectOverCDP**: attach to existing browser (Chromium). https://playwright.dev/docs/api/class-browsertype
  - **Playwright – BrowserServer.wsEndpoint**: websocket URL from `launchServer()`. https://playwright.dev/docs/api/class-browserserver

---

## Deliverables (Definition of Done)

1. **New plugin: `internal/plugins/playwright`**
   - Supports roles: `start | script | stop`.
   - **start**: launch BrowserServer (Chromium) → write session file `{ wsEndpoint, pid, createdAt }`.
   - **script**: reattach via `connect_over_cdp(wsEndpoint)` and run user script (Python; v1 only).
   - **stop**: read session file, kill PID, and remove file.
2. **Renamed/update plugin: `internal/plugins/browser_use`**
   - Single role `task`: reattach via CDP (`cdp_url = wsEndpoint`) and run one NL task using `browser-use`.
   - Supports follow-up semantics by executing **exactly one task per Rocketship step** (short prompts).
3. **Shared session file layer**
   - Location: `${RUN_DIR:-.rocketship}/tmp/browser_sessions/<session_id>.json`.
   - JSON schema: `{ "wsEndpoint": "...", "pid": 12345, "createdAt": "RFC3339" }`.
   - Helper lib in Go to read/write/validate this file.
4. **Examples + docs**
   - `examples/browser/persistent-session/checkout.yaml` demonstrating interleaved steps.
   - `docs/plugins/browser/persistent-sessions.md` explaining lifecycle and CDP attach.
5. **Basic tests**
   - Unit tests for session file helpers.
   - Smoke test that logs in once (playwright) then performs actions (browser_use) without re-login.

---

## Plugin Contracts (YAML)

### `playwright` plugin

```yaml
# Step A: launch a persistent browser
plugin: playwright
config:
  role: "start"
  session_id: "checkout-{{ .run.id }}"
  headless: true
  slow_mo_ms: 0
  # Optional: custom args if needed
  launch_args: ["--disable-gpu"]

# Step B: deterministic script (Python)
plugin: playwright
config:
  role: "script"
  session_id: "checkout-{{ .run.id }}"
  language: "python"  # v1: python only
  script: |
    # Uses existing session via CDP
    page.goto("https://shop.example.com/login")
    page.get_by_label("Email").fill("qa@example.com")
    page.get_by_label("Password").fill(env["PW"])
    page.get_by_role("button", name="Sign in").click()
    page.wait_for_url("**/products")

# Step Z: shutdown
plugin: playwright
config:
  role: "stop"
  session_id: "checkout-{{ .run.id }}"
```

### `browser_use` plugin

```yaml
# Single task per step; attaches to same CDP browser
plugin: browser_use
config:
  session_id: "checkout-{{ .run.id }}"
  task: "Open the first product and add to cart"
  allowed_domains: ["shop.example.com"]
  max_steps: 10
  use_vision: false
```

---

## Flow Example (Interleaved)

```yaml
name: "Checkout persistent-session demo"
tests:
  - name: "happy path"
    steps:
      - name: "start browser"
        plugin: playwright
        config:
          {
            role: "start",
            session_id: "checkout-{{ .run.id }}",
            headless: true,
          }

      - name: "login (deterministic)"
        plugin: playwright
        config:
          role: "script"
          session_id: "checkout-{{ .run.id }}"
          language: "python"
          script: |
            page.goto("https://shop.example.com/login")
            page.get_by_label("Email").fill("qa@example.com")
            page.get_by_label("Password").fill("correct-horse-battery-staple")
            page.get_by_role("button", name="Sign in").click()
            page.wait_for_url("**/products")

      - name: "browse & add via agent"
        plugin: browser_use
        config:
          session_id: "checkout-{{ .run.id }}"
          task: "Navigate to the first product and click 'Add to cart'"

      - name: "assert cart (deterministic)"
        plugin: playwright
        config:
          role: "script"
          session_id: "checkout-{{ .run.id }}"
          language: "python"
          script: |
            page.goto("https://shop.example.com/cart")
            assert page.get_by_text("1 item").is_visible()

      - name: "stop browser"
        plugin: playwright
        config: { role: "stop", session_id: "checkout-{{ .run.id }}" }
```

---

## Implementation Plan

### 1) Session File Helpers (Go)

Create `internal/browser/sessionfile/sessionfile.go`:

- `func Path(runDir, sessionID string) string`
- `func Read(ctx, sessionID) (wsEndpoint string, pid int, err error)`
- `func Write(ctx, sessionID, wsEndpoint string, pid int) error`
- `func Remove(ctx, sessionID) error`
- `func EnsureDir() error`

`runDir` resolution: `os.Getenv("ROCKETSHIP_RUN_DIR")` else `.rocketship` at repo root. Ensure `.../tmp/browser_sessions/` exists.

### 2) Playwright Plugin (Go + Python)

- New Go plugin at `internal/plugins/playwright/` with an activity that spawns a Python entrypoint:
  - **start**: run `python -m internal.plugins.playwright.start --session <id> --headless ...`
    - Python:
      - `from playwright.sync_api import sync_playwright`
      - `browser_server = playwright.chromium.launch_server(headless=headless, args=launch_args)`
      - `ws = browser_server.ws_endpoint`
      - write session file `{ws, pid=os.getpid()}`
  - **script**: run `python -m internal.plugins.playwright.script --session <id> --file <tmp_script.py>`
    - Python:
      - `browser = chromium.connect_over_cdp(ws)`
      - `context = browser.contexts[0] or browser.new_context()`
      - `page = context.pages[0] or context.new_page()`
      - execute provided **Python** script with `page` and `env` in scope; capture exceptions → stderr/json.
  - **stop**: read session, send SIGTERM to PID (best-effort), then remove session file.
- Make headless default true. Allow `slow_mo_ms` and basic `launch_args` passthrough.

### 3) browser_use Plugin (Go + Python)

- Rename folder to `internal/plugins/browser_use/`; update references.
- Activity spawns python `internal/plugins/browser_use/task.py`:
  - Read session file → `cdp_url=wsEndpoint`.
  - `from browser_use import Browser, Agent`
  - `browser = Browser(cdp_url=cdp_url, keep_alive=True)`
  - `await browser.start()`
  - `agent = Agent(task=task, browser_session=browser, allowed_domains=allowed_domains, ...)`
  - `await agent.run()`
  - Return URL/title/screenshot if available.
- Enforce **one** NL task per step; keep prompts short. Let retries reattach to same session.

### 4) Python Packaging

- Reuse existing Python executor pattern.
- Add `requirements.txt` for new plugin: `playwright`, `browser-use`, `python-dotenv` (optional).
- On CI, run `playwright install --with-deps chromium` before tests.

### 5) Timeouts & Retries

- Reattachment is idempotent; if session file not found → return actionable error (`start must run first`).

### 6) Docs & Examples

- `docs/plugins/browser/persistent-sessions.md`: lifecycle diagram; mention CDP, wsEndpoint, session file.
- `examples/browser/persistent-session/checkout.yaml` as above.

---

## Acceptance Checklist

- [ ] `playwright.start` creates session file; re-running **script** attaches without new login.
- [ ] `browser_use.task` attaches via CDP and can proceed from state left by Playwright.
- [ ] Interleaving steps preserve cookies/localStorage and active tab.
- [ ] `playwright.stop` kills the BrowserServer and cleans up files.
- [ ] All configs validated; clear errors if session missing/mismatched.
- [ ] Short prompts per `browser_use` step; long flows work when split into multiple steps.

---

## Notes

- Keep the **session file format stable**. In future cloud runs, swap file for a registry (e.g., Redis) without changing plugin contracts.
- Stick to **Python scripts** for Playwright in v1 (smaller surface); TS support can come later.

## IMPORTANT!!!

Today there already is the `browser` plugin that uses browser-use. You should read and understand how that plugin works and how it launches a browser and uses the browser-use agent. We want to deprecate this in favor of the new `playwright` and `browser_use` plugins. Please copy whatever is needed from the `browser` plugin to the new `playwright` / `browser_use` plugins. There should not be much code duplication. If there ends up being a lot, you should tell me and we can discuss how to best proceed. Like potentially merging into 1 plugin.

# End of brief
