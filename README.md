# Rocketship

### 🚀 **Rocketship** – AI-Native End-to-End API Testing

**Rocketship** is an open-source, AI-driven platform revolutionizing end-to-end API testing. Designed to reduce manual overhead, Rocketship uses intelligent automation to streamline test creation and maintenance, enabling developers to build robust software faster.

### 🎯 Vision & Manifesto

We envision a world where software testing is automated intelligently and continuously evolves alongside your application code. Our mission is to empower developers to focus on innovation rather than maintenance, ensuring high-quality software releases at rapid speed.

### Core Features:

- **Automated AI-Driven Test Generation**: Automatically generate robust API and browser test suites directly from your codebase.
- **Diff-Based Test Maintenance**: AI-driven test suggestions and updates based on code changes and pull requests.
- **Event-Driven Test Execution**: Tests are triggered precisely when needed, ensuring efficiency and timely feedback.
- **Seamless CI/CD Integration**: Direct integration with popular CI/CD tools including Buildkite and GitHub Actions.
- **Developer-Centric Approach**: Quick and easy setup using Docker Compose for local development and Helm charts for Kubernetes deployments.
- **Flexible LLM Replacement**: Decide whether to use proprietary models or your own locally running ones via environment variables.

### 📋 Table of Contents

- [Quick Start](#-quick-start)
- [Installation (CLI)](#-installation-cli)
- [Architecture & Technology Stack](#-architecture--technology-stack)
- [How Rocketship Works](#-how-rocketship-works)
- [Contributing](#-contributing)
- [Roadmap](#-roadmap)
- [License](#-license)

### 🚀 Quick Start

Spin up Rocketship locally with Docker:

```bash
docker-compose up -d
```

Deploy on Kubernetes using Helm:

```bash
helm install rocketship-ai rocketship/rocketship-chart
```

### 📦 Installation (CLI)

Install Rocketship CLI with Homebrew:

```bash
brew install rocketship
```

Generate tests easily from your directory:

```bash
rocketship generate-tests .
```

### 🔧 Architecture & Technology Stack

Rocketship uses cutting-edge, scalable technologies:

- **Backend**: Typescript & Go
- **Frontend**: React.js
- **Messaging**: NATS (lightweight pub/sub)
- **AI Engine**: Mastra, GPT-4, Code Llama
- **Database**: PostgreSQL
- **Embedding DB**: ChromaDB for vector-based code indexing

### 🔍 How Rocketship Works

Rocketship employs specialized AI agents and an event-driven architecture to handle all aspects of end-to-end testing, from initial test generation to continuous updates:

- **Migration Agent**: Scans your codebase to automatically create end-to-end (e2e) tests, significantly reducing the time needed to bootstrap test coverage.
- **Diff Agent**: Identifies changes in your code (via pull requests or commits) and generates updated or additional tests as needed.

In the default Docker Compose setup, the following containers provide a fully functional Rocketship environment:

1. **rocketship_api**

   - **Role**: The core backend service. It receives requests, orchestrates AI agents, handles test definitions, and exposes APIs for the UI and CLI.
   - **Implementation**: A combination of TypeScript/Go services, leveraging frameworks like Express or NestJS for endpoint management.

2. **rocketship_frontend**

   - **Role**: A React-based user interface where you can review AI-generated tests, trigger test runs, and configure your testing environment.
   - **Implementation**: Serves static React files or a Next.js application, accessible at a specific port (e.g., http://localhost:3000).

3. **rocketship_nats**

   - **Role**: A NATS server that handles all pub/sub messaging between services. When code changes or manual triggers occur, events are published to NATS, prompting test runs or AI-based test generation.
   - **Implementation**: Runs a lightweight NATS instance, providing an event-driven backbone for Rocketship.

4. **rocketship_db**

   - **Role**: A PostgreSQL database that stores test definitions, user configurations, and run results.
   - **Implementation**: Runs a standard Postgres container with data persistence.

5. **rocketship_vectorstore**

   - **Role**: A containerized instance of ChromaDB or another vector database, used to store and retrieve embeddings of your code and tests. This lets AI agents accurately generate or update tests by referencing relevant code snippets.
   - **Implementation**: Manages vector-based embeddings, ensuring quick semantic search for AI tasks.

6. **rocketship_llm (optional)**
   - **Role**: If you choose to run a local Large Language Model (like Code Llama), you can include a container that provides an LLM API endpoint. Otherwise, Rocketship can integrate with external/proprietary models via environment variables.
   - **Implementation**: Runs an open-source model or interacts with a GPU-powered environment if available.

This architecture ensures your AI agents have immediate access to the data they need (database and embeddings), while events flow naturally via NATS, triggering test generation or maintenance whenever relevant changes occur.

### 🤝 Contributing

Rocketship AI warmly welcomes contributions:

- Fork and clone the repository
- Follow the coding guidelines in our [Contribution Guide](docs/contribution.md)
- Submit Pull Requests clearly documenting your changes

Community feedback and contributions are highly valued, and we actively encourage discussions and improvements via GitHub issues.

### 📅 Roadmap

Rocketship has a structured roadmap with incremental enhancements planned:

- **v1.0**: Initial API testing capability with Migration and Diff Agents
- **v1.5**: Introduce browser-based test support with intelligent DOM interactions
- **v2.0**: Expanded integrations and deeper AI-driven test analysis and maintenance
- **v2.5+**: Cloud-hosted managed solutions and advanced test scenarios (performance, load testing)

### 📄 License

Rocketship is open-sourced under the MIT License.
