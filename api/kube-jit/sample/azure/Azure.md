## Azure Oauth
1. Update Azure AD App Registration
Go to the Azure Portal.
Register a new application under Azure Active Directory > App Registrations.
Configure the Redirect URI to match your frontend URL (e.g., http://localhost:5173/).
Generate a client secret and note the Client ID and Client Secret.

Azure Permissions:

To fetch a user's groups from Azure AD, you need the Directory.Read.All permission in your Azure AD app registration. This permission allows your app to read group memberships for users in the directory.
Ensure that this permission is granted and admin-consented in your Azure AD app registration.
- Directory.Read.All
- email
- openid
- profile
- User.Read

Grant admin consent

2. Test the Integration
Restart your backend and frontend.
Verify that the Azure AD login button appears and works as expected.
Ensure that user data is fetched and stored correctly in the session.

## AKS and Postgress authentication via Azure Managed Identity

### Supports AKS clusters configuredfor Microsoft Entra ID with Kubernetes RBAC ONLY!

Managed Identity is Azure's equivalent of GCP's Workload Identity, allowing your pods to securely authenticate to Azure resources without needing to manage credentials.

Key Concepts for Azure Managed Identity

Managed Identity:
- Azure provides System-Assigned and User-Assigned Managed Identities.
- These identities can be assigned to your AKS cluster or individual pods.

Azure AD Authentication:
- Azure resources like PostgreSQL and AKS clusters can be configured to use Azure Active Directory (AAD) for authentication.
- Your API pod can use its Managed Identity to obtain AAD tokens for accessing these resources.

Azure Workload Identity - https://learn.microsoft.com/en-us/azure/aks/workload-identity-deploy-cluster:
- Azure Workload Identity is the recommended approach for Kubernetes. It integrates Kubernetes Service Accounts (KSA) with Azure AD, allowing pods to authenticate as Managed Identities.

## Guide for single subscription/tenant below, for cross tenant see - https://learn.microsoft.com/en-us/azure/aks/workload-identity-cross-tenant


- Create resource group and cluster (with entra ID and K8s RBAC option)

Retrieve the OIDC issuer URL
```sh
CLUSTER_NAME=api
RESOURCE_GROUP=test

export AKS_OIDC_ISSUER="$(az aks show --name "${CLUSTER_NAME}" --resource-group "${RESOURCE_GROUP}" --query "oidcIssuerProfile.issuerUrl" --output tsv)"

echo $AKS_OIDC_ISSUER
By default, the issuer is set to use the base URL https://{region}.oic.prod-aks.azure.com/{tenant_id}/{uuid}, where the value for {region} matches the location to which the AKS cluster is deployed. The value {uuid} represents the OIDC key, which is a randomly generated guid for each cluster that is immutable.
```

Create a managed identity
```sh
export SUBSCRIPTION="$(az account show --query id --output tsv)"
export USER_ASSIGNED_IDENTITY_NAME="myIdentity$RANDOM_ID"
az identity create --name "${USER_ASSIGNED_IDENTITY_NAME}" --resource-group "${RESOURCE_GROUP}" --location "${LOCATION}" --subscription "${SUBSCRIPTION}"
```

output
```json
{
  "clientId": "c762c439-f269-47ba-ba1a-e958249f3633",
  "id": "/subscriptions/f55741e9-1730-4d6e-8624-bf7b932a6145/resourcegroups/test/providers/Microsoft.ManagedIdentity/userAssignedIdentities/myIdentity213a2767-5038-48e7-8f3d-17074157b47c",
  "location": "eastus",
  "name": "myIdentity213a2767-5038-48e7-8f3d-17074157b47c",
  "principalId": "8ea97dcb-a67f-438a-95b2-0de055bd797e",
  "resourceGroup": "test",
  "systemData": null,
  "tags": {},
  "tenantId": "b2d528a4-58f5-4aef-b021-041d1b5378f1",
  "type": "Microsoft.ManagedIdentity/userAssignedIdentities"
}
```

Next, create a variable for the managed identity's client ID.
```sh
export USER_ASSIGNED_CLIENT_ID="$(az identity show --resource-group "${RESOURCE_GROUP}" --name "${USER_ASSIGNED_IDENTITY_NAME}" --query 'clientId' --output tsv)"

echo $USER_ASSIGNED_CLIENT_ID
```

Use the $USER_ASSIGNED_CLIENT_ID in the annotation of the KSA, via helm value for `serviceAccount.annotations`
```yaml
serviceAccount:
  annotations:
    azure.workload.identity/client-id: <USER_ASSIGNED_CLIENT_ID>
```

Create the federated identity credential - make sure to set the SERVICE_ACCOUNT_NAMESPACE and SERVICE_ACCOUNT_NAME as per your Helm value and Namespace.
```sh
export SERVICE_ACCOUNT_NAMESPACE=kube-jit-api
export SERVICE_ACCOUNT_NAME=kube-jit-api
export FEDERATED_IDENTITY_CREDENTIAL_NAME="myFedIdentity-<RANDOM_ID>"

az identity federated-credential create --name ${FEDERATED_IDENTITY_CREDENTIAL_NAME} --identity-name "${USER_ASSIGNED_IDENTITY_NAME}" --resource-group "${RESOURCE_GROUP}" --issuer "${AKS_OIDC_ISSUER}" --subject system:serviceaccount:"${SERVICE_ACCOUNT_NAMESPACE}":"${SERVICE_ACCOUNT_NAME}" --audience api://AzureADTokenExchange

az identity federated-credential list --resource-group "${RESOURCE_GROUP}" --identity-name "${FEDERATED_IDENTITY_CREDENTIAL_NAME}"

Confirm the subject
```

Create role assignment so the identity can auth to downstream clusers
```sh
export DOWNSTREAM_CLUSTER_NAME="controller"    # The name of the external aks cluster you want to add permissions for auth on
export DOWNSTREAM_RESOURCE_GROUP="test"        # The resource group of the external aks cluster
export SUBSCRIPTION_ID="f55741e9-1730-4d6e-8624-bf7b932a6145"                      # The subscription ID fo the external cluster

az role assignment create \
  --assignee ${USER_ASSIGNED_CLIENT_ID} \
  --role "Azure Kubernetes Service Cluster User Role" \
  --scope "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${DOWNSTREAM_RESOURCE_GROUP}/providers/Microsoft.ContainerService/managedClusters/${DOWNSTREAM_CLUSTER_NAME}"
```

Assign the AKS role to the user-assigned managed identity that you created previously. This step gives the managed identity permission to auth to AKS clusters:

REPLACE your Managed Identity's object principal ID in rbac-aks.yaml - yes its not the client ID - you can get it from portal or from output when you created the managed identity, json key is principalId.
```sh
kubectl config use-context <downstream controller cluster>

kubectl create -f ../../controller/kube-jit-operator/config/rbac/jitrequest_editor_role.yaml

kubectl create -f rbac-aks.yaml 
```
