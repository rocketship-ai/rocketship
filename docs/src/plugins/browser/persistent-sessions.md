# Persistent Browser Sessions with Playwright + browser-use

Rocketship splits browser testing across two plugins that share a single Chromium session via Chrome DevTools Protocol (CDP):

- **`playwright`** launches Chromium, records its CDP websocket, and runs deterministic Python scripts.
- **`browser_use`** attaches to the same websocket and executes one concise, agentic task.

## Session Files

`playwright.start` spawns Chromium with `--remote-debugging-port=0`, then writes `${ROCKETSHIP_RUN_DIR:-.rocketship}/tmp/browser_sessions/<session_id>.json`:

```json
{
  "wsEndpoint": "ws://127.0.0.1:45373/devtools/browser/6ce7...",
  "pid": 53210,
  "createdAt": "2025-01-15T16:39:12Z"
}
```

The PID is the Chromium process. `playwright.stop` terminates it (SIGTERM on Unix, Kill on Windows) and removes the JSON file. A per-session profile directory is kept under `tmp/browser_sessions/<session_id>/profile` so cookies and storage survive across steps.

## Lifecycle

1. **Start** – `playwright.start` launches Chromium (headless by default), records `wsEndpoint`, and returns immediately.
2. **Deterministic scripting** – `playwright.script` (Python only) reconnects with `connect_over_cdp()` and runs the provided script. You can inject secrets via `config.env`.
3. **Agentic follow-up** – `browser_use` opens the same CDP endpoint, runs one short task, and reports extracted context or artifacts.
4. **Stop** – `playwright.stop` kills the stored PID and clears the session file to free the port/profile.

Saves and assertions (`json_path`) remain available on every step for variable passing and validation.

## Example Flow

`examples/browser/persistent-session/checkout.yaml` demonstrates an interleaved run:

```yaml
- name: start browser
  plugin: playwright
  config:
    role: start
    session_id: "checkout-{{ .run.id }}"
    headless: true

- name: visit landing page
  plugin: playwright
  config:
    role: script
    session_id: "checkout-{{ .run.id }}"
    language: python
    script: |
      page.goto("https://example.com")
      result = {"title": page.title()}

- name: summarize via agent
  plugin: browser_use
  config:
    session_id: "checkout-{{ .run.id }}"
    task: "Read the page currently open and summarize it in one sentence."
    allowed_domains:
      - example.com
    max_steps: 4

- name: stop browser
  plugin: playwright
  config:
    role: stop
    session_id: "checkout-{{ .run.id }}"
```

## Tips

- Keep `browser_use` prompts short; chain multiple steps for long flows so state persists cleanly.
- Ensure `python3`, `playwright`, and `browser-use` are installed (`pip install playwright browser-use` and `playwright install chromium`).
- If you encounter “session ... is not active”, confirm the session ID matches exactly and that `playwright.start` ran earlier in the same test.
- When debugging locally, set `ROCKETSHIP_LOG=DEBUG` to watch session creation, script execution, and teardown events.
