# Persistent Session Validation Plan (for Non-Sandbox Agent)

## Why These Changes Landed
- Reverted the `playwright` plugin back to a **Python-first workflow** so that `playwright.script` steps execute user provided Python (per product brief).
- Replaced the Node-based BrowserServer launcher with a Python runner that **spawns Chromium directly with a remote-debugging port**. The runner now emits the browser PID so `playwright.stop` can terminate the real process.
- Session files remain at `${RUN_DIR:-.rocketship}/tmp/browser_sessions/<session_id>.json`, storing `{ wsEndpoint, pid, createdAt }`.
- The same CDP endpoint is consumed by the refreshed `browser_use` plugin, keeping one Chromium instance across deterministic + agentic steps.
- Updated unit scaffolding (`persistent_session_test.go`) now stubs the Python runners instead of Node.

## Local Setup Expectations
1. Install Python dependencies if you have not already:
   ```bash
   pip install playwright browser-use
   playwright install chromium
   ```
2. Ensure `python3` and `playwright` CLI are on `PATH`. The plugin shells out to `python3`.
3. Clean any stale session files:
   ```bash
   rm -rf .rocketship/tmp/browser_sessions
   ```

## Validation Checklist
1. **Go tests** (requires Go ≥ 1.24):
   ```bash
   go test ./internal/browser/sessionfile ./internal/plugins/playwright ./internal/plugins/browser_use
   ```
2. **Manual smoke** using a persistent session example (once authored):
   ```bash
   ROCKETSHIP_LOG=DEBUG rocketship run -af examples/browser/persistent-session/checkout.yaml
   ```
   - Verify the `playwright.start` step produces a `tmp/browser_sessions/<id>.json` file with a live PID.
   - Confirm subsequent `playwright.script` and `browser_use.task` steps reuse the same authenticated state.
3. **Stop semantics**:
   - After `playwright.stop`, ensure the session JSON is removed and the Chromium PID is gone.
   - Confirm a second `playwright.start` with the same `session_id` succeeds once the previous session is stopped.

## Outstanding Work / Questions
- End-to-end coverage still needs a real example in `examples/browser/persistent-session/` plus integration documentation in `docs/plugins/browser/`.
- Confirm whether we want to persist user-data directories between runs (currently scoped to the session lifetime under `tmp/browser_sessions/<session_id>/profile`).
- Validate on Windows: the Python runner uses `start_new_session=True`; confirm `terminateProcessTree` (Win impl) is sufficient.

## Next Agent Actions
1. Run the validation checklist above on an unsandboxed machine.
2. File follow-up PRs for:
   - Example YAML + docs.
   - Additional integration tests if feasible (potentially behind a build tag that requires Playwright to be installed).
3. Report any platform-specific adjustments (especially env detection for `python3` vs `py`) so we can generalize the runner.
