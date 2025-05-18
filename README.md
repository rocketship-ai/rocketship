<p align="center">
  <a href="https://docs.rocketship.sh">
    <img src="docs/misc/assets/transparent.png#gh-light-mode-only" alt="Rocketship black logo" width="210" />
    <img src="docs/misc/assets/transparent-reverse.png#gh-dark-mode-only" alt="Rocketship white logo" width="210" />
  </a>
</p>
<h3 align="center">E2E API Testing For Any Cloud Environment</h3>
<p align="center">Run Enterprise-Grade e2e Tests With a Single Command</p>

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

🚀 Rocketship is an **open‑source testing engine** that can verify complex, API-driven scenarios that are made by your customers— or your systems. Today's world is filled with event-driven micro-services that can be hard to test. Rocketship brings durable execution backed by **Temporal** to your testing infra, and offers an extensible plugin system so you can add the APIs and protocols that matter to you.

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
brew install temporal # pre-req for the local engine
```

```bash
curl -Lo /usr/local/bin/rs https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-arm64 && chmod +x /usr/local/bin/rs # for arm64 macos
```

#### Save a test spec

```bash
cat > simple-test.yaml << 'EOF'
name: "Simple Test Suite"
description: "A simple test suite!"
version: "v1.0.0"
tests:
  - name: "Test 1"
    steps:
      - name: "Check API status"
        plugin: "http"
        config:
          method: "GET"
          url: "https://httpbin.org/status/200"
        assertions:
          - type: "status_code"
            expected: 200
  - name: "Test 2"
    steps:
      - name: "Do nothing for 1s!"
        plugin: "delay"
        config:
          duration: "1s"
EOF
```

#### Run it

```bash
rs run -af simple-test.yaml # starts the engine, runs the tests, shuts the engine down
```

You can run scripts like this on the CLI, or in your CI, or across a Kubernetes cluster.

## Documentation

Working on it!

## Roadmap

I have a ton of ideas for Rocketship, and I'm open to any and all suggestions. Here are just some of the things you can expect in weeks not years:

- [ ] **LLM Browser Testing Support** A plugin powered by [Workflow Use](https://github.com/browser-use/workflow-use) to build & run deterministic browser tests.
- [ ] **Smoke Testing** A test suite-wide configuration to schedule tests to run on a cadence.
- [ ] **Environment Variables** Pass in environment variables to your tests. Run tests against different environments.
- [ ] **Core AWS Plugins** Add support for AWS services like S3, SQS, SNS, etc. Other providers to follow.
- [ ] **Agentic Friendly Testing** Vibe code in peace. Have your agent iteratively test your codebase for regressions.

## Contribute!!!

I would love to build this with you! I'm looking to start a community for 🚀. Reach out to me on [X](https://x.com/matteo_agius) or [LinkedIn](https://www.linkedin.com/in/magiusdarrigo) and let's chat. A great first contribution is building a [plugin](https://github.com/rocketship-ai/rocketship/tree/main/internal/plugins) for your favorite API. If you want to contribute to Rocketship, start by reading [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Rocketship is distributed under the [MIT license](https://github.com/rocketship-ai/rocketship/blob/main/LICENSE).
