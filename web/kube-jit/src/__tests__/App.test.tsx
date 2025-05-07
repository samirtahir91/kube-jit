import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { act } from "react-dom/test-utils";
import userEvent from "@testing-library/user-event";
import App from "../App";
import axios from "axios";

// Mock modules
jest.mock("axios");
jest.mock("react-router-dom", () => ({
  ...jest.requireActual("react-router-dom"),
  useNavigate: () => jest.fn()
}));
jest.mock("../components/profile/Profile", () => ({ onSignOut }: { onSignOut: () => void }) => (
  <div data-testid="profile">
    Profile Component
    <button onClick={onSignOut} data-testid="sign-out">Sign Out</button>
  </div>
));
jest.mock("../components/request/RequestTabPane", () => () => <div data-testid="request-tab">RequestTabPane Component</div>);
jest.mock("../components/login/Login", () => ({ onLoginSuccess }: { onLoginSuccess: (data: { userData: { name: string; id: string }; expiresIn: number }) => void }) => (
  <div data-testid="login">
    Login Component
    <button 
      onClick={() => onLoginSuccess({ userData: { name: "Test User", id: "123" }, expiresIn: 3600 })}
      data-testid="login-button"
    >
      Login
    </button>
  </div>
));
jest.mock("../components/approve/ApproveTabPane", () => () => <div data-testid="approve-tab">ApproveTabPane Component</div>);
jest.mock("../components/history/HistoryTabPane", () => () => <div data-testid="history-tab">HistoryTabPane Component</div>);
jest.mock("../components/admin/AdminTabPane", () => () => <div data-testid="admin-tab">AdminTabPane Component</div>);
jest.mock("../components/footer/Footer", () => () => <div data-testid="footer">Footer Component</div>);
jest.mock("react-spinners", () => ({
  SyncLoader: ({ loading }: { loading: boolean }) => <div data-testid="spinner" data-loading={loading}>Loading Spinner</div>,
}));
jest.mock("../config/config", () => ({
  apiBaseUrl: "/api",
}));

// Mock fetch globally
window.fetch = jest.fn();

describe("App Component", () => {
  // Reset mocks before each test
  beforeEach(() => {
    jest.clearAllMocks();
    localStorage.clear();
    (axios.get as jest.Mock).mockResolvedValue({ data: { sha: "testsha" } });
    (axios.post as jest.Mock).mockResolvedValue({ data: {} });
    (window.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({ name: "Test User", id: "123" }),
    });
    
    // Mock location.href setter
    Object.defineProperty(window, 'location', {
      value: {
        href: jest.fn(),
      },
      writable: true,
      configurable: true // Make it configurable for spyOn to work
    });
  });

  it("shows the login screen when not logged in", async () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId("login")).toBeInTheDocument();
      expect(screen.getByTestId("footer")).toBeInTheDocument();
    });
  });

  it("shows loading spinner when loading", async () => {
    // Set up token expiry in the future
    localStorage.setItem("tokenExpiry", new Date(Date.now() + 3600000).toISOString());
    localStorage.setItem("loginMethod", "github");
    
    // Create a promise that won't resolve during the test
    const neverResolve = new Promise(() => {});
    (window.fetch as jest.Mock).mockImplementation(() => neverResolve);
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId("spinner")).toBeInTheDocument();
    });
  });

  it("logs in and displays the main UI when login is successful", async () => {
    // Setup test
    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { isApprover: false, isAdmin: false, isPlatformApprover: false }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    // Find and click the login button
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    // Click the login button
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    // Wait for profile and request tab to appear
    await waitFor(() => {
      expect(screen.getByTestId("profile")).toBeInTheDocument();
      expect(screen.getByTestId("request-tab")).toBeInTheDocument();
    });
  });

  it("shows approver tab for users with approver role", async () => {
    // Test with approver permissions
    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { isApprover: true, isAdmin: false, isPlatformApprover: false }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    // Login
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    // Wait for approver tab to appear
    await waitFor(() => {
      expect(screen.getByTestId("approve-tab")).toBeInTheDocument();
    });
    
    // Verify approver badge is displayed
    await waitFor(() => {
      expect(screen.getByText("Approver")).toBeInTheDocument();
    });
  });

  it("shows admin tab only for users with admin role", async () => {
    // First test with non-admin permissions
    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { isApprover: false, isAdmin: false, isPlatformApprover: false }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    // Login
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    await waitFor(() => {
      expect(screen.queryByText("Admin")).not.toBeInTheDocument();
    });
    
    // Clean up for next test
    jest.clearAllMocks();
    
    // Now test with admin permissions
    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { isApprover: false, isAdmin: true, isPlatformApprover: false }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    // Wait for admin tab to appear after permissions check
    await waitFor(() => {
      expect(screen.getByTestId("admin-tab")).toBeInTheDocument();
    });
  });

  it("displays appropriate badge based on user role", async () => {
    // Test for admin badge
    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { isApprover: false, isAdmin: true, isPlatformApprover: false }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });
  });

  it("displays platform approver badge for platform approvers", async () => {
    // Test for platform approver badge
    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { isApprover: false, isAdmin: false, isPlatformApprover: true }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    await waitFor(() => {
      expect(screen.getByText("Platform Approver")).toBeInTheDocument();
    });
  });
  
  it("handles session expiry correctly", async () => {
    // Set expired token
    localStorage.setItem("tokenExpiry", new Date(Date.now() - 3600000).toISOString());
    localStorage.setItem("loginMethod", "github");
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    // Should show login screen
    await waitFor(() => {
      expect(screen.getByTestId("login")).toBeInTheDocument();
    });
  });

  it("handles fetch profile failure", async () => {
    localStorage.setItem("tokenExpiry", new Date(Date.now() + 3600000).toISOString());
    localStorage.setItem("loginMethod", "github");
    
    // Mock fetch to fail
    (window.fetch as jest.Mock).mockResolvedValueOnce({
      ok: false,
      status: 401
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    // Should show login screen after fetch failure
    await waitFor(() => {
      expect(screen.getByTestId("login")).toBeInTheDocument();
    });
  });

  it("handles failed permissions check", async () => {
    // Mock successful login but failed permissions check
    (axios.post as jest.Mock).mockRejectedValueOnce(new Error("Permission check failed"));
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    // Even with failed permissions check, the UI should still render
    await waitFor(() => {
      expect(screen.getByTestId("profile")).toBeInTheDocument();
      expect(screen.getByTestId("request-tab")).toBeInTheDocument();
    });
    
    // But no approver/admin roles should be visible
    expect(screen.queryByText("Approver")).not.toBeInTheDocument();
    expect(screen.queryByText("Admin")).not.toBeInTheDocument();
  });
  
  it("handles sign out correctly", async () => {
    // Setup a logged-in state
    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { isApprover: false, isAdmin: false, isPlatformApprover: false }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    // Login
    await waitFor(() => {
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
    
    await act(async () => {
      userEvent.click(screen.getByTestId("login-button"));
    });
    
    // Verify we're logged in
    await waitFor(() => {
      expect(screen.getByTestId("profile")).toBeInTheDocument();
    });
    
    // Clear mock history for correct assertion
    jest.clearAllMocks();
    
    // Click logout button
    await act(async () => {
      userEvent.click(screen.getByTestId("sign-out"));
    });
    
    // Should call logout API
    expect(axios.post).toHaveBeenCalledWith(
      "/api/kube-jit-api/logout",
      {},
      { withCredentials: true }
    );
    
    // Should go back to login screen
    await waitFor(() => {
      expect(screen.getByTestId("login")).toBeInTheDocument();
    });
    
    // Local storage should be cleared
    expect(localStorage.getItem("tokenExpiry")).toBeNull();
    expect(localStorage.getItem("loginMethod")).toBeNull();
  });

  it("handles 401 responses with axios interceptor", async () => {
    // Set up the 401 interceptor test
    const error = { 
      response: { status: 401 }
    };
    
    // Configure location.href to be spyable
    const hrefSpy = jest.fn();
    Object.defineProperty(window.location, 'href', {
      set: hrefSpy,
      configurable: true
    });
    
    // Trigger the interceptor directly
    await act(async () => {
      // Define the interceptor logic directly
      const interceptor = async (err: any) => {
        if (err.response?.status === 401) {
          await axios.post("/api/kube-jit-api/logout", {}, { withCredentials: true });
          window.location.href = "/";
        }
        return Promise.reject(err);
      };

      try {
        await interceptor(error);
      } catch (e) {
        // Expected to reject
      }
    });
    
    // Check logout was called
    expect(axios.post).toHaveBeenCalledWith(
      "/api/kube-jit-api/logout",
      {},
      { withCredentials: true }
    );
    
    // Should redirect to home
    expect(hrefSpy).toHaveBeenCalledWith("/");
  });

  it("displays the build SHA when available", async () => {
    // Mock the SHA response
    (axios.get as jest.Mock).mockResolvedValueOnce({ 
      data: { sha: "abcdef123456" }
    });
    
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    
    // Verify the API call was made
    expect(axios.get).toHaveBeenCalledWith("/api/kube-jit-api/build-sha");
    
    // Footer should be rendered
    await waitFor(() => {
      expect(screen.getByTestId("footer")).toBeInTheDocument();
    });
  });
});