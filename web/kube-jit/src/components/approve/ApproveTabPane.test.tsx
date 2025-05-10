import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import axios from 'axios';
import ApproveTabPane from './ApproveTabPane';
import { PendingRequest } from '../../types';
import config from '../../config/config';

// Mock axios
vi.mock('axios');
const mockedAxios = axios as unknown as {
    get: ReturnType<typeof vi.fn>;
    post: ReturnType<typeof vi.fn>;
};

// Mock RequestTable
vi.mock('../requestTable/RequestTable', () => ({
    default: ({ requests, selectedRequests, handleSelectRequest }: {
        requests: PendingRequest[],
        selectedRequests: number[],
        handleSelectRequest: (id: number) => void
    }) => (
        <div data-testid="request-table">
            {requests.map(req => (
                <div key={req.ID} data-testid={`request-${req.ID}`}>
                    <span>{req.username}</span>
                    <input
                        type="checkbox"
                        data-testid={`checkbox-${req.ID}`}
                        checked={selectedRequests.includes(req.ID)}
                        onChange={() => handleSelectRequest(req.ID)}
                    />
                </div>
            ))}
        </div>
    ),
}));

// Mock react-bootstrap components used directly
vi.mock('react-bootstrap', async (importOriginal) => {
    const actual = await importOriginal<typeof import('react-bootstrap')>();

    // --- Mock for Modal and its sub-components ---
    const MockModal = ({ show, onHide, children, ...props }: {
        show: boolean,
        onHide: () => void,
        children: React.ReactNode,
        [key: string]: any
    }) =>
        show ? (
            <div data-testid="mock-modal" {...props}>
                {children}
                {/* This button is just for the mock's internal structure if needed,
                    tests usually interact with buttons rendered as part of 'children' (e.g., in Modal.Footer)
                    or a specific close button if the component uses Modal.Header's closeButton prop. */}
            </div>
        ) : null;

    (MockModal as any).Header = ({ children, closeButton, ...props }: { children: React.ReactNode, closeButton?: boolean, [key: string]: any }) => (
        <div data-testid="mock-modal-header" {...props}>
            {children}
            {closeButton && <button data-testid="mock-modal-header-close-button" onClick={() => {
                // In a real scenario, this would call the onHide from the parent Modal.
                // For testing, if onHide needs to be triggered by this, the parent mock modal
                // would need to pass its onHide function down or be structured differently.
                // However, tests usually trigger onHide via other means (e.g., clicking a "Cancel" button).
            }}>×</button>}
        </div>
    );
    (MockModal as any).Title = ({ children, ...props }: { children: React.ReactNode, [key: string]: any }) => <div data-testid="mock-modal-title" {...props}>{children}</div>;
    (MockModal as any).Body = ({ children, ...props }: { children: React.ReactNode, [key: string]: any }) => <div data-testid="mock-modal-body" {...props}>{children}</div>;
    (MockModal as any).Footer = ({ children, ...props }: { children: React.ReactNode, [key: string]: any }) => <div data-testid="mock-modal-footer" {...props}>{children}</div>;

    // --- Mock for Tab and Tab.Pane ---
    // Tab itself might not be rendered if only Tab.Pane is used directly from it.
    // If Tab is used as <Tab><Tab.Pane /></Tab>, then MockTab needs to render children.
    const MockTabContainer = ({ children, ...props }: any) => <div {...props}>{children}</div>; // A simple container for Tab
    (MockTabContainer as any).Pane = ({ children, eventKey, ...props }: { eventKey: string, children: React.ReactNode, [key: string]: any }) => (
        <div data-testid={`mock-tab-pane-${eventKey}`} {...props}>
            {children}
        </div>
    );

    // --- Mock for Button ---
    const MockButton = ({ children, onClick, variant, disabled, ...props }: {
        children: React.ReactNode,
        onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void,
        variant?: string,
        disabled?: boolean,
        [key: string]: any
    }) => (
        <button onClick={onClick} data-variant={variant} disabled={disabled} {...props}>
            {children}
        </button>
    );

    return {
        ...actual, // Spread actual to keep any unmocked parts
        Modal: MockModal,
        Tab: MockTabContainer, // Use the container that has Pane
        Button: MockButton,
    };
});


const mockPendingRequests: PendingRequest[] = [
    {
        ID: 1,
        username: 'user.one',
        users: ['user.one@example.com'],
        clusterName: 'cluster-alpha',
        namespaces: ['ns-a', 'ns-b'],
        roleName: 'view',
        startDate: new Date('2025-05-10T10:00:00Z').toISOString(),
        endDate: new Date('2025-05-10T12:00:00Z').toISOString(),
        justification: 'Need access for task A',
        CreatedAt: new Date('2025-05-10T09:00:00Z').toISOString(),
        status: 'Pending',
        userID: 'user1',
        groupIDs: [],
        approvedList: [],
    },
    {
        ID: 2,
        username: 'user.two',
        users: ['user.two@example.com'],
        clusterName: 'cluster-beta',
        namespaces: ['ns-c'],
        roleName: 'edit',
        startDate: new Date('2025-05-11T14:00:00Z').toISOString(),
        endDate: new Date('2025-05-11T16:00:00Z').toISOString(),
        justification: 'Need access for task B',
        CreatedAt: new Date('2025-05-11T13:00:00Z').toISOString(),
        status: 'Pending',
        userID: 'user2',
        groupIDs: [],
        approvedList: [],
    },
];

const baseProps = {
    userId: 'approver123',
    username: 'approver.admin',
    setLoadingInCard: vi.fn(),
};

// Helper to find modal buttons
const getModalButton = (name: RegExp) => {
    const modal = screen.getByTestId('mock-modal');
    return within(modal).getByRole('button', { name });
};


describe('ApproveTabPane', () => {
    beforeEach(() => {
        vi.clearAllMocks(); // Clears mock call history etc.
        mockedAxios.get.mockResolvedValue({ data: { pendingRequests: mockPendingRequests } });
        mockedAxios.post.mockResolvedValue({ data: {} });
    });

    afterEach(() => {
        vi.useRealTimers(); // Ensure real timers are restored globally after each test
    });

    it('fetches pending requests on initial render and displays them', async () => {
        render(<ApproveTabPane {...baseProps} />);
        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true);
        await waitFor(() => {
            expect(mockedAxios.get).toHaveBeenCalledWith(`${config.apiBaseUrl}/kube-jit-api/approvals`, { withCredentials: true });
        });
        await waitFor(() => {
            expect(screen.getByTestId('request-table')).toBeInTheDocument();
            expect(screen.getByText('user.one')).toBeInTheDocument();
            expect(screen.getByText('user.two')).toBeInTheDocument();
        });
        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false);
    });

    it('displays "No pending requests" message when API returns empty list', async () => {
        mockedAxios.get.mockResolvedValue({ data: { pendingRequests: [] } });
        render(<ApproveTabPane {...baseProps} />);
        await waitFor(() => {
            expect(screen.getByText('No pending requests (hit refresh to check again).')).toBeInTheDocument();
        });
        expect(screen.queryByTestId('request-table')).not.toBeInTheDocument();
    });

    it('handles error when fetching pending requests', async () => {
        mockedAxios.get.mockRejectedValue(new Error('Network fetch error'));
        render(<ApproveTabPane {...baseProps} />);
        await waitFor(() => {
            expect(screen.getByText('Error fetching pending requests. Please try again.')).toBeInTheDocument();
        });
        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false);
    });

    it('allows selecting and deselecting requests', async () => {
        render(<ApproveTabPane {...baseProps} />);
        await waitFor(() => expect(screen.getByTestId('request-table')).toBeInTheDocument());

        const checkbox1 = screen.getByTestId('checkbox-1') as HTMLInputElement;
        const checkbox2 = screen.getByTestId('checkbox-2') as HTMLInputElement;
        const approveButton = screen.getByRole('button', { name: /Approve/i });
        const rejectButton = screen.getByRole('button', { name: /Reject/i });

        expect(approveButton).toBeDisabled();
        expect(rejectButton).toBeDisabled();

        fireEvent.click(checkbox1);
        expect(checkbox1.checked).toBe(true);
        expect(approveButton).not.toBeDisabled();
        expect(rejectButton).not.toBeDisabled();

        fireEvent.click(checkbox2);
        expect(checkbox2.checked).toBe(true);

        fireEvent.click(checkbox1);
        expect(checkbox1.checked).toBe(false);
        expect(approveButton).not.toBeDisabled(); // Still one selected
        expect(rejectButton).not.toBeDisabled();

        fireEvent.click(checkbox2);
        expect(checkbox2.checked).toBe(false);
        expect(approveButton).toBeDisabled();
        expect(rejectButton).toBeDisabled();
    });

    it('handles approving selected requests', async () => {
        // Using real timers for this test
        render(<ApproveTabPane {...baseProps} />);
        await screen.findByTestId('request-table'); 

        fireEvent.click(screen.getByTestId('checkbox-1'));
        fireEvent.click(screen.getByRole('button', { name: /Approve/i }));

        await screen.findByTestId('mock-modal'); 
        expect(screen.getByText('Confirm Approval')).toBeInTheDocument();
        expect(within(screen.getByTestId('mock-modal')).getByText(/Request ID: 1/i)).toBeInTheDocument();

        fireEvent.click(getModalButton(/Confirm/i));

        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true);
        
        await waitFor(() => {
            expect(mockedAxios.post).toHaveBeenCalledWith(
                `${config.apiBaseUrl}/kube-jit-api/approve-reject`,
                {
                    requests: [mockPendingRequests[0]],
                    approverID: baseProps.userId,
                    approverName: baseProps.username,
                    status: 'Approved',
                },
                { withCredentials: true }
            );
        });

        await screen.findByText('Request(s) approved successfully.');
        expect(screen.queryByText('user.one')).not.toBeInTheDocument(); // Request removed
        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false);

        expect(screen.getByText('Request(s) approved successfully.')).toBeInTheDocument();
    });

    it('handles rejecting selected requests', async () => {
        render(<ApproveTabPane {...baseProps} />);
        await waitFor(() => expect(screen.getByTestId('request-table')).toBeInTheDocument());

        fireEvent.click(screen.getByTestId('checkbox-2'));
        fireEvent.click(screen.getByRole('button', { name: /Reject/i }));

        await waitFor(() => expect(screen.getByTestId('mock-modal')).toBeInTheDocument());
        expect(screen.getByText('Confirm Rejection')).toBeInTheDocument();

        fireEvent.click(getModalButton(/Confirm/i));

        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true);
        await waitFor(() => {
            expect(mockedAxios.post).toHaveBeenCalledWith(
                `${config.apiBaseUrl}/kube-jit-api/approve-reject`,
                {
                    requests: [mockPendingRequests[1]],
                    approverID: baseProps.userId,
                    approverName: baseProps.username,
                    status: 'Rejected',
                },
                { withCredentials: true }
            );
        });
        await waitFor(() => {
            expect(screen.getByText('Request(s) rejected successfully.')).toBeInTheDocument();
            expect(screen.queryByText('user.two')).not.toBeInTheDocument(); // Request removed
        });
        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false);
    });

    it('handles error during approve/reject API call with specific message', async () => {
        // Using real timers for this test initially
        mockedAxios.post.mockRejectedValue({ response: { data: { error: 'API specific error message' } } });
        render(<ApproveTabPane {...baseProps} />);
        await screen.findByTestId('request-table');

        fireEvent.click(screen.getByTestId('checkbox-1'));
        fireEvent.click(screen.getByRole('button', { name: /Approve/i }));
        await screen.findByTestId('mock-modal');
        fireEvent.click(getModalButton(/Confirm/i));

        // The error message should appear
        await screen.findByText('API specific error message');
        expect(screen.getByText('user.one')).toBeInTheDocument(); // Request not removed
    });

    it('handles generic error during approve/reject API call', async () => {
        mockedAxios.post.mockRejectedValue(new Error('Network failure'));
        render(<ApproveTabPane {...baseProps} />);
        await waitFor(() => expect(screen.getByTestId('request-table')).toBeInTheDocument());

        fireEvent.click(screen.getByTestId('checkbox-1'));
        fireEvent.click(screen.getByRole('button', { name: /Approve/i }));
        await waitFor(() => expect(screen.getByTestId('mock-modal')).toBeInTheDocument());
        fireEvent.click(getModalButton(/Confirm/i));

        await waitFor(() => {
            expect(screen.getByText('Network failure')).toBeInTheDocument();
        });
    });

    it('allows dismissing error and success messages', async () => {
        mockedAxios.get.mockRejectedValue(new Error('Initial fetch error'));
        render(<ApproveTabPane {...baseProps} />);

        let errorMessageContainer: HTMLElement | null = null;
        await waitFor(() => {
            const errorText = screen.getByText('Error fetching pending requests. Please try again.');
            errorMessageContainer = errorText.closest('.error-message');
            expect(errorMessageContainer).toBeInTheDocument();
        });
        if (errorMessageContainer) {
            fireEvent.click(within(errorMessageContainer).getByRole('button', { name: /×/i }));
        }
        expect(screen.queryByText('Error fetching pending requests. Please try again.')).not.toBeInTheDocument();

        // Trigger success
        mockedAxios.get.mockResolvedValue({ data: { pendingRequests: mockPendingRequests } }); // For re-render
        mockedAxios.post.mockResolvedValue({ data: {} });
        // Need to re-render or simulate action leading to success
        fireEvent.click(screen.getByRole('button', { name: /Refresh/i })); // Re-fetch to get requests back
        await waitFor(() => expect(screen.getByTestId('request-table')).toBeInTheDocument());

        fireEvent.click(screen.getByTestId('checkbox-1'));
        fireEvent.click(screen.getByRole('button', { name: /Approve/i }));
        await waitFor(() => expect(screen.getByTestId('mock-modal')).toBeInTheDocument());
        fireEvent.click(getModalButton(/Confirm/i));


        let successMessageContainer: HTMLElement | null = null;
        await waitFor(() => {
            const successText = screen.getByText('Request(s) approved successfully.');
            successMessageContainer = successText.closest('.success-message');
            expect(successMessageContainer).toBeInTheDocument();
        });
        if (successMessageContainer) {
            fireEvent.click(within(successMessageContainer).getByRole('button', { name: /×/i }));
        }
        expect(screen.queryByText('Request(s) approved successfully.')).not.toBeInTheDocument();
    });

    it('handles refresh button click', async () => {
        render(<ApproveTabPane {...baseProps} />);
        await waitFor(() => expect(mockedAxios.get).toHaveBeenCalledTimes(1)); // Initial fetch

        const refreshButton = screen.getByRole('button', { name: /Refresh/i });
        fireEvent.click(refreshButton);

        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true); // For the refresh
        await waitFor(() => expect(mockedAxios.get).toHaveBeenCalledTimes(2)); // Initial + refresh
        expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false); // After refresh
    });

    it('shows loading state on refresh button', async () => {
        let resolveFetch: any;
        mockedAxios.get.mockImplementationOnce(() => new Promise(res => resolveFetch = res)); // First call hangs

        render(<ApproveTabPane {...baseProps} />);
        const refreshButton = screen.getByRole('button', { name: /Refresh/i });
        
        // Wait for initial fetch to start
        await waitFor(() => expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true));
        expect(refreshButton).toBeDisabled(); // isRefreshing should be true

        // Resolve the first fetch
        resolveFetch({ data: { pendingRequests: mockPendingRequests } });
        await waitFor(() => expect(refreshButton).not.toBeDisabled()); // isRefreshing false

        // Second fetch for refresh click
        mockedAxios.get.mockImplementationOnce(() => new Promise(res => resolveFetch = res));
        fireEvent.click(refreshButton);
        await waitFor(() => expect(refreshButton).toBeDisabled()); // isRefreshing true again

        resolveFetch({ data: { pendingRequests: [] } }); // Resolve second fetch
        await waitFor(() => expect(refreshButton).not.toBeDisabled());
    });

    it('displays selected request details in the confirmation modal', async () => {
        render(<ApproveTabPane {...baseProps} />);
        await waitFor(() => expect(screen.getByTestId('request-table')).toBeInTheDocument());
    
        fireEvent.click(screen.getByTestId('checkbox-1'));
        fireEvent.click(screen.getByTestId('checkbox-2'));
        fireEvent.click(screen.getByRole('button', { name: /Approve/i }));
    
        await waitFor(() => expect(screen.getByTestId('mock-modal')).toBeInTheDocument());
    
        const modal = screen.getByTestId('mock-modal');
        // Check for details of request 1
        expect(within(modal).getByText((content) => content.includes('Request ID: 1'))).toBeInTheDocument();
        // Check for details of request 2
        expect(within(modal).getByText((content) => content.includes('Request ID: 2'))).toBeInTheDocument();
    });
});