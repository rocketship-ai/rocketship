# Rocketship

---

### 🚀 **Rocketship AI** – AI-Native End-to-End API Testing

**Rocketship** is an open-source, AI-driven platform revolutionizing end-to-end API testing. Designed to reduce manual overhead, Rocketship AI uses intelligent automation to streamline test creation and maintenance, enabling developers to build robust software faster.

### 🎯 Vision & Manifesto

We envision a world where software testing is automated intelligently and continuously evolves alongside your application code. Our mission is to empower developers to focus on innovation rather than maintenance, ensuring high-quality software releases at rapid speed.

### Core Features:

- **Automated AI-Driven Test Generation**: Automatically generate robust API test suites directly from your backend code.
- **Diff-Based Test Maintenance**: AI-driven test suggestions based on code changes and pull requests.
- **Event-Driven Test Execution**: Tests are triggered precisely when needed, ensuring efficiency and timely feedback.
- **Seamless CI/CD Integration**: Direct integration with popular CI/CD tools including Buildkite and GitHub Actions.
- **Developer-Centric Approach**: Quick and easy setup using Docker Compose for local development and Helm charts for Kubernetes deployments.
- **Flexible Authentication**: Optional, simple security through environment variables.

### 📋 Table of Contents

- [Quick Start](#quick-start)
- [Installation (CLI)](#installation-cli)
- [Architecture & Technology Stack](#architecture--technology-stack)
- [How Rocketship AI Works](#how-rocketship-ai-works)
- [Contributing](#contributing)
- [Roadmap](#roadmap)
- [License](#license)

### 🚀 Quick Start

Spin up Rocketship AI locally with Docker:

```bash
docker-compose up -d
```

Deploy on Kubernetes using Helm:

```bash
helm install rocketship-ai rocketship/rocketship-chart
```

### 📦 Installation (CLI)

Install Rocketship AI CLI with Homebrew:

```bash
brew install rocketship
```

Generate tests easily from your repository:

```bash
rocketship generate-tests --repo=https://github.com/your-repo
```

### 🔧 Architecture & Technology Stack

Rocketship AI uses cutting-edge, scalable technologies:

- **Backend**: Node.js (TypeScript)
- **Frontend**: React.js
- **Messaging**: NATS (lightweight pub/sub)
- **AI Engine**: Mastra, GPT-4, Code Llama
- **Database**: PostgreSQL
- **Embedding DB**: ChromaDB for vector-based code indexing

### 🔍 How Rocketship AI Works

Rocketship employs specialized AI agents:

- **Migration Agent**: Scans backend code to auto-generate API tests, significantly reducing initial test creation time.
- **Diff Agent**: Automatically detects code changes (via GitHub webhooks) and suggests necessary test updates.

Tests run dynamically through an event-driven architecture using NATS as the message queue, ensuring real-time, precise execution aligned with code changes.

### 🤝 Contributing

Rocketship AI warmly welcomes contributions:

- Fork and clone the repository
- Follow the coding guidelines in our [Contribution Guide](docs/contribution.md)
- Submit Pull Requests clearly documenting your changes

Community feedback and contributions are highly valued, and we actively encourage discussions and improvements via GitHub issues.

### 📅 Roadmap

Rocketship AI has a structured roadmap with incremental enhancements planned:

- **v1.0**: Initial API testing capability with Migration and Diff Agents
- **v1.5**: Introduce browser-based test support with intelligent DOM interactions
- **v2.0**: Expanded integrations and deeper AI-driven test analysis and maintenance
- **v2.5+**: Cloud-hosted managed solutions and advanced test scenarios (performance, load testing)

### 📄 License

Rocketship AI is open-sourced under the MIT License.
