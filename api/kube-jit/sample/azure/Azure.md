1. Update Azure AD App Registration
Go to the Azure Portal.
Register a new application under Azure Active Directory > App Registrations.
Configure the Redirect URI to match your frontend URL (e.g., http://localhost:5173/).
Generate a client secret and note the Client ID and Client Secret.

2. Test the Integration
Restart your backend and frontend.
Verify that the Azure AD login button appears and works as expected.
Ensure that user data is fetched and stored correctly in the session.