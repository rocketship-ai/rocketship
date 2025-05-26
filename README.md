<p align="center">
  <img src="docs/src/assets/transparent.png#gh-light-mode-only" alt="Rocketship black logo" width="210" style="display: block; margin: 0 auto; padding: 20px;">
  <img src="docs/src/assets/transparent-reverse.png#gh-dark-mode-only" alt="Rocketship white logo" width="210" style="display: block; margin: 0 auto; padding: 20px;">
</p>
<h3 align="center">Enterprise-Grade API Testing for Humans and Agents</h3>
<p align="center">Validate Any Data Resource, API, or Workflow With Declarative Tests</p>

<p align="center">
  <a href="https://github.com/rocketship-ai/rocketship/releases"><img src="https://img.shields.io/github/v/release/rocketship-ai/rocketship.svg" alt="Github release"></a>
  <a href="https://github.com/rocketship-ai/rocketship/actions/workflows/all.yml"><img src="https://github.com/rocketship-ai/rocketship/actions/workflows/release.yml/badge.svg" alt="Build status"></a>
  <a href="https://goreportcard.com/report/github.com/rocketship-ai/rocketship"><img src="https://goreportcard.com/badge/github.com/rocketship-ai/rocketship" alt="Go Report Card"></a>
  <br>
</p>
<p align="center">
    <a href="https://github.com/rocketship-ai/rocketship/releases">Download</a> ·
    <a href="https://docs.rocketship.sh">Documentation</a> ·
</p>

<br>

🚀 Rocketship is an **open‑source testing platform** that can verify complex, API-driven scenarios that are made by your customers— or your systems. Today's world is filled with event-driven micro-services that can be hard to test. Rocketship brings durable execution backed by **Temporal** to your testing infra, and offers extensible [plugins](https://github.com/rocketship-ai/rocketship/tree/main/internal/plugins) so you can add the APIs and protocols that matter to you.

Define your test scenarios as **declarative YAML specs** -> and have Rocketship run them locally or in your cloud environment.

Core features:

- **Rocketship CLI** Run the engine locally or connect to a remote address.
- **Deploy-Ready Images** Need long-running, highly-scalable tests? Or just want to save test history? Host Rocketship on your own infra.
- **Declarative YAML** Define your test scenarios as declarative YAML specs.
- **Durable Execution** Need a test step to retry? Or a test to run for 10 hours? No problem!
- **Plugin Ecosystem** Add the APIs and protocols that matter to you.

## Getting Started

#### Install

```bash
brew install temporal # pre-req for if you want to run the local engine
```

```bash
# for arm64 macos
curl -Lo /usr/local/bin/rocketship https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-arm64
chmod +x /usr/local/bin/rocketship
```

For detailed installation instructions for other platforms and optional aliases, see the [Installation Guide](https://docs.rocketship.sh/installation).

#### Save a test spec

```bash
cat > simple-test.yaml << 'EOF'
name: "Simple Test Suite"
description: "A simple test suite!"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Create a test user"
        plugin: "http"
        config:
          method: "POST"
          url: "https://tryme.rocketship.sh/users"
          body: |
            {
              "name": "Test User",
              "email": "test@example.com"
            }
        assertions:
          - type: "json_path"
            path: ".name"
            expected: "Test User"
  - name: "Test 2"
    steps:
      - name: "Create a test order"
        plugin: "http"
        config:
          method: "POST"
          url: "https://tryme.rocketship.sh/orders"
          body: |
            {
              "product": "Test Product",
              "quantity": 1
            }
        assertions:
          - type: "status_code"
            expected: 200
EOF
```

#### Run it

```bash
rocketship run -af simple-test.yaml # starts the local engine, runs the tests, shuts the engine down
```

The examples use a hosted test server at `tryme.rocketship.sh` that you can use:

- Test CRUD operations for a resource type
- Resources are isolated based off a session header
- FYI: Resource cleanup is done hourly (every :00)

## Documentation

[https://docs.rocketship.sh](https://docs.rocketship.sh)

## Roadmap

Building the next-gen of integration testing for humans and AI agents. Suggestions and issues are welcomed! Here's what's coming in weeks, not years:

- [ ] **AI Agent Integration** MCP support for AI agents to automatically generate, run, and maintain integration tests based on code changes.
- [ ] **Data Resource Plugins** Native plugin support for secret managers, databases (PostgreSQL, MongoDB), message queues (Kafka, RabbitMQ), file systems (S3, GCS), and more.
- [ ] **LLM Browser Testing** A plugin powered by [Workflow Use](https://github.com/browser-use/workflow-use) for deterministic browser-based testing.
- [ ] **Test and Suite-Wide Config** Schedule tests on a cadence, add retryability, and more.
- [x] **Parameterized Tests & Scripting** Parameterize your tests with environment variables, secrets, and scripted steps.

## Contribute!!!

I would love to build this with you! I'm looking to start a community for 🚀. Reach out to me on [LinkedIn](https://www.linkedin.com/in/magiusdarrigo) and let's chat. A great first contribution is building a [plugin](https://github.com/rocketship-ai/rocketship/tree/main/internal/plugins) for your favorite API. If you want to contribute to Rocketship, start by reading [Contributing to Rocketship](https://docs.rocketship.sh/contributing).

## License

Rocketship is distributed under the [MIT license](https://github.com/rocketship-ai/rocketship/blob/main/LICENSE).
