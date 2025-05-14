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

## Getting Started

### Prerequisites

- Kubernetes cluster (v1.20+ recommended)
- Identity provider (GitHub, Microsoft or Google)
- Go 1.20+ for development
- [kubectl](https://kubernetes.io/docs/tasks/tools/) and cluster-admin access


### Usage

- **Request Access:** Users can request access via the web UI or API endpoint.
- **Approval:** Approvers receive notifications (e.g., via GitHub or email) and can approve or deny requests.
- **Access Granted:** Upon approval, the user receives temporary RBAC permissions in the target cluster.
- **Automatic Revocation:** Permissions are automatically revoked after the expiry time.

## Contributing

Contributions are welcome! Please open issues or pull requests for bug fixes, features, or documentation improvements.

## License

This project is licensed under the [Apache 2.0 License](LICENSE).

**kube-jit** — Secure, auditable, and developer-friendly Kubernetes access.