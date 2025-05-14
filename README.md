# Rocketship

### 🚀 **Rocketship** – Run Enterprise-Grade e2e Tests With a Single Command

Rocketship is an **open‑source testing platform** that verifies complex, event‑driven micro‑services the same way you reason about them: as real‑world **workflows** that span queues, APIs, databases, and file buckets.  
It combines a declarative YAML spec with Temporal‑style durable execution to provide reliable, scalable testing for modern architectures.

---

## 🐞 What Problems Does Rocketship Solve?

| Pain                             | Traditional Reality                                                                   | Rocketship Fix                                                                                               |
| -------------------------------- | ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| **1. Async complexity**          | Existing API tools assume HTTP request‑response; Async flows are hand‑rolled scripts. | First‑class plugins for Delays & HTTP. With SQS, Kinesis, Dynamo, S3, etc. coming soon.                      |
| **2. CI headaches**              | Full E2E env is heavy, slow, and flaky.                                               | Temporal‑based runner spins timers & retries _without_ holding CI pods; run in your cluster or local Docker. |
| **3. Security / data residency** | SaaS testing tools require exposing internal endpoints.                               | Tests can optionally execute in **Rocketship Agent** pods that are part of your infra.                       |

---

## ✨ Core Features

- **YAML Specs (`rocketship.yaml`)** – Declarative steps: publish message ➜ sleep ➜ assert DB row ➜ assert S3 object.
- **Plugin & Connector SDK** – Drop‑in Go package; implement one Activity function and a JSON schema to add Azure, GCP, or custom infra.
- **Temporal‑powered Engine** – Durable workflows, back‑offs, and long timers without hogging threads.
- **Local‑first / K8s‑native** – `rocketship start` spins Temporal + Engine + Agent + LocalStack via Docker Compose (or Helm in minikube).
- **CI Plugins** – Buildkite, Jenkins, GitHub Actions, etc. samples coming soon.

---

## 🟢 1‑Minute Quick Start

```bash
# 0. Install the Prerequisites (You're going to need temporal in order to run the engine locally)
# macOS
brew install temporal

# 1. Install the Rocketship CLI

####### OPTION 1: Direct Download #######
# macOS (Apple Silicon)
curl -LO https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-arm64
chmod +x rocketship-darwin-arm64
sudo mv rocketship-darwin-arm64 /usr/local/bin/rocketship


####### OPTION 2: Using Go Install (for my fellow Gophers) #######
# using Go
go install github.com/rocketship-ai/rocketship/cmd/rocketship@latest

# 2. Start the Local Server (in terminal 1)
rocketship start server --local

# 3. Run the Test (engine flag is optional if you have a session)
rocketship run --file simple-test.yaml --engine localhost:7700
```

## 🐳 Docker Quick Start

```bash
# Pull the image
docker pull rocketshipai/rocketship:latest

# Run a test suite by mounting a directory to the container
# Use TEST_FILE or TEST_DIR to specify the rocketship.yaml file or directory
docker run -v "$(pwd)/examples:/tests" \
  -e TEST_FILE=/tests/simple-http/rocketship.yaml \
  rocketshipai/rocketship:latest
```

## 🗺️ Roadmap

1. **AI-Powered Test Generation**

   - LLM integration for generating test cases from code changes
   - Automatic test maintenance based on code diffs
   - Natural language test case description and generation

2. **Enhanced Plugin Ecosystem**

   - Kafka plugin for message streaming
   - MongoDB and Redis plugins for NoSQL testing
   - gRPC plugin for service-to-service testing
   - GraphQL plugin for API testing

3. **Developer Experience**

   - Interactive test debugger with step-through capability
   - Visual test flow builder and editor
   - Real-time test execution visualization
   - Enhanced test reporting with insights and trends

4. **Enterprise Features**

   - Role-based access control (RBAC)
   - Test environment management
   - Secrets management integration
   - Test data management and cleanup

5. **Cloud Integration**

   - Native Azure and GCP plugins
   - Cloud-specific best practices and patterns
   - Multi-cloud test orchestration

6. **Performance & Scale**
   - Distributed test execution
   - Test sharding and parallelization
   - Resource optimization for large test suites

Want to contribute? Check out our [contribution guidelines](CONTRIBUTING.md)
