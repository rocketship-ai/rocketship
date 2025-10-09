# System Limits

## Suite Cleanup Timeout

Suite cleanup workflows have a **45-minute timeout**. This limit applies everywhere:

- **Auto mode** (`rocketship run -af`)
- **Manual mode** (`rocketship start server` / `rocketship stop server`)
- **Rocketship Cloud** (persistent servers)

If cleanup exceeds 45 minutes, Temporal cancels the workflow. The run outcome (passed/failed) is NOT changed - cleanup failures are logged but don't fail the suite.

### Why This Matters

Cleanup can include long-running operations:

- `delay` steps: `{ duration: "10m" }`
- Environment teardown scripts: `./env/down.sh` (30+ minutes)
- Cloud resource deletion: RDS, CloudFormation, Kubernetes clusters

### Best Practices

**Prioritize critical cleanup first:**
```yaml
cleanup:
  always:
    - name: "Delete test database"  # Critical - do first
      plugin: supabase
      config: { action: execute_sql, query: "DROP DATABASE test_db" }
    - name: "Delete logs"  # Nice to have - do last
      plugin: http
      config: { method: DELETE, url: "{{ logs_url }}" }
```

**Use idempotent operations:**
Design cleanup to be safely re-runnable if it times out or fails partway through.

**Break up long operations:**
Instead of one 30-minute script, use multiple smaller steps that can fail independently:
```yaml
cleanup:
  always:
    - name: "Stop services"
      plugin: script
      config: { cmd: "./scripts/stop-services.sh" }
    - name: "Delete volumes"
      plugin: script
      config: { cmd: "./scripts/delete-volumes.sh" }
    - name: "Remove network"
      plugin: script
      config: { cmd: "./scripts/remove-network.sh" }
```

**Log progress:**
Add log steps between long operations so you can see how far cleanup progressed:
```yaml
cleanup:
  always:
    - name: "Starting teardown"
      plugin: log
      config: { message: "Tearing down infrastructure..." }
    - name: "Delete stack"
      plugin: script
      config: { cmd: "./env/down.sh" }
    - name: "Teardown complete"
      plugin: log
      config: { message: "Infrastructure torn down" }
```
