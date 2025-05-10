import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterAll, beforeAll } from 'vitest';
import Login from './Login';
import axios from 'axios';

vi.mock('axios');
const mockedAxios = axios as unknown as { get: ReturnType<typeof vi.fn> };

const baseProps = {
  onLoginSuccess: vi.fn(),
  setLoading: vi.fn(),
};

// Store the original window.location
const originalLocation = window.location;

beforeAll(() => {
  // Delete the existing location and assign a mock
  // @ts-ignore
  delete window.location;
  // @ts-ignore
  window.location = {
    href: '',
    search: '' as any,
    pathname: '/',
    assign: vi.fn((url: string) => {
      // @ts-ignore
      window.location.href = url;
    }),
  } as any;
});

afterAll(() => {
  // Restore the original window.location
  (window.location as any) = originalLocation;
});

describe('Login', () => {
  beforeEach(() => {
    vi.clearAllMocks(); // Clear all mocks, including axios and window.location.assign
    // Reset mock location state for each test
    window.location.href = '';
    window.location.search = '';
    window.location.pathname = '/';
    // If window.location.assign is a vi.fn(), clear its call history
    if (vi.isMockFunction(window.location.assign)) {
      (window.location.assign as ReturnType<typeof vi.fn>).mockClear();
    }

    mockedAxios.get = vi.fn().mockResolvedValue({
      data: {
        client_id: 'test-client-id',
        redirect_uri: 'http://localhost/callback',
        auth_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
        provider: 'github',
      },
    });
  });

  it('renders GitHub login button when provider is github', async () => {
    render(<Login {...baseProps} />);
    expect(await screen.findByText('Log in with GitHub')).toBeInTheDocument();
  });

  it('renders Google login button when provider is google', async () => {
    mockedAxios.get = vi.fn().mockResolvedValue({
      data: {
        client_id: 'test-client-id',
        redirect_uri: 'http://localhost/callback',
        provider: 'google',
      },
    });
    render(<Login {...baseProps} />);
    expect(await screen.findByText('Sign in with Google')).toBeInTheDocument();
  });

  it('renders Microsoft login button when provider is azure', async () => {
    mockedAxios.get = vi.fn().mockResolvedValue({
      data: {
        client_id: 'test-client-id',
        redirect_uri: 'http://localhost/callback',
        auth_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
        provider: 'azure',
      },
    });
    render(<Login {...baseProps} />);
    expect(await screen.findByText('Log in with Microsoft')).toBeInTheDocument();
  });

  it('redirects to GitHub OAuth on button click', async () => {
    render(<Login {...baseProps} />);
    const btn = await screen.findByText('Log in with GitHub');
    fireEvent.click(btn);
    expect(window.location.href).toContain('github.com/login/oauth/authorize');
  });

  it('redirects to Google OAuth on button click', async () => {
    mockedAxios.get = vi.fn().mockResolvedValue({
      data: {
        client_id: 'test-client-id',
        redirect_uri: 'http://localhost/callback',
        provider: 'google',
      },
    });
    render(<Login {...baseProps} />);
    const btn = await screen.findByText('Sign in with Google');
    fireEvent.click(btn);
    expect(window.location.href).toContain('accounts.google.com/o/oauth2/auth');
  });

  it('redirects to Azure OAuth on button click', async () => {
    mockedAxios.get = vi.fn().mockResolvedValue({
      data: {
        client_id: 'test-client-id',
        redirect_uri: 'http://localhost/callback',
        auth_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
        provider: 'azure',
      },
    });
    render(<Login {...baseProps} />);
    const btn = await screen.findByText('Log in with Microsoft');
    fireEvent.click(btn);
    expect(window.location.href).toContain('login.microsoftonline.com');
  });

  it('handles GitHub OAuth callback and calls onLoginSuccess', async () => {
    // Simulate code and state in URL for the callback
    window.location.search = '?code=abc123&state=github';

    // Mock the callback API response
    mockedAxios.get = vi.fn((url: string) => {
      if (url.includes('/oauth/github/callback')) {
        return Promise.resolve({
          data: {
            userData: { id: '1', name: 'GitHub User' },
            expiresIn: 3600,
          },
        });
      }
      return Promise.resolve({ // Fallback for client_id
        data: { client_id: 'test-client-id', redirect_uri: 'http://localhost/callback', provider: 'github' },
      });
    });

    render(<Login {...baseProps} />);

    await waitFor(() => {
      expect(baseProps.setLoading).toHaveBeenCalledWith(true);
    });
    await waitFor(() => {
      expect(baseProps.onLoginSuccess).toHaveBeenCalledWith({
        userData: { id: '1', name: 'GitHub User' },
        expiresIn: 3600,
      });
    });
    await waitFor(() => {
      expect(baseProps.setLoading).toHaveBeenCalledWith(false);
    });
  });

  it('handles Google OAuth callback and calls onLoginSuccess', async () => {
    window.location.search = '?code=def456&state=google';

    mockedAxios.get = vi.fn((url: string) => {
      if (url.includes('/oauth/google/callback')) {
        return Promise.resolve({
          data: {
            userData: { id: '2', name: 'Google User' },
            expiresIn: 3600,
          },
        });
      }
      return Promise.resolve({ // Fallback for client_id
        data: { client_id: 'test-client-id', redirect_uri: 'http://localhost/callback', provider: 'google' },
      });
    });

    render(<Login {...baseProps} />);

    await waitFor(() => {
      expect(baseProps.setLoading).toHaveBeenCalledWith(true);
    });
    await waitFor(() => {
      expect(baseProps.onLoginSuccess).toHaveBeenCalledWith({
        userData: { id: '2', name: 'Google User' },
        expiresIn: 3600,
      });
    });
    await waitFor(() => {
      expect(baseProps.setLoading).toHaveBeenCalledWith(false);
    });
  });

  it('handles Azure OAuth callback and calls onLoginSuccess', async () => {
    window.location.search = '?code=ghi789&state=azure';

    mockedAxios.get = vi.fn((url: string) => {
      if (url.includes('/oauth/azure/callback')) {
        return Promise.resolve({
          data: {
            userData: { id: '3', name: 'Azure User' },
            expiresIn: 3600,
          },
        });
      }
      return Promise.resolve({ // Fallback for client_id
        data: { client_id: 'test-client-id', redirect_uri: 'http://localhost/callback', auth_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize', provider: 'azure' },
      });
    });

    render(<Login {...baseProps} />);

    await waitFor(() => {
      expect(baseProps.setLoading).toHaveBeenCalledWith(true);
    });
    await waitFor(() => {
      expect(baseProps.onLoginSuccess).toHaveBeenCalledWith({
        userData: { id: '3', name: 'Azure User' },
        expiresIn: 3600,
      });
    });
    await waitFor(() => {
      expect(baseProps.setLoading).toHaveBeenCalledWith(false);
    });
  });
});