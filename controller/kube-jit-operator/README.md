# kube-jit-operator

A Kubernetes operator that creates short-lived rolebindings for users based on a JitRequest custom resource. It empowers self-service of Just-In-Time privileged access using Kubernetes RBAC.

## Description

### Key Features
- Uses a custom cluster scoped resource `JitRequest`, where the upstream API creates a JitRequest with:
  - reporter
  - clusterRole
  - additionalEmails (optional users to also add to role binding)
  - namespaces
  - justification
  - startTime
  - endTime

- The operator checks if the JitRequest's cluster role is allowed, from the `allowedClusterRoles` list defined in a `KubeJitConfig` custom resource (set by admins/operators) and then pre-approves the request.
- Calls back to the Kube JIT API with status updates as per the details as per the `JitRequest` spec.
- Requeues the `JitRequest` object for the defined `startTime`
- Creates the RoleBinding as requested, rejects and cleans-up `JitRequest` if validations fail.
- Deletes expired `JitRequests` and child objects (RoleBindings) at scheduled `endTime`.

### Logging and Debugging
- By default, logs are JSON formatted, and log level is set to info and error.
- Set `DEBUG_LOG` to `true` in the manager deployment environment variable for debug level logs.


### Additional Information
- The CRD includes extra data printed with `kubectl get jitreq`:
  - User
  - Cluster Role
  - Namespace
  - Start Time
  - End Time
- Events are recorded for:
  - Rejected `JitRequests`
  - Failure to create a RoleBinding for a `JitRequest`
  - Validation on allowed cluster roles

## Example `JitRequest` Resource

Here is an example of how a `JitRequest` resource looks:

```yaml
apiVersion: jit.kubejit.io/v1
kind: JitRequest
metadata:
  name: jitrequest-sample
spec:
  user: dev
  approver: your-boss
  userEmails:
    - "dev@dev.com"
    - "dev3@dev.com"
  requestorEmail: dev@dev.com
  namespaces: 
    - foo
    - bar
  startTime: 2025-01-18T11:48:10Z
  endTime: 2025-01-18T11:51:10Z
  clusterRole: edit
  ticketID: "123"
  callbackUrl: https://kube-jit-api@dev.com/k8s-callback
```

Above the jiraFields are mapped to the customFields in the `KubeJitConfig`:
```yaml
apiVersion: jit.kubejit.io/v1
kind: KubeJitConfig
metadata:
  name: kube-jit-operator-default
spec:
  allowedClusterRoles:
    - admin
    - edit
  namespaceAllowedRegex: ".*"
```

## Getting Started

### Prerequisites
- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To deploy with Helm using public Docker image
A helm chart is generated using `make helm`.
- Edit the `values.yaml` as required.
```sh
cd charts/kube-jit-operator
helm upgrade --install -n kube-jit-operator-system <release_name> . --create-namespace
```
- You can use the latest public image on DockerHub - `samirtahir91076/kube-jit-operator:latest`
  - See [tags](https://hub.docker.com/r/samirtahir91076/kube-jit-operator/tags) 
- Deploy the chart with Helm.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/kube-jit-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/kube-jit-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

### Integration Testing
- Tests will be run against a real cluster, i.e. Kind or Minikube
```sh
make kind-create # optional to use a kind cluster

make test-config

make test-cache

make test
```

### E2E Testing (ToDo - Currently blocked by Kind cluster setup issues. Do not run yet.)
- Tests will be run against a Kind cluster
```sh
# If on MAC
export TEST_OS="mac"  # MAC OS only

make test-e2e
```

**Run the controller in the foreground for testing:**
```sh
export KUBE_JIT_OPERATOR_CONFIG_PATH=/tmp/jit-test/
export OPERATOR_NAMESPACE=default
# run
make run
```

**Generate coverage html report:**
```sh
go tool cover -html=cover.out -o coverage.html
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/kube-jit-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/kube-jit-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

