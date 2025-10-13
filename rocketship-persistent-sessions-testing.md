# Rocketship — Persistent Browser Session Testing (Follow-up Agent Brief)

You are taking over a branch that introduces two new browser plugins (`playwright` and `browser_use`) that share a single Chromium session via Playwright's BrowserServer CDP endpoint. Your job is to **validate the implementation end-to-end, harden it, and ship any fixes** required to make both plugins production-ready.

This brief assumes **zero prior context** and that you have unrestricted execution (no sandbox). Treat this as an investigative + stabilization mission.

---

## Current State (as of hand-off)

- **Go helpers** live in `internal/browser/sessionfile/` (JSON session metadata) and are already covered by unit tests.
- **Playwright plugin** is implemented under `internal/plugins/playwright/` with:
  - `playwright.go` (activity entrypoint)
  - `helpers.go` (templating/saves/assertions helpers)
  - `process_*.go` (per-OS process control)
  - `playwright_runner.py` (embedded Python CLI invoked by Go)
  - `persistent_session_test.go` (stubbed smoke test that calls into `browser_use`)
- **browser_use plugin** lives in `internal/plugins/browser_use/` with matching helpers + its own embedded `browser_use_runner.py`.
- **Docs & example**
  - `docs/src/plugins/browser/persistent-sessions.md`
  - `docs/src/examples/ai/browser-testing.md` (callout pointing to new flow)
  - `examples/browser/persistent-session/checkout.yaml` (interleaved scenario)
- No other plugins depend on this work yet; the legacy `browser` plugin still exists but is now documented as the old mode.

Known gaps: the branch was not tested with a real Playwright/browser-use runtime because the previous agent lacked Go 1.24.7 locally. All tooling now compiles, but runtime regressions may surface when you execute the binaries.

---

## Environment Expectations

Please prepare the following before running tests:

1. **Go 1.24.7+** installed and active (`go version`).
2. **Python 3.11+** with `pip`.
3. Python packages:
   ```bash
   pip install --upgrade playwright browser-use langchain-openai langchain-anthropic
   playwright install --with-deps chromium
   ```
4. Any LLM API keys required by `browser-use` (OpenAI/Anthropic) if you plan to execute agentic tasks against real sites.
5. Optional: set `ROCKETSHIP_LOG=DEBUG` when running CLI commands to capture verbose output.

---

## Validation Plan

Work through the checkpoints in order. Fix issues as they appear; update docs/example/tests if behavior changes.

### 1. Static + Unit Tests

- Run `go fmt ./...` (should be clean).
- Run `go test ./...` – ensure:
  - `internal/browser/sessionfile` tests succeed on real filesystem.
  - `internal/plugins/playwright/persistent_session_test.go` passes (it uses stubbed Python scripts; no real browser needed).
- Add additional unit tests if you find unhandled edge cases (e.g., malformed session file errors, env templating mistakes, etc.).

### 2. Real Playwright Session Smoke Test

Using the CLI binary (after `make install`):

1. Set `ROCKETSHIP_LOG=DEBUG`.
2. Run the example suite:
   ```bash
   rocketship run -af examples/browser/persistent-session/checkout.yaml \
     --env PW="correct-horse-battery-staple"
   ```
   - Confirm `playwright.start` creates `.rocketship/tmp/browser_sessions/<session>.json` with live `wsEndpoint`.
   - Ensure `playwright.script` reuses the same CDP session (cookies retained, no new browser process).
   - Validate the `browser_use` step completes one natural-language task without launching a second browser.
   - Confirm `playwright.stop` removes the session file and kills the BrowserServer process.
3. Capture artifacts (logs, screenshots if produced) for later troubleshooting.

### 3. Failure Handling

Intentionally break scenarios to confirm friendly errors:

- Run a `browser_use` step **before** `playwright.start` → expect a “session not active” error.
- Delete/modify the session JSON mid-run → ensure plugins report useful messages (no panics).
- Kill the BrowserServer manually during a run to see how reconnect logic behaves; add retries or clearer errors if needed.

### 4. Platform Checks

- Validate both plugins on macOS/Linux (Windows support is stubbed; smoke-test if you have access).
- Confirm file permissions (`0o600`/`0o755`) work on the target OS.

### 5. Documentation & Examples

- Walk through `docs/src/plugins/browser/persistent-sessions.md` and ensure the steps match the actual CLI behavior.
- Update docs if you tweak flags, defaults, or outputs.
- Expand `examples/browser/persistent-session/checkout.yaml` with assertions/saves if that helps detect regressions.

### 6. Optional Enhancements

If time permits:

- Add integration coverage that uses real Playwright (behind a build tag) to run in CI when chromium dependencies are present.
- Implement graceful shutdown during Ctrl+C (playwright stop triggered automatically).
- Improve `browser_use_runner.py` serialization logic to surface richer results.

---

## Success Criteria

You are done when:

- `go test ./...` is green on a Go 1.24.7 environment.
- The `checkout.yaml` example executes successfully end-to-end on a real browser, with a single persistent session across deterministic and agentic steps.
- Documentation accurately reflects the current behavior and any caveats.
- All session files and background processes are cleaned up consistently, even on error paths.

Document any outstanding risks or TODOs in a follow-up note before handing the branch back.

Good luck, and thank you for hardening the new persistent browser workflow! :)
