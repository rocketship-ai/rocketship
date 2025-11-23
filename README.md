<p align="center">
  <img src="docs/src/assets/transparent.png#gh-light-mode-only" alt="Rocketship black logo" width="210" style="display: block; margin: 0 auto; padding: 20px;">
  <img src="docs/src/assets/transparent-reverse.png#gh-dark-mode-only" alt="Rocketship white logo" width="210" style="display: block; margin: 0 auto; padding: 20px;">
</p>
<h3 align="center">A testing framework for your coding agent.</h3>
<p align="center">Let your coding agent write and run E2E tests for your web app.</p>

<p align="center">
  <a href="https://github.com/rocketship-ai/rocketship/releases"><img src="https://img.shields.io/github/v/release/rocketship-ai/rocketship.svg" alt="Github release"></a>
  <a href="https://goreportcard.com/report/github.com/rocketship-ai/rocketship"><img src="https://goreportcard.com/badge/github.com/rocketship-ai/rocketship" alt="Go Report Card"></a>
  <br>
</p>
<p align="center">
    <a href="https://github.com/rocketship-ai/rocketship/releases">Download</a> ·
    <a href="https://docs.rocketship.sh">Documentation</a>
</p>

![rocketship demo gif](docs/src/assets/demo2.gif)

🚀 Rocketship is an open‑source testing framework that your coding agent can use to QA test customer journeys by using community-owned plugins like [Supabase](https://docs.rocketship.sh/plugins/supabase/), [Playwright](https://docs.rocketship.sh/plugins/playwright/), [Agent](https://docs.rocketship.sh/plugins/agent/), etc. It gives your coding agent a test harness so it can ship changes without silently breaking critical user flows like logins, signups, checkouts, you name it. **Here's how it works:**

1. You install the Rocketship CLI and add a `.rocketship` directory to your repository. Any `.yaml` files in this directory will be picked up and run by Rocketship.
2. Your coding agent builds out a new feature, customer journey, or other user-driven scenario and writes a Rocketship test that asserts the flow works end-to-end.
3. You run this test locally (and in CI) before merging. Once it’s checked in, it guards that flow against regressions every time your agent edits the code.

## Core Features

- **Rocketship CLI** Run the engine locally or connect to a remote address.
- **Declarative YAML** Define your test scenarios as declarative YAML specs.
- **Built-in Features** Variable passing, retryability, lifecycle hooks, and more.
- **Plugin Ecosystem** Add the APIs and protocols that matter to you.
- **Deploy-Ready Images** Need to save history or run tests on a schedule? Host Rocketship on your own infra.

## Agent Quickstart

Paste the [ROCKETSHIP_QUICKSTART.md](https://raw.githubusercontent.com/rocketship-ai/rocketship/main/ROCKETSHIP_QUICKSTART.md) file into your coding agent's context window, so that it understands how to build and run tests.

Once it has that context, you can:

- Ask it to propose `.rocketship/*.yaml` tests for your critical flows (login, signup, checkout, etc.).
- Have it update the matching Rocketship test whenever it edits those flows, and run `rocketship run -ad .rocketship` before committing or opening a PR.

## Human Quickstart

### Prerequisites

```bash
# the testing orchestration engine
brew install temporal
# required if you want browser testing
pip install playwright
playwright install chromium
# required if you use the agent plugin
pip install claude-agent-sdk
export ANTHROPIC_API_KEY=your-key
```

### Install

**Mac users:**

```bash
brew tap rocketship-ai/tap
brew install rocketship
```

**Linux bros:**

```bash
curl -fsSL https://raw.githubusercontent.com/rocketship-ai/rocketship/main/scripts/install.sh | bash
```

### Save a test spec

```bash
mkdir -p .rocketship
```

```yaml
# .rocketship/auth_flows.yaml
name: "Auth Flows"
tests:
  - name: "Existing user can log in"
    steps:
      - name: "Create user in Supabase"
        plugin: supabase
        config:
          url: "{{ .env.SUPABASE_URL }}"
          key: "{{ .env.SUPABASE_KEY }}"
          operation: "auth_sign_up"
          auth:
            email: "test-{{ .run.id }}@example.com"
            password: "password123"
        save:
          - json_path: ".user.email"
            as: "login_email"
          - json_path: ".user.id"
            as: "supabase_user_id"

      - name: "Log in via browser"
        plugin: playwright
        config:
          role: script
          script: |
            from playwright.sync_api import expect

            page.goto("{{ .env.FRONTEND_URL }}/login")
            page.locator("input[type='email']").fill("{{ login_email }}")
            page.locator("input[type='password']").fill("password123")
            page.locator("button[type='submit']").click()

            expect(page).to_have_url("{{ .env.FRONTEND_URL }}/dashboard")

      - name: "AI checks dashboard"
        plugin: agent
        config:
          prompt: |
            In the current browser session, verify:
            - The dashboard loaded for the logged-in user
            - The page greets the user with their email: "Hello {{ login_email }}" (or similar)
            - A "New Project" or similar CTA is visible
          capabilities: ["browser"]
```

### Run it

```bash
rocketship run -ad .rocketship # starts the local engine, runs the tests, shuts the engine down
```

## Documentation

[https://docs.rocketship.sh](https://docs.rocketship.sh)

## Contribute!!!

I would love to build this with you! Reach out to me on [LinkedIn](https://www.linkedin.com/in/magiusdarrigo) and let's chat. A great first contribution is building a [plugin](https://github.com/rocketship-ai/rocketship/tree/main/internal/plugins) for your favorite API. If you want to contribute to Rocketship, start by reading [Contributing to Rocketship](https://docs.rocketship.sh/contributing).

## License

Rocketship is distributed under the [MIT license](https://github.com/rocketship-ai/rocketship/blob/main/LICENSE).
