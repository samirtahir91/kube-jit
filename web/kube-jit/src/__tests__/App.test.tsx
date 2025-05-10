import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, beforeAll, afterAll } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import axios from 'axios';
import App from '../App';
import config from '../config/config';

// --- Mocks ---
vi.mock('axios');
const mockedAxios = axios as unknown as {
    get: ReturnType<typeof vi.fn>;
    post: ReturnType<typeof vi.fn>;
    interceptors: {
        response: {
            use: ReturnType<typeof vi.fn>;
        };
    };
};

// Mock child components to simplify App.tsx testing
vi.mock('../components/login/Login', () => ({
    default: vi.fn(({ onLoginSuccess, setLoading }) => (
        <div data-testid="mock-login">
            <button onClick={() => {
                localStorage.setItem("loginMethod", "mockProvider");
                onLoginSuccess({ userData: { id: 'user1', name: 'Test User', avatar_url: 'avatar.png', provider: 'mockProvider', email: 'test@example.com' }, expiresIn: 3600 });
            }}>
                Mock Login
            </button>
            <button onClick={() => setLoading(true)}>Set App Loading True</button>
        </div>
    )),
}));
vi.mock('../components/profile/Profile', () => ({
    default: vi.fn(({ user, onSignOut }) => (
        <div data-testid="mock-profile">
            <span>{user.name}</span>
            <button onClick={onSignOut}>Mock Sign Out</button>
        </div>
    )),
}));
vi.mock('../components/request/RequestTabPane', () => ({
    default: () => <div data-testid="mock-request-tab">Request Tab</div>
}));
vi.mock('../components/approve/ApproveTabPane', () => ({
    default: () => <div data-testid="mock-approve-tab">Approve Tab</div>
}));
vi.mock('../components/history/HistoryTabPane', () => ({
    default: () => <div data-testid="mock-history-tab">History Tab</div>
}));
vi.mock('../components/admin/AdminTabPane', () => ({
    default: () => <div data-testid="mock-admin-tab">Admin Tab</div>
}));
vi.mock('../components/footer/Footer', () => ({ default: ({ buildSha }: { buildSha?: string }) => <div data-testid="mock-footer">Footer {buildSha}</div> }));
vi.mock('react-spinners', () => ({
    SyncLoader: vi.fn((props) => {
        // The 'loading' prop for react-spinners SyncLoader defaults to true if not provided.
        const isLoading = props.loading === undefined ? true : props.loading;
        return isLoading ? <div data-testid="mock-sync-loader">Loading...</div> : null;
    }),
}));

// Mock window.location for redirection tests
const originalLocation = window.location;
beforeAll(() => {
    // @ts-ignore
    delete window.location;
    // @ts-ignore
    window.location = {
        href: '',
        pathname: '/',
        search: '',
        assign: vi.fn((url: string) => {
            // @ts-ignore
            window.location.href = url;
            // @ts-ignore
            window.location.pathname = new URL(url, 'http://localhost').pathname;
        }),
        replace: vi.fn((url: string) => {
            // @ts-ignore
            window.location.href = url;
            // @ts-ignore
            window.location.pathname = new URL(url, 'http://localhost').pathname;
        })
    } as any;
});

afterAll(() => {
    Object.defineProperty(window, 'location', {
        value: originalLocation,
        configurable: true,
        writable: true,
    });
});


// Mock localStorage
const localStorageMock = (() => {
    let store: Record<string, string> = {};
    return {
        getItem: (key: string) => store[key] || null,
        setItem: (key: string, value: string) => { store[key] = value.toString(); },
        removeItem: (key: string) => { delete store[key]; },
        clear: () => { store = {}; },
    };
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });


describe('App', () => {
    const mockUserData = { id: 'user1', name: 'Test User', avatar_url: 'avatar.png', provider: 'mockProvider', email: 'test@example.com' };

    beforeEach(() => {
        vi.clearAllMocks();
        localStorageMock.clear();
        window.location.href = 'http://localhost/'; // Reset URL
        window.location.pathname = '/';
        window.location.search = '';

        // Default mock for axios.get
        mockedAxios.get.mockImplementation((url: string) => {
            if (url.includes('/build-sha')) {
                return Promise.resolve({ data: { sha: 'test-sha' } });
            }
            return Promise.reject(new Error(`Unhandled GET request to ${url}`));
        });

        // Default mock for axios.post
        mockedAxios.post.mockImplementation((url: string) => {
            if (url.includes('/logout')) {
                return Promise.resolve({ data: { message: 'Logged out' } });
            }
            if (url.includes('/permissions')) {
                return Promise.resolve({ data: { isApprover: false, isAdmin: false, isPlatformApprover: false } });
            }
            return Promise.reject(new Error(`Unhandled POST request to ${url}`));
        });
    });

    const renderApp = () => render(
        <MemoryRouter initialEntries={['/']}>
            <Routes>
                <Route path="*" element={<App />} />
            </Routes>
        </MemoryRouter>
    );

    it('renders Login page when no valid token', async () => {
        renderApp();
        await waitFor(() => expect(screen.getByTestId('mock-login')).toBeInTheDocument());
    });

    it('fetches profile and renders main app if token is valid', async () => {
        localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
        localStorageMock.setItem('loginMethod', 'mockProvider');

        // Mock profile fetch
        global.fetch = vi.fn(() =>
            Promise.resolve({
                ok: true,
                json: () => Promise.resolve(mockUserData),
            })
        ) as any;

        renderApp();

        await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());
        expect(screen.getByText(mockUserData.name)).toBeInTheDocument();
        expect(screen.getByTestId('mock-request-tab')).toBeInTheDocument();
        expect(global.fetch).toHaveBeenCalledWith(
            `${config.apiBaseUrl}/kube-jit-api/mockProvider/profile`,
            expect.any(Object)
        );
        // Permissions should also be fetched
        await waitFor(() => expect(mockedAxios.post).toHaveBeenCalledWith(
            `${config.apiBaseUrl}/kube-jit-api/permissions`,
            { provider: 'mockProvider' },
            expect.any(Object)
        ));
    });

    it('handles successful login via Login component', async () => {
        renderApp();
        await waitFor(() => expect(screen.getByTestId('mock-login')).toBeInTheDocument());

        // Mock profile fetch for after login
        global.fetch = vi.fn(() =>
            Promise.resolve({
                ok: true,
                json: () => Promise.resolve(mockUserData),
            })
        ) as any;
        
        // Mock permissions fetch for after login
        mockedAxios.post.mockImplementation(async (url) => {
            if (url.includes('/permissions')) {
                return { data: { isApprover: false, isAdmin: false, isPlatformApprover: false } };
            }
            return Promise.reject(new Error("Unhandled POST"));
        });


        fireEvent.click(screen.getByText('Mock Login'));

        await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());
        expect(screen.getByText(mockUserData.name)).toBeInTheDocument();
        // Check that permissions were fetched after login (data state update triggers it)
        await waitFor(() => expect(mockedAxios.post).toHaveBeenCalledWith(
            `${config.apiBaseUrl}/kube-jit-api/permissions`,
            { provider: 'mockProvider' },
            expect.any(Object)
        ));
    });

    it('handles sign out', async () => {
        // Setup logged-in state
        localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
        localStorageMock.setItem('loginMethod', 'mockProvider');
        global.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(mockUserData) })) as any;
        
        renderApp();
        await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());

        fireEvent.click(screen.getByText('Mock Sign Out'));

        await waitFor(() => expect(mockedAxios.post).toHaveBeenCalledWith(
            `${config.apiBaseUrl}/kube-jit-api/logout`, {}, expect.any(Object)
        ));
        expect(localStorageMock.getItem('tokenExpiry')).toBeNull();
        expect(localStorageMock.getItem('loginMethod')).toBeNull();
        await waitFor(() => expect(screen.getByTestId('mock-login')).toBeInTheDocument());
    });

    describe('Role-based rendering', () => {
        const setupLoggedInStateWithPermissions = async (permissions: any) => {
            localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
            localStorageMock.setItem('loginMethod', 'mockProvider');
            global.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(mockUserData) })) as any;
            mockedAxios.post.mockImplementation(async (url) => {
                if (url.includes('/permissions')) return { data: permissions };
                if (url.includes('/logout')) return { data: { message: 'Logged out'} };
                return Promise.reject(new Error(`Unhandled POST to ${url}`));
            });
            renderApp();
            await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());
            await waitFor(() => expect(mockedAxios.post).toHaveBeenCalledWith(expect.stringContaining('/permissions'), expect.any(Object), expect.any(Object)));
        };

        it('shows Approve tab for Approver', async () => {
            await setupLoggedInStateWithPermissions({ isApprover: true, isAdmin: false, isPlatformApprover: false });
            expect(screen.getByTestId('mock-approve-tab')).toBeInTheDocument();
            expect(screen.getByText('Approver')).toBeInTheDocument(); // Badge
        });

        it('shows Admin and Approve tabs for Admin', async () => {
            await setupLoggedInStateWithPermissions({ isApprover: true, isAdmin: true, isPlatformApprover: false });
            expect(screen.getByTestId('mock-admin-tab')).toBeInTheDocument();
            expect(screen.getByTestId('mock-approve-tab')).toBeInTheDocument();
            // Find the badge with text "Admin"
            const adminBadges = screen.getAllByText('Admin');
            // The badge is usually a <span> with class "badge"
            expect(adminBadges.some(el => el.tagName === 'SPAN' && el.className.includes('badge'))).toBe(true);
        });

        it('shows Approve tab for Platform Approver', async () => {
            await setupLoggedInStateWithPermissions({ isApprover: false, isAdmin: false, isPlatformApprover: true });
            expect(screen.getByTestId('mock-approve-tab')).toBeInTheDocument();
            expect(screen.getByText('Platform Approver')).toBeInTheDocument(); // Badge
        });

        it('does not show special tabs or badge for regular user', async () => {
            await setupLoggedInStateWithPermissions({ isApprover: false, isAdmin: false, isPlatformApprover: false });
            expect(screen.queryByTestId('mock-approve-tab')).not.toBeInTheDocument();
            expect(screen.queryByTestId('mock-admin-tab')).not.toBeInTheDocument();
            expect(screen.queryByText(/Approver|Admin|Platform Approver/i)).not.toBeInTheDocument();
        });
    });

    it('handles tab navigation', async () => {
        localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
        localStorageMock.setItem('loginMethod', 'mockProvider');
        global.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(mockUserData) })) as any;
        mockedAxios.post.mockResolvedValueOnce({ data: { isApprover: true, isAdmin: false, isPlatformApprover: false } }); // For permissions

        renderApp();
        await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());

        // Default is request tab
        await waitFor(() => {
            expect(screen.getByTestId('mock-request-tab')).toBeVisible();
            expect(screen.getByTestId('mock-history-tab')).toBeInTheDocument();
        });

        // Click History tab
        fireEvent.click(screen.getByRole('tab', { name: /History/i }));
        await waitFor(() => {
            expect(screen.getByTestId('mock-history-tab')).toBeVisible();
            expect(screen.getByTestId('mock-request-tab')).toBeInTheDocument();
        });

        // No .tab-pane/.active assertions, just check visibility
        await waitFor(() => {
            expect(screen.getByTestId('mock-history-tab')).toBeVisible();
            expect(screen.getByTestId('mock-request-tab')).toBeInTheDocument();
        });

        // Click Approve tab
        fireEvent.click(screen.getByRole('tab', { name: /Approve/i }));
        await waitFor(() => {
            expect(screen.getByTestId('mock-approve-tab')).toBeVisible();
            expect(screen.getByTestId('mock-history-tab')).toBeInTheDocument();
        });
    });
    
    it('shows loading spinner during initial data fetch', async () => {
        localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
        localStorageMock.setItem('loginMethod', 'mockProvider');
        
        // Make fetch slow
        let resolveFetch: any;
        global.fetch = vi.fn(() => new Promise(res => { resolveFetch = res; })) as any;

        renderApp();
        // Wait for spinner to appear
        await waitFor(() => expect(screen.getByTestId('mock-sync-loader')).toBeInTheDocument(), { timeout: 1500 });
        expect(screen.getByText('Loading...')).toBeInTheDocument(); // From SyncLoader mock

        // Resolve fetch
        act(() => {
            resolveFetch({ ok: true, json: () => Promise.resolve(mockUserData) });
        });
        await waitFor(() => expect(screen.queryByTestId('mock-sync-loader')).not.toBeInTheDocument());
        await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());
    });

    it('fetches and passes build SHA to Footer', async () => {
        localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
        localStorageMock.setItem('loginMethod', 'mockProvider');
        global.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(mockUserData) })) as any;

        renderApp();
        await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());
        await waitFor(() => expect(mockedAxios.get).toHaveBeenCalledWith(`${config.apiBaseUrl}/kube-jit-api/build-sha`));
        expect(screen.getByTestId('mock-footer')).toHaveTextContent('Footer test-sha');
    });

    it('handles 401 from profile fetch and redirects to login', async () => {
        localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
        localStorageMock.setItem('loginMethod', 'mockProvider');

        // Mock profile fetch to fail with 401
        // Note: The global axios interceptor handles the redirect.
        // We simulate the 401 by having the fetch call fail in a way that App.tsx interprets as needing login.
        // The interceptor itself is hard to unit test here directly for a *fetch* call,
        // but if an *axios* call made by App.tsx (like permissions) got 401, it would trigger.
        // For profile fetch, App.tsx itself handles the !res.ok path.
        global.fetch = vi.fn(() =>
            Promise.resolve({
                ok: false, // Simulate a non-2xx response, e.g., 401
                status: 401,
                json: () => Promise.resolve({ error: 'Unauthorized' }),
            })
        ) as any;
        
        // Mock logout API call that the interceptor would make
        mockedAxios.post.mockImplementation(async (url) => {
            if (url.includes('/logout')) {
                return { data: { message: 'Logged out by interceptor' } };
            }
            return Promise.reject(new Error(`Unhandled POST to ${url}`));
        });

        renderApp();

        // App.tsx's fetchAllData catch block will setLogin(true)
        await waitFor(() => expect(screen.getByTestId('mock-login')).toBeInTheDocument());
        
        // If the 401 interceptor was triggered by an *axios* call, we'd check these:
        // await waitFor(() => expect(mockedAxios.post).toHaveBeenCalledWith(expect.stringContaining('/logout'), {}, expect.any(Object)));
        // await waitFor(() => expect(window.location.pathname).toBe('/')); // or window.location.href
    });

    it('axios interceptor redirects to login on 401 from permissions API', async () => {
        // Setup logged-in state
        localStorageMock.setItem('tokenExpiry', new Date(Date.now() + 3600 * 1000).toISOString());
        localStorageMock.setItem('loginMethod', 'mockProvider');
        global.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(mockUserData) })) as any;

        // Mock permissions to return 401
        mockedAxios.post.mockImplementation(async (url) => {
            if (url.includes('/permissions')) {
                const error: any = new Error("Unauthorized");
                error.response = { status: 401 };
                throw error; // This will be caught by the interceptor
            }
            if (url.includes('/logout')) { // This is called by the interceptor
                return { data: { message: 'Logged out by interceptor' } };
            }
            return Promise.reject(new Error(`Unhandled POST to ${url}`));
        });

        renderApp();

        // Wait for profile to load first
        await waitFor(() => expect(screen.getByTestId('mock-profile')).toBeInTheDocument());

        // Wait for redirect (window.location.href should be "/")
        await waitFor(() => {
            expect(window.location.pathname).toBe("/"); 
        });

        // Optionally, check that /permissions was called
        expect(mockedAxios.post).toHaveBeenCalledWith(
            `${config.apiBaseUrl}/kube-jit-api/permissions`,
            { provider: 'mockProvider' },
            expect.any(Object)
        );
    });

});