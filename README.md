<p align="center">
  <img src="docs/src/assets/transparent.png#gh-light-mode-only" alt="Rocketship black logo" width="210" style="display: block; margin: 0 auto; padding: 20px;">
  <img src="docs/src/assets/transparent-reverse.png#gh-dark-mode-only" alt="Rocketship white logo" width="210" style="display: block; margin: 0 auto; padding: 20px;">
</p>
<h3 align="center">A testing framework for your coding agent.</h3>
<p align="center">Let your coding agent write and run e2e tests for your customer journeys.</p>

<p align="center">
  <a href="https://github.com/rocketship-ai/rocketship/releases"><img src="https://img.shields.io/github/v/release/rocketship-ai/rocketship.svg" alt="Github release"></a>
  <a href="https://goreportcard.com/report/github.com/rocketship-ai/rocketship"><img src="https://goreportcard.com/badge/github.com/rocketship-ai/rocketship" alt="Go Report Card"></a>
  <br>
</p>
<p align="center">
    <a href="https://github.com/rocketship-ai/rocketship/releases">Download</a> ·
    <a href="https://docs.rocketship.sh">Documentation</a>
</p>

**add gif here**<br>

🚀 Rocketship is an open‑source testing framework that your coding agent can use to QA test and verify complex, user-driven scenarios by using community-owned plugins like [Supabase](https://docs.rocketship.sh/plugins/supabase/), [Playwright](https://docs.rocketship.sh/plugins/playwright/), [Agent](https://docs.rocketship.sh/plugins/agent/), and others. **Here's how it works:**

1. You install the Rocketship CLI and add a `.rocketship` directory to your repository. Any `.yaml` files in this directory will be picked up and run by Rocketship.
2. Your coding agent builds out a new feature, customer journey, or other user-driven scenario. It creates a new rocketship test validating that the scenario works as expected. Iterating on code until the test case passes.
3. You check-in this new test alongside your other rocketship tests. Ensuring your coding agent never causes a code regression and breaks a critical flow in your app again.

## Under The Hood

- **Rocketship CLI** Run the engine locally or connect to a remote address.
- **Declarative YAML** Define your test scenarios as declarative YAML specs.
- **Built-in Features** Variable passing, retryability, lifecycle hooks, and more.
- **Plugin Ecosystem** Add the APIs and protocols that matter to you.
- **Deploy-Ready Images** Need to save history or run tests on a schedule? Host Rocketship on your own infra.

## Getting Started

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
mkdir -p .rocketship/auth_flows.yaml
```

```yaml
name: "Auth Flows"
tests:
  - name: "Existing User Login"
    steps:
      - name: "Create a new user via API"
        plugin: http
        config:
          method: POST
          url: "{{ .vars.base_url }}/users"
          body: |
            {
              "name": "Nick Martin",
              "email": "nick@rocketship.sh"
            }
        assertions:
          - type: status_code
            expected: 200
        save:
          - json_path: ".id"
            as: "user_id"

      - name: "Verify user in browser"
        plugin: playwright
        config:
          role: script
          script: |
            from playwright.sync_api import expect

            # Navigate to user profile
            page.goto("{{ .vars.base_url }}/users/{{ user_id }}")

            # Verify user details are displayed
            expect(page.locator("h1")).to_contain_text("Nick Martin")

            result = {"verified": True}
```

### Run it

```bash
rocketship run -ad .rocketship # starts the local engine, runs the tests, shuts the engine down
```

## Give Your Coding Agent Context on Rocketship

Paste the [ROCKETSHIP_QUICKSTART.md](https://raw.githubusercontent.com/rocketship-ai/rocketship/main/ROCKETSHIP_QUICKSTART.md) file into your coding agent's context window, so that it understands how to build and run tests.

## Documentation

[https://docs.rocketship.sh](https://docs.rocketship.sh)

## Contribute!!!

I would love to build this with you! Reach out to me on [LinkedIn](https://www.linkedin.com/in/magiusdarrigo) and let's chat. A great first contribution is building a [plugin](https://github.com/rocketship-ai/rocketship/tree/main/internal/plugins) for your favorite API. If you want to contribute to Rocketship, start by reading [Contributing to Rocketship](https://docs.rocketship.sh/contributing).

## License

Rocketship is distributed under the [MIT license](https://github.com/rocketship-ai/rocketship/blob/main/LICENSE).
