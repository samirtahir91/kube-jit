https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity

```sh
gcloud iam service-accounts create kube-jit-api \
    --project=sacred-entry-304212

# Grant the GSA viewer access to GKE clusters
gcloud projects add-iam-policy-binding sacred-entry-304212 \
    --member "serviceAccount:kube-jit-api@sacred-entry-304212.iam.gserviceaccount.com" \
    --role "roles/container.viewer"

# Grant the GSA permission to impersonate itself (if needed)
gcloud iam service-accounts add-iam-policy-binding kube-jit-api@sacred-entry-304212.iam.gserviceaccount.com \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:sacred-entry-304212.svc.id.goog[kube-jit-api/kube-jit-api]" \
    --project=sacred-entry-304212

# Annotate the Kubernetes ServiceAccount so that GKE sees the link between the service accounts
kubectl annotate serviceaccount kube-jit-api \
    --namespace  kube-jit-api \
    iam.gke.io/gcp-service-account=kube-jit-api@sacred-entry-304212.iam.gserviceaccount.com

# In addition to the IAM roles, you need to configure RBAC on the target GKE clusters to allow the GSA to perform actions on Kubernetes resources. For example, if your API needs to create jitRequest resources, you can configure RBAC like this:
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: jitrequest-creator
  namespace: default
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: jitrequest-manager
subjects:
  - kind: User
    name: "kube-jit-api@sacred-entry-304212.iam.gserviceaccount.com"
```