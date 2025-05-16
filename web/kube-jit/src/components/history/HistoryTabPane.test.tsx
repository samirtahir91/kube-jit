import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import axios from 'axios';
import HistoryTabPane from './HistoryTabPane';
import { Request } from '../../types';

// Mock axios
vi.mock('axios');
const mockedAxios = axios as unknown as { get: ReturnType<typeof vi.fn> };

// Mock RequestTable to simplify testing
vi.mock('../requestTable/RequestTable', () => ({
  default: ({ requests }: { requests: Request[] }) => (
    <div data-testid="request-table">
      {requests.map(req => <div key={req.ID} data-testid={`request-${req.ID}`}>{req.username}</div>)}
    </div>
  ),
}));

// Mock react-datepicker
vi.mock('react-datepicker', () => {
    const MockDatePicker = ({ selected, onChange, placeholderText, id }: any) => (
      <input
        id={id}
        data-testid={`datepicker-${id}`}
        type="text"
        value={selected ? selected.toISOString().split('T')[0] : ''}
        onChange={(e) => onChange(new Date(e.target.value))}
        placeholder={placeholderText}
      />
    );
    return { default: MockDatePicker };
  });


const mockRequests: Request[] = [
  {
    ID: 1,
    username: 'testuser1',
    users: ['testuser1@example.com'],
    clusterName: 'cluster-a',
    namespaces: ['ns1'],
    roleName: 'view',
    startDate: new Date('2025-01-01T10:00:00Z').toISOString(),
    endDate: new Date('2025-01-01T12:00:00Z').toISOString(),
    justification: 'Test request 1',
    CreatedAt: new Date('2025-01-01T09:00:00Z').toISOString(),
    UpdatedAt: '',
    DeletedAt: null,
    status: 'Approved',
    approverIDs: [],
    approverNames: ['admin'],
    notes: 'All good',
    namespaceApprovals: [],
    userID: 'user123',
  },
  {
    ID: 2,
    username: 'testuser2',
    users: ['testuser2@example.com'],
    clusterName: 'cluster-b',
    namespaces: ['ns2'],
    roleName: 'edit',
    startDate: new Date('2025-01-02T10:00:00Z').toISOString(),
    endDate: new Date('2025-01-02T12:00:00Z').toISOString(),
    justification: 'Test request 2',
    CreatedAt: new Date('2025-01-02T09:00:00Z').toISOString(),
    UpdatedAt: '',
    DeletedAt: null,
    status: 'Rejected',
    approverIDs: [],
    approverNames: ['admin'],
    notes: 'Not approved',
    namespaceApprovals: [],
    userID: 'user456',
  },
];

const baseProps = {
  isAdmin: false,
  isPlatformApprover: false,
  activeTab: 'history',
  originTab: 'request', // To trigger initial fetch
  userId: 'user123',
  setLoadingInCard: vi.fn(),
};

describe('HistoryTabPane', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedAxios.get.mockResolvedValue({ data: mockRequests });
  });

  it('renders nothing if activeTab is not "history"', () => {
    render(<HistoryTabPane {...baseProps} activeTab="pending" />);
    expect(screen.queryByText('Search access requests')).not.toBeInTheDocument();
  });

  it('renders search form and fetches requests on initial load if originTab is "request"', async () => {
    render(<HistoryTabPane {...baseProps} />);
    expect(screen.getByText('Search access requests')).toBeInTheDocument();
    await waitFor(() => {
      expect(mockedAxios.get).toHaveBeenCalledTimes(1);
      expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true);
    });
    await waitFor(() => {
        expect(screen.getByTestId('request-table')).toBeInTheDocument();
        expect(screen.getByText('testuser1')).toBeInTheDocument();
    });
    expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false);
  });

  it('does not fetch requests on initial load if originTab is not "request"', async () => {
    render(<HistoryTabPane {...baseProps} originTab="other" />);
    expect(screen.getByText('Search access requests')).toBeInTheDocument();
    // ensure no async calls are made, or simply check mock not called
    await expect(vi.waitFor(() => expect(mockedAxios.get).toHaveBeenCalled(), {timeout: 50})).rejects.toThrow();
    expect(mockedAxios.get).not.toHaveBeenCalled();
  });


  it('shows User ID and Username fields for admin', () => {
    render(<HistoryTabPane {...baseProps} isAdmin={true} originTab="other" />); // Changed originTab
    expect(screen.getByLabelText('User ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Username')).toBeInTheDocument();
  });

  it('shows User ID and Username fields for platform approver', () => {
    render(<HistoryTabPane {...baseProps} isPlatformApprover={true} originTab="other" />); // Changed originTab
    expect(screen.getByLabelText('User ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Username')).toBeInTheDocument();
  });

  it('does not show User ID and Username fields for non-admin/non-platform approver', () => {
    render(<HistoryTabPane {...baseProps} originTab="other" />); // Changed originTab
    expect(screen.queryByLabelText('User ID')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
  });

  it('handles search button click and fetches requests', async () => {
    render(<HistoryTabPane {...baseProps} originTab="other" />); // Prevent initial fetch
    mockedAxios.get.mockClear(); 
    baseProps.setLoadingInCard.mockClear();

    const searchButton = screen.getByRole('button', { name: 'Search' });
    fireEvent.click(searchButton);

    await waitFor(() => {
      expect(mockedAxios.get).toHaveBeenCalledTimes(1);
      expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true);
    });
    await waitFor(() => {
        expect(screen.getByTestId('request-table')).toBeInTheDocument();
    });
    expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false);
  });

  it('updates search parameters on input change for admin', async () => {
    render(<HistoryTabPane {...baseProps} isAdmin={true} originTab="other" />);
    mockedAxios.get.mockClear();

    fireEvent.change(screen.getByLabelText('User ID'), { target: { value: 'adminUser' } });
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'adminName' } });
    fireEvent.change(screen.getByLabelText(/Limit/), { target: { value: '50' } });
    
    const startDateInput = screen.getByTestId('datepicker-startDate');
    fireEvent.change(startDateInput, { target: { value: '2025-02-01' } });
    
    const endDateInput = screen.getByTestId('datepicker-endDate');
    fireEvent.change(endDateInput, { target: { value: '2025-02-10' } });


    fireEvent.click(screen.getByRole('button', { name: 'Search' }));

    await waitFor(() => {
      expect(mockedAxios.get).toHaveBeenCalledWith(
        expect.stringContaining('/history'),
        expect.objectContaining({
          params: {
            userID: 'adminUser',
            username: 'adminName',
            limit: 50,
            startDate: new Date('2025-02-01T00:00:00.000Z').toISOString(), 
            endDate: new Date('2025-02-10T00:00:00.000Z').toISOString(),
          },
          withCredentials: true,
        })
      );
    });
  });

  it('uses own userId for non-admin search', async () => {
    render(<HistoryTabPane {...baseProps} userId="currentUser123" originTab="other" />);
    mockedAxios.get.mockClear();

    fireEvent.click(screen.getByRole('button', { name: 'Search' }));

    await waitFor(() => {
      expect(mockedAxios.get).toHaveBeenCalledWith(
        expect.stringContaining('/history'),
        expect.objectContaining({
          params: expect.objectContaining({
            userID: 'currentUser123', 
            username: undefined, 
            limit: 1, 
          }),
        })
      );
    });
  });

  it('displays error message on fetch failure', async () => {
    mockedAxios.get.mockRejectedValue(new Error('Network Error'));
    render(<HistoryTabPane {...baseProps} originTab="other" />);
    baseProps.setLoadingInCard.mockClear();

    fireEvent.click(screen.getByRole('button', { name: 'Search' }));

    await waitFor(() => {
      expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(true);
    });
    
    let errorContainer: HTMLElement | null = null;
    await waitFor(() => {
      // Find the container directly, or the text and then its parent
      const errorMessageTextElement = screen.getByText('Error fetching requests. Please try again.');
      expect(errorMessageTextElement).toBeInTheDocument();
      errorContainer = errorMessageTextElement.closest('.error-message');
      expect(errorContainer).toBeInTheDocument(); // Ensure container is found
    });
    
    expect(baseProps.setLoadingInCard).toHaveBeenCalledWith(false);
    expect(screen.queryByTestId('request-table')).not.toBeInTheDocument();

    // Test closing the error message
    if (errorContainer) { // errorContainer is now HTMLElement | null
        fireEvent.click(within(errorContainer).getByRole('button', { name: /×/i }));
    } else {
        throw new Error("Error message container not found for close button test.");
    }
    expect(screen.queryByText('Error fetching requests. Please try again.')).not.toBeInTheDocument();
  });

  it('displays "No records found" message when search yields no results', async () => {
    mockedAxios.get.mockResolvedValue({ data: [] });
    render(<HistoryTabPane {...baseProps} originTab="other" />);

    fireEvent.click(screen.getByRole('button', { name: 'Search' }));

    let successContainer: HTMLElement | null = null;
    await waitFor(() => {
      const noRecordsMessageTextElement = screen.getByText('No records found.');
      expect(noRecordsMessageTextElement).toBeInTheDocument();
      successContainer = noRecordsMessageTextElement.closest('.success-message');
      expect(successContainer).toBeInTheDocument(); // Ensure container is found
    });
    expect(screen.queryByTestId('request-table')).not.toBeInTheDocument();

     // Test closing the "No records found" message
    if (successContainer) { // successContainer is now HTMLElement | null
        fireEvent.click(within(successContainer).getByRole('button', { name: /×/i }));
    } else {
        throw new Error("Success message container not found for close button test.");
    }
    expect(screen.queryByText('No records found.')).not.toBeInTheDocument();
  });

  it('respects max limit for admin', () => {
    render(<HistoryTabPane {...baseProps} isAdmin={true} originTab="other" />);
    const limitInput = screen.getByLabelText(/Limit \(max 100\)/) as HTMLInputElement;
    fireEvent.change(limitInput, { target: { value: '150' } });
    expect(limitInput.value).toBe('1'); 
    
    fireEvent.change(limitInput, { target: { value: '99' } });
    expect(limitInput.value).toBe('99');
  });

  it('respects max limit for non-admin', () => {
    render(<HistoryTabPane {...baseProps} originTab="other" />);
    const limitInput = screen.getByLabelText(/Limit \(max 20\)/) as HTMLInputElement;
    fireEvent.change(limitInput, { target: { value: '25' } });
    expect(limitInput.value).toBe('1'); 

    fireEvent.change(limitInput, { target: { value: '19' } });
    expect(limitInput.value).toBe('19');
  });
});