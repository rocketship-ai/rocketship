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

## 🟢 5‑Minute Quick Start

```bash
# 1. Install Prerequisites & CLI
# macOS
brew install go temporal && go install github.com/rocketship-ai/rocketship/cmd/rocketship@latest

# Linux
curl -sSf https://temporal.download/cli.sh | sh && \
  # Install Go 1.24+ from https://go.dev/dl/
  go install github.com/rocketship-ai/rocketship/cmd/rocketship@latest

# 2. Start the Local Server (in terminal 1)
rocketship start server --local

# 3. [OPTIONAL] Create a Session (in terminal 2)
rocketship start session --engine localhost:7700

# 4. Create a Test File
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
    steps:
      - name: "Wait for 10 seconds"
        plugin: "delay"
        config:
          duration: "10s"
EOF

# 5. Run the Test (engine flag is optional if you have a session)
rocketship run --file simple-test.yaml --engine localhost:7700
```

You should see output like:

```
Starting test run "Simple Delay Test Suite"... 🚀
Running test: "Test 1"...
Running test: "Test 2"...
Test: "Test 1" passed
Test: "Test 2" passed
Test run: "Simple Delay Test Suite" finished. All 2 tests passed.
```
