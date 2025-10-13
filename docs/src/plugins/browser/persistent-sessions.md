# Persistent Browser Sessions with Playwright + browser-use

Rocketship now splits browser testing across two plugins that share a single Chromium session via Playwright's CDP endpoint:

- **`playwright`** owns the browser lifecycle and deterministic scripting.
- **`browser_use`** executes short, agentic follow-up tasks inside the same session.

The first `playwright.start` step launches a Playwright `BrowserServer`, records its websocket endpoint in a session file, and keeps the process running. Subsequent `playwright.script` and `browser_use` steps attach to the saved endpoint. When the workflow finishes, `playwright.stop` terminates the browser server and removes the session file.

## Session Files

Session metadata lives under `${ROCKETSHIP_RUN_DIR:-.rocketship}/tmp/browser_sessions/<session_id>.json` with the following schema:

```json
{
  "wsEndpoint": "ws://127.0.0.1:45373/devtools/browser/6ce7...",
  "pid": 53210,
  "createdAt": "2024-04-04T16:39:12Z"
}
```

The PID is the long-running Python process that hosts the Playwright `BrowserServer`. `playwright.stop` sends it a graceful SIGTERM (or equivalent on Windows) and removes the JSON file.

## Lifecycle at a Glance

1. **Start** – `playwright.start` launches Chromium via `launch_server()`, writes the session file, and returns immediately.
2. **Deterministic steps** – `playwright.script` (Python only in v1) reconnects with `connect_over_cdp()` and runs your scripted logic.
3. **Agentic task** – `browser_use` attaches to the same CDP URL, executes one natural-language task, and returns a concise result.
4. **Stop** – `playwright.stop` kills the BrowserServer process and cleans up the session file.

All steps support Rocketship saves and assertions via `json_path`, making it easy to persist state or validate agent output.

## Example

`examples/browser/persistent-session/checkout.yaml` demonstrates an interleaved flow:

```yaml
- name: start browser
  plugin: playwright
  config:
    role: start
    session_id: "checkout-{{ .run.id }}"
    headless: true

- name: login (deterministic)
  plugin: playwright
  config:
    role: script
    session_id: "checkout-{{ .run.id }}"
    language: python
    script: |
      page.goto("https://shop.example.com/login")
      page.get_by_label("Email").fill("qa@example.com")
      page.get_by_label("Password").fill("correct-horse-battery-staple")
      page.get_by_role("button", name="Sign in").click()
      page.wait_for_url("**/products")

- name: browse & add via agent
  plugin: browser_use
  config:
    session_id: "checkout-{{ .run.id }}"
    task: "Navigate to the first product and click 'Add to cart'"
    allowed_domains:
      - shop.example.com
    max_steps: 10

- name: stop browser
  plugin: playwright
  config:
    role: stop
    session_id: "checkout-{{ .run.id }}"
```

## Tips

- Use short prompts and split multi-stage browsing into multiple `browser_use` steps. Each step reuses the same cookies, storage, and tabs left by Playwright.
- `playwright.script` currently supports Python only. You can pass `config.env` to inject secrets for deterministic scripts.
- When running locally, ensure `python3`, `playwright`, and `browser-use` are installed (`pip install playwright browser-use` and `playwright install chromium`).
- If you see `session ... is not active`, confirm that a matching `playwright.start` step ran earlier in the test and that the session IDs match exactly.

With these plugins you can handle precise login flows with Playwright and delegate exploratory actions to `browser-use`, all while keeping a single persistent browser alive for the entire test.
