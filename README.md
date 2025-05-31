<p align="center">
  <img src="docs/src/assets/transparent.png#gh-light-mode-only" alt="Rocketship black logo" width="210">
  <img src="docs/src/assets/transparent-reverse.png#gh-dark-mode-only" alt="Rocketship white logo" width="210">
</p>

<h3 align="center">CI/CD for AI Agents — Autonomous Pipelines from <kbd>git push</kbd> to Production</h3>
<p align="center">Build · Test · Fix · Deploy — all orchestrated by Rocketship + your favorite LLMs</p>

<p align="center">
  <a href="https://github.com/rocketship-ai/rocketship/releases"><img src="https://img.shields.io/github/v/release/rocketship-ai/rocketship.svg"></a>
  <a href="https://github.com/rocketship-ai/rocketship/actions/workflows/all.yml"><img src="https://github.com/rocketship-ai/rocketship/actions/workflows/release.yml/badge.svg"></a>
  <a href="https://goreportcard.com/report/github.com/rocketship-ai/rocketship"><img src="https://goreportcard.com/badge/github.com/rocketship-ai/rocketship"></a>
</p>

<p align="center">
  <a href="https://github.com/rocketship-ai/rocketship/releases">Download</a> ·
  <a href="https://docs.rocketship.sh">Documentation</a>
</p>

---

> **Rocketship 2.0** turns your repository into a fully‑autonomous **DAG‑based CI/CD system** driven by AI agents.  
> Ship faster by letting agents build, test, _fix_, and deploy code while you focus on product.

### Why Rocketship?

|                  | Traditional CI                      | **Rocketship**                                                                        |
| ---------------- | ----------------------------------- | ------------------------------------------------------------------------------------- |
| Pipeline engine  | YAML → shell runners                | YAML → **Temporal** workflows (durable, stateful)                                     |
| Agents           | DIY scripting                       | **Native Multi‑Agent Control Plane (MCP)**                                            |
| Failure handling | Fail fast                           | **Self‑healing loops** (call LLM, patch code, re‑run tests)                           |
| Hosting          | Cloud SaaS _or_ self‑hosted runners | **Hybrid**: keep your code & secrets on your infra, Rocketship provides control plane |
| Extensibility    | Marketplace / plugins               | **First‑class plugin SDK** (build, test, deploy, chat‑GPT, …)                         |

### Core Features

- **DAG Pipelines** — define any build/test/deploy graph with simple YAML.
- **AI Loops** — on failure Rocketship can call an LLM, apply the generated patch, and continue until green ✅.
- **Multi‑Agent Control Plane** — coordinate multiple specialised agents (coder, reviewer, tester, SRE, …) in one run.
- **Durable Execution** — powered by [Temporal], pipelines survive crashes, scale horizontally, and support long‑running jobs.
- **Bring‑Your‑Own‑Infra** — run workers on Kubernetes, bare metal, Railway, Render, or anywhere Docker runs.
- **Plugin Ecosystem** — shell, Docker, Kubernetes, HTTP, SQL, Terraform, Vercel/Render/Railway deploy, OpenAI, Anthropic… build your own in Go.

---

## Quick Start ⏱️

> **Prereqs:** Go 1.22+, Docker, and (for local development) the Temporal server.

```bash
# 1. install Temporal locally (or point to a remote cluster)
brew install temporal

# 2. install the Rocketship CLI
curl -Lo /usr/local/bin/rocketship \
  https://github.com/rocketship-ai/rocketship/releases/latest/download/rocketship-darwin-arm64
chmod +x /usr/local/bin/rocketship
```
