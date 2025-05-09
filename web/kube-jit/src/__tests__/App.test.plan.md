# App.tsx Test Plan

background:
i've shared all the components, you can see the login flow, fetch client id from api and login with provider google/azure or github, oauth flow redirects back with code query which sends to my api to get a access token from the oauth provider and returns an encrypted http only cookie (with access token) along with expiryDate to my react client. Then my client sets the tokenExpire in local storage, if there are any 401 erros to my api it is redirected to logout and clears cookies by hitting the logout api endpoint, likewise if tokenExpiry is past current time it will redirect to logout and clear cookies/localstorage.



## 1. Initial Loading State
- [ ] Renders loading spinner and footer while loading (`loading` is `true`).

## 2. Authenticated User States
- [ ] Renders main UI (profile, tabs, footer) when authenticated.
- [ ] Renders correct tabs:
  - [ ] Always: Request, History.
  - [ ] If approver/admin/platform approver: Approve tab.
  - [ ] If admin: Admin tab.
- [ ] Renders correct badge:
  - [ ] Admin, Platform Approver, Approver, or none.
- [ ] Passes correct props to child panes.
- [ ] Clicking "Sign Out" logs out and returns to login.

## 3. Unauthenticated State
- [x] Renders login page and footer when not logged in.
- [ ] On successful login, sets user data and shows main UI.

## 4. Fallback State
- [ ] If not loading, not logged in, and no user data: renders only the footer.

## 5. Side Effects
- [ ] Fetches build SHA and displays in footer.
- [ ] Fetches user profile and sets state on mount.
- [ ] Checks permissions and updates badge/tabs on user data change.

## 6. Error Handling
- [ ] Handles failed profile fetch by showing login.
- [ ] Handles failed permissions fetch gracefully.

## 7. API Integration
- [ ] Axios interceptor redirects to login on 401.

---

## Example Test (Implemented)

````tsx
// filepath: [App.test.tsx](http://_vscodecontentref_/0)
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from '../App';

describe('App', () => {
  it('shows login when not logged in', () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    expect(screen.getByText(/sign in/i)).toBeInTheDocument();
  });
});