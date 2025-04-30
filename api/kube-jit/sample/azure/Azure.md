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