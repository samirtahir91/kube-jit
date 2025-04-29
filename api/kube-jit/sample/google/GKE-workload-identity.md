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

## Cloud SQL Proxy
https://cloud.google.com/sql/docs/mysql/connect-kubernetes-engine#workload-identity

This will add permission for the api KSA to use GCP hosted Postgres (Cloud SQL)
```sh
gcloud projects add-iam-policy-binding sacred-entry-304212 \
    --member "serviceAccount:kube-jit-api@sacred-entry-304212.iam.gserviceaccount.com" \
    --role "roles/cloudsql.client"
```

https://cloud.google.com/sql/docs/mysql/connect-kubernetes-engine#run_the_in_a_sidecar_pattern


## Google Oauth 
There are 2 parts for Google Oauth
1. Login
2. Group fetch (backend api calls google admin directory apis via impersonating user)

### Login
- Create a new client in cloud auth, i.e. kube-jit - https://console.cloud.google.com/auth/clients?
- Add the authorised JavaScript origins - the url you are exposing the web deployment on (you may need to add the same url with and without the port if facing issues), i.e. for `http://localhost:5173`, add the 2 origins:
  - `http://kube-jit.samirtahir.dev:5173`
  - `http://kube-jit.samirtahir.dev`
- Add the same URL to Authorised redirect URIs (this will be the same value you configure in helm values for oauth.redirectUri)
  - `http://kube-jit.samirtahir.dev:5173`
- Create, save the client ID and Client Secret safely
  - Use the client ID in the helm value `oauth.clientID`
  - Create the `kube-jit-api-secrets` secret and add your client secret to it, note the secret key ref and set that as the helm value for `oauth.clientSecretKeyRef`

### Group fetch - via GSA
Make sure your GSA being used via Workload Identity has:
- Domain-wide delegation enabled in Google Workspace admin panel (https://admin.google.com/).
  - Add client ID of your GSA
  - Add authorised scope https://www.googleapis.com/auth/admin.directory.group.readonly
- IAM permission roles/iam.serviceAccountTokenCreator on itself.
```sh
gcloud iam service-accounts add-iam-policy-binding \
  kube-jit-api@sacred-entry-304212.iam.gserviceaccount.com \
  --member="serviceAccount:kube-jit-api@sacred-entry-304212.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountTokenCreator" \
  --project sacred-entry-304212
```