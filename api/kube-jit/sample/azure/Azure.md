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
Managed Identity is Azure's equivalent of GCP's Workload Identity, allowing your pods to securely authenticate to Azure resources without needing to manage credentials.

Key Concepts for Azure Managed Identity

Managed Identity:
- Azure provides System-Assigned and User-Assigned Managed Identities.
- These identities can be assigned to your AKS cluster or individual pods.

Azure AD Authentication:
- Azure resources like PostgreSQL and AKS clusters can be configured to use Azure Active Directory (AAD) for authentication.
- Your API pod can use its Managed Identity to obtain AAD tokens for accessing these resources.

Azure Workload Identity:
- Azure Workload Identity is the recommended approach for Kubernetes. It integrates Kubernetes Service Accounts (KSA) with Azure AD, allowing pods to authenticate as Managed Identities.