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

🚀 Rocketship is an **open‑source testing framework** that can verify complex, API-driven scenarios that are made by your customers— or your systems. Rocketship brings durable execution backed by **Temporal** to your testing infra, and offers extensible [plugins](https://github.com/rocketship-ai/rocketship/tree/main/internal/plugins) so you can add the APIs and protocols that matter to you.

Define your test scenarios as **declarative YAML specs** -> and have Rocketship run them locally or in your cloud environment as deterministic workflows.

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
cat > rocketship.yaml << 'EOF'
name: "Simple Test Suite"
description: "Showing some of the plugins"
version: "v1.0.0"
vars:
  base_url: "https://tryme.rocketship.sh"
tests:
  - name: "User Workflow with Processing Delay"
    steps:
      - name: "Create a new user"
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
          - type: json_path
            path: ".name"
            expected: "Nick Martin"
        save:
          - json_path: ".id"
            as: "user_id"

      - name: "Wait for user processing"
        plugin: delay
        config:
          duration: "2s"

      - name: "Validate user creation with script"
        plugin: script
        config:
          language: javascript
          script: |
            function main() {
              const userId = state.user_id;

              // Simulate some business logic validation
              if (!userId || userId === "") {
                throw new Error("User ID is missing or empty");
              }

              return {
                validation_status: "passed",
                user_ready: true,
                message: `User ${userId} is ready for operations`
              };
            }

            main();
EOF
```

#### Run it

```bash
rocketship run -af rocketship.yaml # starts the local engine, runs the tests, shuts the engine down
```

The examples use a hosted test server at `tryme.rocketship.sh` that you can use:

- Test CRUD operations for a resource type
- Resources are isolated based off a session header
- FYI: Resource cleanup is done hourly (every :00)

## Try the MCP Server

Rocketship includes an MCP (Model Context Protocol) server that enables AI coding agents like Cursor, Windsurf, or Claude to automatically generate and manage your test suites.

#### Quick Setup

**For Claude Code:**

Add to the `.mcp.json` file in your project:

```json
{
  "mcpServers": {
    "rocketship": {
      "command": "npx",
      "args": ["-y", "@rocketshipai/mcp-server@latest"]
    }
  }
}
```

**Head to the [Rocketship MCP Server Docs](https://docs.rocketship.sh/mcp-server) to see how to add it for Cursor, Windsurf, and more.**

#### What Can It Do?

The MCP server provides these tools to AI assistants:

- **`scan_and_generate_test_suite`** - Analyzes your codebase and generates a complete test structure
- **`generate_test_from_prompt`** - Creates test files from natural language descriptions
- **`validate_test_file`** - Validates Rocketship YAML files
- **`run_and_analyze_tests`** - Executes tests and provides intelligent failure analysis
- **`analyze_git_diff`** - Suggests test updates based on code changes

#### Example Usage

Just ask your coding agent:

- "Generate API tests for my authentication endpoints"
- "Create a test suite for my Supabase database operations"
- "Update my tests based on the latest PR changes"
- "Run my integration tests and explain any failures"

The MCP server intelligently selects the right Rocketship plugins (HTTP, SQL, Supabase, etc.) based on your request and generates valid test configurations ready to run.

## Documentation

[https://docs.rocketship.sh](https://docs.rocketship.sh)

## Roadmap

Building the next-gen of integration testing for humans and AI agents. Suggestions and issues are welcomed! Here's what's coming in weeks, not years:

- [x] **Parameterized Tests & Scripting** Parameterize your tests with environment variables, secrets, and scripted steps.
- [x] **AI Agent Integration** MCP support for AI agents to automatically generate, run, and maintain integration tests based on code changes.
- [x] **LLM Browser Testing** A plugin powered by [Browser Use](https://github.com/browser-use/browser-use) for browser-based testing.
- [ ] **Test and Suite-Wide Config** Schedule tests on a cadence, add retryability, and more.
- [ ] **More Native Plugins** Native plugin support for secret managers, databases (PostgreSQL, MongoDB), message queues (Kafka, RabbitMQ), file systems (S3, GCS), and more.

## Contribute!!!

I would love to build this with you! I'm looking to start a community for 🚀. Reach out to me on [LinkedIn](https://www.linkedin.com/in/magiusdarrigo) and let's chat. A great first contribution is building a [plugin](https://github.com/rocketship-ai/rocketship/tree/main/internal/plugins) for your favorite API. If you want to contribute to Rocketship, start by reading [Contributing to Rocketship](https://docs.rocketship.sh/contributing).

## License

Rocketship is distributed under the [MIT license](https://github.com/rocketship-ai/rocketship/blob/main/LICENSE).
