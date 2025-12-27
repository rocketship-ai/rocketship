# Rocketship Cloud v1 — Design (Updated)

Here’s a tightened v1:

- No PR approvals/merges inside Rocketship. All done using GitHub.
- Git-as-SoT but the Web UI / CLI can edit & run tests immediately from a working copy; Rocketship can optionally open a PR or commit to a branch (if the user has Git perms and wants to).
- No workspaces. Just Org → Project.
- Roles reduced to Read / Write / None at Project scope.
- Project scoping by repo path (mono-repo friendly).
- CI tokens: scoped permissions + TTL (or never-expire) + manual revoke.

Below is the full v1 design—DB, JWTs, enforcement, flows, and guardrails.

## 1) Tenancy & hierarchy (v1)

**Org → Project.** That’s it.

- **Org** = billing + membership + global admin(s).
- **Project** = {repo URL, default branch, `path_scope` globs, discovered environments, (future) registered workers}.

**Enterprise dedicated cluster** maps 1:1 to one or more Orgs. No workspace layer needed.

**Core tables (minimal):**

```
orgs(id, name, created_at, plan)
users(id, email, github_user_id, created_at)
org_admins(org_id, user_id)                        -- org admins have Write on every project
projects(id, org_id, name, repo_url, default_branch, path_scope, created_at)
project_members(project_id, user_id, role)         -- role ∈ {read, write}
ci_tokens(id, project_id, name, scopes_json, never_expires, expires_at, revoked_at, created_at, hashed_secret)
environments(id, project_id, name, created_at)     -- lightweight index of discovered env ids (no policy)
workers(id, project_id, env_id, status, last_heartbeat_at, capabilities_json)  -- v2 placeholder
```

- `path_scope`: directory or glob patterns (e.g., `frontend/.rocketship/**`). Multiple → JSON array.

## 2) Identity & membership

- **Org Admins:** create projects, assign/remove project members, manage billing, rotate secrets, delete projects; promote/demote admins. Implicit **Write** to all projects.
- **Project Members**
  - **Write:** run tests in any env, edit tests in UI and run edited tests, edit schedules, ask Rocketship to create PR or commit (if Git allows), mint tokens; includes Read.
  - **Read:** view runs, artifacts, logs, configs at ref; cannot run/modify; cannot view tokens.

If a user has no project role, they see nothing in that project (optionally just minimal metadata to request access).

## 3) Permissions model (simple, explicit)

**Actions → required role**
| Action | Scope | Required |
|---|---|---|
| View runs/artifacts/config | project | Read |
| Run test/suite (any env) | project | Write |
| Edit schedules | project | Write |
| Edit config in UI & run uncommitted | project | Write |
| Ask Rocketship to open PR / commit | project | Write (+ Git provider must allow) |
| Register worker | project | Write _(v2 feature)_ |
| Add/remove project members | project | Org Admin |
| Create/delete project | org | Org Admin |
| Billing | org | Org Admin |
| Token Mgmt | project | Write |

## 4) Git-as-SoT with UI edits (no approvals in-app)

**Read path**

- UI renders from **repo@commit** cache, shows SHA; banner if default branch advanced.

**Write path (UI)**

- Write user edits via forms → **Run** executes immediately as **uncommitted** (detached), stamped with bundle hash.
- **Commit** (if Git says user can push) or **Open PR** (approval/merge in GitHub).

**CLI**

- CLI can edit & run tests immediately from a working copy; If the user wants to make a git commit, they can do so using their existing git client.

**CI & default-branch**

- PRs/pushes: webhooks refresh cache and can trigger checks (schema validate, preview); Rocketship never gates merge.

**Mono-repo scopes**

- Editor/runner read/write **only** within `path_scope`. Webhooks react only to scoped changes. Enforced client + server.

## 5) Token & auth architecture (v1)

**Users (CLI/Web)**

- Access JWT (RS256) + opaque refresh. Claims (trimmed):

```
sub:"user:<uuid>"
org_admin_of:[...]
proj_roles:[{project_id:"p1",role:"write"},{project_id:"p2",role:"read"}]
gh:{id,login}
```

**CI Tokens (non-interactive)**

- Opaque secret (shown once), hashed at rest. No refresh. TTL or never-expire; revocable. Project-scoped.
- **Scopes:** e.g., `["read","write"]`. **No environment embedded.**

**Workers**

- mTLS + short-lived work-permit JWT (project/env) **reserved for v2**. **[MY OPINION]:** don’t add worker-specific features in v1.

**Run labels (stamped on every run)**

```
environment: <id>                 # e.g., local/dev/stage/prod
initiator: user | ci | agent | schedule
config_source: uncommitted | repo_commit
commit_sha: <sha>                 # when repo_commit
bundle_sha: sha256:...            # when uncommitted (with manifest)
submitted_by: <user_id|token_id>
```

**Environment selection (precedence)**

1. `--env <id>` flag
2. First-file shebang/header in any selected Rocketship YAML / variables YAML / env file: `# rs env=<id>`
3. Default: `local`

- Conflicting headers across selected files → error (require `--env`).

**config_source detection**

- If any change under `path_scope` vs `HEAD` (modified/staged/untracked) → `uncommitted`; else `repo_commit` (pin `HEAD` SHA).

## 6) Enforcement points (what actually checks what)

**Engine interceptors**

1. Verify JWT/token; resolve actor → set `initiator` = `user|ci|agent`.
2. Resolve resource scope (org/project, env via selection rules).
3. Project membership check: Write/Read.
4. Action policy:
   - **Run:** must be **Write**; if `config_source=uncommitted`, enforce `path_scope` constraints and bundle limits. _(No env guardrails in v1.)_
   - **Commit/OpenPR:** must be **Write**; verify Git push rights; else only Open PR.
   - **RegisterWorker:** Write _(v2; ignore in v1)_.
   - **List/View:** Read.
5. Attach context `{actor, project_id, environment, initiator, config_source, commit_sha|bundle_sha}` and start Temporal workflow.

**Ephemeral Runs (unregistered dirs)**

- Allowed: CLI uploads bundle; Engine runs just as a **scratch project** in user’s org with just shorter retention. UI banner: “Ephemeral run — Register Project to persist”.
- Maybe introduce and enforce --scratch for users who might not be allowed to trigger a run for a project in the CLI, but want to run the test in the cloud.
- In the future, I assume we will have a web ui sidebar tab for these kinds of runs. But idk.

## 7) Guardrails (v1, user-friendly)

- **Uncommitted clarity:** Web UI: badge + bundle hash + “Download snapshot”; promote to PR in one click. CLI: just use your git client.
- **Path-scope enforcement:** editor and server refuse out-of-scope mutations; webhooks ignore out-of-scope changes.
- **Rate/Quota:** soft warnings, clear hard caps; per-project defaults.
- **Shebang parsing:** only first non-empty comment line; conflicts error.

## 8) End-to-end flows (concrete)

**A) Web UI – edit & run**

1. Write user edits → **Run** → `config_source=uncommitted` (bundle hash).
2. Optional **Open PR** / **Commit** (if allowed).

**B) Web UI – open PR**

1. Write user → Open PR.
2. PR checks (optional) run in CI.
3. After merge, cache updates; future runs use merged SHA.

**C) CLI – zero-flags run (registered project)**

```
# auto project (remote + path_scope); env by flag/header/default; config_source auto
rocketship run
```

- Engine verifies Write; pins `repo_commit` or accepts `uncommitted` bundle.

**D) CLI – unregistered project**

```
rocketship run
```

- Runs as **Ephemeral Run** in cloud (short retention).

**E) CI run**

- CI calls `POST /runs` with token.
- Engine sets `initiator=ci`, `config_source=repo_commit` (ref→SHA).

**F) Agent-initiated**

- Auth as agent path or `--initiator agent`; `initiator=agent`; rest identical to user runs.

**G) Re-run any prior run**

- Write user can re-run any run, any env.
- Keep environment unless user sets `--env`.
- Keep `repo_commit` (same SHA) or `uncommitted` (same snapshot); flips to `uncommitted` only if user edits before re-run.

**H) Scheduled (smoke/cadence)**

- `initiator=schedule`.
- By default, schedule resolves **project default branch HEAD** at fire time → `config_source=repo_commit`.
- Ad-hoc modified runs are allowed any time, but schedules keep tracking HEAD until changes are merged.
