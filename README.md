# Kube JIT

Kube JIT is an open source solution for implementing secure, self-service, just-in-time (JIT) access to Kubernetes resources using RBAC, with flexible integration to multiple identity providers. Kube JIT supports Azure/Microsoft OAuth, Google OAuth, and GitHub OAuth (via GitHub Apps), leveraging groups or teams from these providers for namespace ownership and approval workflows on access requests.

Kube JIT enables organizations to reduce standing privileges and improve compliance by granting temporary, auditable access to Kubernetes namespaces or roles. Approval workflows are managed using your existing group or team structures in your chosen identity provider, making access management seamless and secure.

## Architecture

![Diagram](docs/diagrams/Kube-JIT.svg)

1. **User requests access** to a Kubernetes resource (namespace, role, etc.).
2. **Identity group/team** is used to determine group membership and route approval requests.
3. **Approvers** (e.g., team/group members) review and approve or deny requests.
4. **Temporary RBAC roles** are created in Kubernetes, granting access for a limited time.
5. **Automatic expiry** ensures permissions are revoked after the approved window.

## Features

- **Just-in-Time Access:** Grant temporary RBAC permissions to users only when needed, with automatic expiry and revocation.
- **Multi-Provider Integration:** Supports Azure/Microsoft OAuth, Google OAuth, and GitHub OAuth (via GitHub Apps).
- **Group/Team-Based Approval:** Leverages your identity provider’s groups or teams for namespace ownership and access approval workflows.
- **Self-Service Requests:** Users can request access via a web UI or API, reducing operational overhead.
- **Auditing & Compliance:** All access requests and grants are logged for auditability and compliance.
- **Kubernetes Native:** Works with standard Kubernetes RBAC and integrates seamlessly with your existing clusters.
- **Automatic Expiry:** Ensures that all granted permissions are automatically revoked after the approved time window.
- **Extensible:** Designed to support additional identity providers and custom approval workflows.
- **Secure by Design:** Minimizes standing privileges and enforces least-privilege access.

## Installation

### 1. Prerequisites

- Kubernetes cluster(s) (v1.20+ recommended)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) access to all clusters
- [Helm 3](https://helm.sh/docs/intro/install/) installed
- Identity provider (Azure/Microsoft, Google, or GitHub)
- Go 1.20+ for development (if building from source)
- Docker (for building images, if not using pre-built)

### 2. Deploy the API and Web UI (Management Cluster)

These components should be deployed **together on your management cluster** using the provided Helm charts and sample values.

The Oauth/Identitiy provider config is defined in the API chart, see the [values.yaml](./api/kube-jit/chart/kube-jit-api/values.yaml)

```sh
# Clone the repo and cd into it
git clone https://github.com/your-org/kube-jit-gh-teams.git
cd kube-jit-gh-teams

# (Optional) Build and load images if running locally
cd api/kube-jit
make docker-build
cd ../../web
make build

# Deploy API
cd api/kube-jit
# Create your values file, see sample folder for examples
helm install -n kube-jit-api kube-jit-api chart/kube-jit-api --create-namespace -f values.yaml

# Deploy Web UI
cd ../../web
# Create your values file, see sample folder for examples
helm install -n kube-jit-web kube-jit-web chart/kube-jit-web --create-namespace -f values.yaml
```

> [!TIP]
> You can customize your deployment using the sample values files in api/kube-jit/sample/ and web/sample/.


### 3. Deploy the Controller (Downstream Clusters)
```sh
# Switch context to your downstream cluster
kubectl config use-context <downstream-cluster-context>

# Deploy the controller using Helm
cd controller/kube-jit-operator
# Create your values file, see config/samples folder for examples
helm install -n kube-jit-operator kube-jit-operator charts/kube-jit-operator --create-namespace -f values.yaml
```


## Contributing

Contributions are welcome! Please open issues or pull requests for bug fixes, features, or documentation improvements.

## License

This project is licensed under the [Apache 2.0 License](LICENSE).

**kube-jit** — Secure, auditable, and developer-friendly Kubernetes access.