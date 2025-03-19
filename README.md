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

Rocketship employs specialized AI agents and a temporal-based architecture to handle all aspects of end-to-end testing, from initial test generation to test execution and continuous updates:

- **Migration Agent**: Scans your codebase to automatically create end-to-end (e2e) tests, significantly reducing the time needed to bootstrap test coverage.
- **Diff Agent**: Identifies changes in your code (via pull requests or commits) and generates updated or additional tests as needed.
- **Worker Services**: User-owned services that execute tests in response to temporal workflows, enabling secure access to permission-gated resources and internal systems.

In the default Docker Compose setup, the following components provide a fully functional Rocketship environment:

1. **rocketship_api**

   - **Role**: The core backend service that orchestrates temporal workflows, manages test definitions, and exposes APIs for the UI and CLI.
   - **Implementation**: A combination of TypeScript/Go services using the Temporal SDK for reliable workflow execution.

2. **rocketship_temporal**

   - **Role**: Manages workflow orchestration, ensuring reliable test execution and handling retries, timeouts, and state management.
   - **Implementation**: Runs Temporal server components for workflow management.

3. **rocketship_worker**

   - **Role**: User-managed workers that execute tests within your secure environment, allowing access to internal resources like WAFs, AWS IAM, and other permission-gated systems.
   - **Implementation**: Lightweight services that connect to Temporal and execute test workflows within your infrastructure.

4. **rocketship_frontend**

   - **Role**: A React-based user interface for reviewing AI-generated tests, monitoring workflow execution, and configuring your testing environment.
   - **Implementation**: Serves static React files or a Next.js application, accessible at a specific port (e.g., http://localhost:3000).

5. **rocketship_db**

   - **Role**: A PostgreSQL database that stores test definitions, user configurations, and run results.
   - **Implementation**: Runs a standard Postgres container with data persistence.

6. **rocketship_vectorstore**

   - **Role**: A containerized instance of ChromaDB or another vector database, used to store and retrieve embeddings of your code and tests.
   - **Implementation**: Manages vector-based embeddings, ensuring quick semantic search for AI tasks.

7. **rocketship_llm (optional)**
   - **Role**: If you choose to run a local Large Language Model (like Code Llama), you can include a container that provides an LLM API endpoint.
   - **Implementation**: Runs an open-source model or interacts with a GPU-powered environment if available.

This architecture ensures your AI agents can generate tests while execution happens securely within your infrastructure through temporal workflows. The worker-based approach allows you to run tests that require access to internal resources or specific permissions, making it suitable for complex enterprise environments.

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
