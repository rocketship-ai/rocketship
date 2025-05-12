# Rocketship

![Under Construction](docs/misc/assets/under-construction-banner.png)

### 🚀 **Rocketship** – AI‑Native End‑to‑End Testing

Rocketship is an **open‑source, AI‑powered platform** that verifies complex, event‑driven micro‑services the same way you reason about them: as real‑world **workflows** that span queues, APIs, databases, and file buckets.  
It combines a declarative YAML spec, Temporal‑style durable execution, and an LLM "Test‑Copilot" that keeps your tests in sync with every code change—whether written by humans or autonomous agents.

---

## 🐞 What Problems Does Rocketship Solve?

| Pain                             | Traditional Reality                                                                   | Rocketship Fix                                                                                               |
| -------------------------------- | ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| **1. Async complexity**          | Existing API tools assume HTTP request‑response; Async flows are hand‑rolled scripts. | First‑class plugins for SQS, Kinesis, Dynamo, S3, HTTP, …                                                    |
| **2. Test drift**                | Code changes faster than tests; flakiness grows. Tests become outdated.               | **LLM Diff‑Copilot** scans your PR diff → proposes YAML patch; optional auto‑merge.                          |
| **3. CI headaches**              | Full E2E env is heavy, slow, and flaky.                                               | Temporal‑based runner spins timers & retries _without_ holding CI pods; run in your cluster or local Docker. |
| **4. Security / data residency** | SaaS testing tools require exposing internal endpoints.                               | Tests execute in **Rocketship Agent** pods that are part of your infra—only test metadata leaves the VPC.    |
| **5. AI agent deploy risk**      | Agents can commit code 24/7; unsafe merges land in prod.                              | Agents call Rocketship's MCP/gRPC API → must get green tests before `git push`.                              |

---

## ✨ Core Features

- **YAML Specs (`rocketship.yaml`)** – Declarative steps: publish message ➜ sleep ➜ assert DB row ➜ assert S3 object.
- **Plugin & Connector SDK** – Drop‑in Go package; implement one Activity function and a JSON schema to add Azure, GCP, or custom infra.
- **Temporal‑powered Engine** – Durable workflows, back‑offs, and long timers without hogging threads.
- **AI Diff‑Copilot** – `rocketship suggest --diff HEAD~1` emits a ready‑to‑commit patch that updates or adds tests.
- **Local‑first / K8s‑native** – `rocketship start` spins Temporal + Engine + Agent + LocalStack via Docker Compose (or Helm in minikube).
- **CI Plugins** – Buildkite Orb and GitHub Action sample provided.
- **MCP Server Mode** _(opt‑in)_ – Expose Rocketship as a [Model Context Protocol](https://mcp.dev) capability so any LLM agent can invoke `runTest`, `listTests`, or `generateTests`.

---

## 🟢 1‑Minute Quick Start

# 0. Install the Prerequisites

# You'll need Temporal to run the engine locally

# macOS

brew install temporal

# Linux

curl -sSf https://temporal.download/cli.sh | sh

# 1. Install the Rocketship CLI

## Option A: Direct Download

# macOS (Apple Silicon)

curl -LO https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-arm64
chmod +x rocketship-darwin-arm64
sudo mv rocketship-darwin-arm64 /usr/local/bin/rocketship

# macOS (Intel)

curl -LO https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-amd64
chmod +x rocketship-darwin-amd64
sudo mv rocketship-darwin-amd64 /usr/local/bin/rocketship

# Linux (x86_64)

curl -LO https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-linux-amd64
chmod +x rocketship-linux-amd64
sudo mv rocketship-linux-amd64 /usr/local/bin/rocketship

# Windows

# Download rocketship-windows-amd64.exe from the releases page and rename to rocketship.exe

## Option B: Using Go Install (For my fellow Gophers)

# Requires Go 1.24+

go install github.com/rocketship-ai/rocketship/cmd/rocketship@latest

# 2. Start the Local Server (in terminal 1)

rocketship start server --local

# 3. [OPTIONAL] Create a Session (in terminal 2)

rocketship start session --engine localhost:7700

# 4. Create a Test File OR better yet, try one of the examples (examples/simple-http/rocketship.yaml)

cat > simple-test.yaml << 'EOF'
name: "Simple Delay Test Suite"
description: "A simple test suite that demonstrates delays"
version: "v1.0.0"
tests:

- name: "Test 1"
  steps:
  - name: "Wait for 5 seconds"
    plugin: "delay"
    config:
    duration: "5s"
- name: "Test 2"
  steps: - name: "Wait for 10 seconds"
  plugin: "delay"
  config:
  duration: "10s"
  EOF

# 5. Run the Test (engine flag is optional if you have a session)

rocketship run --file simple-test.yaml --engine localhost:7700

```

```
