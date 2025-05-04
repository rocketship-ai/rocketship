# Rocketship

### 🚀 **Rocketship** – AI‑Native End‑to‑End Testing for Cloud‑Native Systems

Rocketship is an **open‑source, AI‑powered platform** that verifies complex, event‑driven micro‑services the same way you reason about them: as real‑world **workflows** that span queues, APIs, databases, and file buckets.  
It combines a declarative YAML spec, Temporal‑style durable execution, and an LLM “Test‑Copilot” that keeps your tests in sync with every code change—whether written by humans or autonomous agents.

---

## 🐞 What Problems Does Rocketship Solve?

| Pain                             | Traditional Reality                                                                  | Rocketship Fix                                                                                               |
| -------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ |
| **1. Async complexity**          | Existing API tools assume request‑response; Kafka/SQS flows are hand‑rolled scripts. | First‑class connectors for SQS, Kinesis, Dynamo, S3, HTTP, gRPC, …                                           |
| **2. Test drift**                | Code changes faster than tests; flakiness grows.                                     | **LLM Diff‑Copilot** scans your PR diff → proposes YAML patch; optional auto‑merge.                          |
| **3. CI headaches**              | Full E2E env is heavy, slow, and flaky.                                              | Temporal‑based runner spins timers & retries _without_ holding CI pods; run in your cluster or local Docker. |
| **4. Security / data residency** | SaaS testing tools require exposing internal endpoints.                              | Tests execute in **Rocketship Agent** pods you control—only metadata leaves the VPC.                         |
| **5. AI agent deploy risk**      | Agents can commit code 24/7; unsafe merges land in prod.                             | Agents call Rocketship’s MCP/gRPC API → must get green tests before `git push`.                              |

---

## ✨ Core Features

- **YAML Specs (`rocketship.yaml`)** – Declarative steps: publish message ➜ sleep ➜ assert DB row ➜ assert S3 object.
- **Plugin & Connector SDK** – Drop‑in Go package; implement one Activity function and a JSON schema to add Azure, GCP, or custom infra.
- **Temporal‑powered Engine** – Durable workflows, back‑offs, and long timers without hogging threads.
- **AI Diff‑Copilot** – `rocketship suggest --diff HEAD~1` emits a ready‑to‑commit patch that updates or adds tests.
- **Local‑first / K8s‑native** – `rocketship start` spins Temporal + Engine + Agent + LocalStack via Docker Compose (or Helm in minikube).
- **CI Plugins** – Buildkite Orb and GitHub Action sample provided.
- **MCP Server Mode** _(opt‑in)_ – Expose Rocketship as a [Model Context Protocol](https://mcp.dev) capability so any LLM agent can invoke `runTest`, `listTests`, or `generateTests`.

---

## 🟢 5‑Minute Quick Start

```bash
# 1.  Install CLI
go install github.com/rocketship/rocketship/cmd/rocketship@latest

# 2.  Bootstrap local stack (Temporal, Agent, LocalStack)
rocketship start

# 3.  Init sample test
rocketship init --example order-workflow
cat rocketship.yaml           # peek at the spec

# 4.  Run end‑to‑end test
rocketship run

# 5.  Watch logs / status
rocketship logs $(rocketship status --latest)
```
