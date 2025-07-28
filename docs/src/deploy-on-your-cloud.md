# Deploy Rocketship on Your Cloud

Choose the right deployment option for your needs. Rocketship can be deployed on your infrastructure with enterprise-grade features including authentication, HTTPS, team management, and monitoring.

## Deployment Options

### 🐳 Docker Deployment
**Best for: Development, testing, and small-to-medium production deployments**

Deploy Rocketship using Docker Compose with full enterprise features:
- ✅ Quick setup (30 minutes)
- ✅ Enterprise authentication (Auth0, Okta, Azure AD)
- ✅ HTTPS with SSL certificates
- ✅ Team-based access control
- ✅ Multi-stack isolation for parallel environments
- ✅ Perfect for teams without Kubernetes expertise

**[→ Docker Deployment Guide](./docker-deployment.md)**

### ⚓ Kubernetes Deployment  
**Best for: Large-scale production deployments**

Deploy Rocketship on Kubernetes using Helm charts for maximum scalability:
- ✅ Enterprise-grade high availability
- ✅ Automatic scaling and resource management
- ✅ Production monitoring with Prometheus
- ✅ Advanced networking and security
- ✅ Multi-region support
- ✅ Integration with existing Kubernetes infrastructure

**[→ Kubernetes Deployment Guide](./kubernetes-deployment.md)**

## Architecture Overview

Both deployment options provide the same core Rocketship architecture:

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│   Users/Teams   │────▶│  Rocketship  │────▶│  Test APIs  │
│  (OIDC Login)   │     │   Platform   │     │  & Browser  │
└─────────────────┘     └──────────────┘     └─────────────┘
                               │
┌─────────────────┐            │              ┌─────────────┐
│    CI/CD Bot    │────────────┘         ┌───▶│   Reports   │
│  (API Tokens)   │                      │    └─────────────┘
└─────────────────┘     ┌──────────────┐ │
                        │   Temporal   │─┘
                        │  (Workflow)  │
                        └──────────────┘
```

**Core Components:**
- **Engine**: gRPC API server that orchestrates test execution
- **Worker**: Executes tests using various plugins (HTTP, Browser, SQL, etc.)
- **Temporal**: Workflow orchestration and durability
- **PostgreSQL**: Stores authentication data and Temporal state
- **External OIDC**: Your existing identity provider

## Decision Matrix

| Feature | Docker | Kubernetes |
|---------|--------|------------|
| **Setup Time** | 30 minutes | 1-2 hours |
| **Kubernetes Knowledge** | Not required | Required |
| **Scalability** | Medium (1-50 concurrent tests) | High (100+ concurrent tests) |
| **High Availability** | Manual | Automatic |
| **Resource Management** | Basic | Advanced |
| **Monitoring** | Basic | Production-grade |
| **Multi-region** | Manual | Built-in |
| **Maintenance** | Simple | Moderate |

## Quick Comparison

### Choose Docker if you:
- Want to get started quickly (under 30 minutes)
- Have small to medium testing needs
- Prefer simple deployment and maintenance
- Don't have Kubernetes expertise
- Need development or testing environments

### Choose Kubernetes if you:
- Need large-scale production deployment
- Require high availability and automatic scaling
- Have existing Kubernetes infrastructure
- Need advanced monitoring and observability
- Plan to run 100+ concurrent tests
- Need multi-region deployment

## Enterprise Features (Both Options)

Both deployment options include complete enterprise capabilities:

### 🔐 Authentication & Security
- **Enterprise SSO**: Auth0, Okta, Azure AD, Google Workspace
- **PKCE OAuth2 flow** for enhanced security
- **Team-based RBAC** with granular permissions
- **API token management** for CI/CD integration

### 🏢 Team Management
- **Organization structure** with teams and roles
- **Repository management** with CODEOWNERS enforcement
- **User permissions** aligned with your org structure
- **Audit trail** for all actions

### 🔒 Production Security
- **HTTPS/TLS encryption** with custom certificates
- **Certificate management** (self-signed, Let's Encrypt, BYOC)
- **Network isolation** between environments
- **Secure token storage** with rotation capabilities

### 📊 Monitoring & Observability
- **Real-time workflow monitoring** via Temporal UI
- **Test execution metrics** and performance data
- **Service health monitoring** and alerting
- **Audit logs** for compliance and troubleshooting

## Next Steps

1. **Choose your deployment option** based on your needs
2. **Follow the detailed guide** for your chosen platform
3. **Configure authentication** with your identity provider
4. **Set up teams and permissions** for your organization
5. **Integrate with CI/CD** for automated testing

## Support and Resources

- **Documentation**: Complete guides for both deployment options
- **Examples**: Real-world test specifications and use cases
- **Command Reference**: Complete CLI documentation
- **Community**: GitHub discussions and issue tracking

Ready to deploy? Choose your path:

**[🐳 Docker Deployment →](./docker-deployment.md)** | **[⚓ Kubernetes Deployment →](./kubernetes-deployment.md)**
