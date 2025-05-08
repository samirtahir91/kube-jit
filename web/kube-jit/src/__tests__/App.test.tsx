import { describe, it, beforeEach, vi, afterEach, expect, Mock } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import App from '../App';

// kube-jit-gh-teams/web/kube-jit/src/App.test.tsx

// Mock react-router-dom
vi.mock('react-router-dom', () => ({
    useNavigate: () => vi.fn(),
}));

// Mock axios
vi.mock('axios', () => ({
    default: {
        interceptors: { response: { use: vi.fn() } },
        post: vi.fn(),
        get: vi.fn(),
    },
}));

// Mock react-spinners
vi.mock('react-spinners', () => ({
    SyncLoader: (props: any) => <div data-testid="sync-loader" {...props} />,
}));

// Mock child components
vi.mock('./components/profile/Profile', () => ({
    default: ({ user, onSignOut }: any) => (
        <div>
            <span data-testid="profile">{user?.name}</span>
            <button onClick={onSignOut}>Sign Out</button>
        </div>
    ),
}));
vi.mock('./components/request/RequestTabPane', () => ({
    default: (props: any) => <div data-testid="request-tab" {...props} />,
}));
vi.mock('./components/approve/ApproveTabPane', () => ({
    default: (props: any) => <div data-testid="approve-tab" {...props} />,
}));
vi.mock('./components/history/HistoryTabPane', () => ({
    default: (props: any) => <div data-testid="history-tab" {...props} />,
}));
vi.mock('./components/admin/AdminTabPane', () => ({
    default: (props: any) => <div data-testid="admin-tab" {...props} />,
}));
vi.mock('./components/login/Login', () => ({
    default: ({ onLoginSuccess, setLoading }: any) => (
        <div>
            <button
                data-testid="login-btn"
                onClick={() => {
                    setLoading(false);
                    onLoginSuccess({
                        userData: { name: 'Test User', id: '123' },
                        expiresIn: 1000,
                    });
                }}
            >
                Login
            </button>
        </div>
    ),
}));
vi.mock('./components/footer/Footer', () => ({
    default: ({ buildSha }: any) => <footer data-testid="footer">{buildSha}</footer>,
}));

// Mock config
vi.mock('./config/config', () => ({
    default: { apiBaseUrl: 'http://mock-api' },
}));

// Mock localStorage
const localStorageMock = (() => {
    let store: Record<string, string> = {};
    return {
        getItem: (key: string) => store[key] || null,
        setItem: (key: string, value: string) => { store[key] = value; },
        removeItem: (key: string) => { delete store[key]; },
        clear: () => { store = {}; },
    };
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });

describe('App', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        window.localStorage.clear();
    });

    it('shows loader when loading', async () => {
        window.localStorage.setItem('tokenExpiry', new Date(Date.now() + 10000).toISOString());
        window.localStorage.setItem('loginMethod', 'github');
        // Mock fetch to never resolve
        global.fetch = vi.fn(() => new Promise(() => {})) as any;
        render(<App />);
        expect(screen.getByTestId('sync-loader')).toBeInTheDocument();
    });

    it('shows login when not logged in', async () => {
        render(<App />);
        expect(screen.getByTestId('login-btn')).toBeInTheDocument();
    });

    it('shows main UI after login', async () => {
        window.localStorage.setItem('tokenExpiry', new Date(Date.now() + 10000).toISOString());
        window.localStorage.setItem('loginMethod', 'github');
        // Mock fetch for profile
        global.fetch = vi.fn(() =>
            Promise.resolve({
                ok: true,
                json: () => Promise.resolve({ name: 'Test User', id: '123' }),
            })
        ) as any;

        const axios = (await import('axios')).default;
        (axios.get as Mock).mockResolvedValue({ data: { sha: 'abc123' } });
        (axios.post as Mock).mockResolvedValue({
            data: { isApprover: true, isAdmin: false, isPlatformApprover: false },
        });

        render(<App />);
        await waitFor(() => expect(screen.getByTestId('profile')).toHaveTextContent('Test User'));
        expect(screen.getByTestId('request-tab')).toBeInTheDocument();
        expect(screen.getByTestId('approve-tab')).toBeInTheDocument();
        expect(screen.getByTestId('history-tab')).toBeInTheDocument();
        expect(screen.queryByTestId('admin-tab')).not.toBeInTheDocument();
        expect(screen.getByText('Approver')).toBeInTheDocument();
        expect(screen.getByTestId('footer')).toHaveTextContent('abc123');
    });

    it('shows admin tab and badge if isAdmin', async () => {
        window.localStorage.setItem('tokenExpiry', new Date(Date.now() + 10000).toISOString());
        window.localStorage.setItem('loginMethod', 'github');
        global.fetch = vi.fn(() =>
            Promise.resolve({
                ok: true,
                json: () => Promise.resolve({ name: 'Admin User', id: '999' }),
            })
        ) as any;
        const axios = (await import('axios')).default;
        (axios.get as Mock).mockResolvedValue({ data: { sha: 'sha-admin' } });
        (axios.post as Mock).mockResolvedValue({
            data: { isApprover: false, isAdmin: true, isPlatformApprover: false },
        });

        render(<App />);
        await waitFor(() => expect(screen.getByTestId('profile')).toHaveTextContent('Admin User'));
        expect(screen.getByTestId('admin-tab')).toBeInTheDocument();
        expect(screen.getByText('Admin')).toBeInTheDocument();
    });

    it('shows platform approver badge', async () => {
        window.localStorage.setItem('tokenExpiry', new Date(Date.now() + 10000).toISOString());
        window.localStorage.setItem('loginMethod', 'github');
        global.fetch = vi.fn(() =>
            Promise.resolve({
                ok: true,
                json: () => Promise.resolve({ name: 'Plat User', id: '888' }),
            })
        ) as any;
        const axios = (await import('axios')).default;
        (axios.get as Mock).mockResolvedValue({ data: { sha: 'sha-plat' } });
        (axios.post as Mock).mockResolvedValue({
            data: { isApprover: false, isAdmin: false, isPlatformApprover: true },
        });

        render(<App />);
        await waitFor(() => expect(screen.getByTestId('profile')).toHaveTextContent('Plat User'));
        expect(screen.getByText('Platform Approver')).toBeInTheDocument();
    });

    it('handles sign out', async () => {
        window.localStorage.setItem('tokenExpiry', new Date(Date.now() + 10000).toISOString());
        window.localStorage.setItem('loginMethod', 'github');
        global.fetch = vi.fn(() =>
            Promise.resolve({
                ok: true,
                json: () => Promise.resolve({ name: 'Test User', id: '123' }),
            })
        ) as any;
        const axios = (await import('axios')).default;
        (axios.get as Mock).mockResolvedValue({ data: { sha: 'sha-logout' } });
        (axios.post as Mock).mockResolvedValue({
            data: { isApprover: true, isAdmin: false, isPlatformApprover: false },
        });

        render(<App />);
        await waitFor(() => expect(screen.getByTestId('profile')).toBeInTheDocument());
        fireEvent.click(screen.getByText('Sign Out'));
        await waitFor(() => expect(screen.getByTestId('login-btn')).toBeInTheDocument());
    });

    it('shows login if profile fetch fails', async () => {
        window.localStorage.setItem('tokenExpiry', new Date(Date.now() + 10000).toISOString());
        window.localStorage.setItem('loginMethod', 'github');
        global.fetch = vi.fn(() => Promise.resolve({ ok: false })) as any;
        render(<App />);
        await waitFor(() => expect(screen.getByTestId('login-btn')).toBeInTheDocument());
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });
});