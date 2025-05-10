import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import RequestTabPane from './RequestTabPane';
import axios from 'axios';

vi.mock('axios');
const mockedAxios = axios as unknown as { get: ReturnType<typeof vi.fn>, post: ReturnType<typeof vi.fn> };

vi.mock('react-datepicker', () => ({
  __esModule: true,
  default: (props: any) => (
    <input
      aria-label={props.id}
      value={props.selected ? props.selected.toISOString().slice(0, 16) : ''}
      onChange={e => props.onChange(new Date(e.target.value))}
      type="datetime-local"
    />
  ),
}));

const defaultProps = {
  setActiveTab: vi.fn(),
  setOriginTab: vi.fn(),
  setLoadingInCard: vi.fn(),
  userId: 'user-1',
  username: 'testuser',
};

beforeEach(() => {
  vi.clearAllMocks();
  mockedAxios.get = vi.fn().mockResolvedValue({
    data: {
      roles: [{ name: 'edit' }, { name: 'view' }],
      clusters: ['dev-cluster', 'prod-cluster'],
    },
  });
});

describe('RequestTabPane', () => {
  it('renders form fields', async () => {
    render(<RequestTabPane {...defaultProps} />);
    expect(await screen.findByText('Submit Access Request')).toBeInTheDocument();
    expect(screen.getByLabelText('User Emails')).toBeInTheDocument();
    expect(screen.getByLabelText('Cluster')).toBeInTheDocument();
    expect(screen.getByLabelText('Namespace(s)')).toBeInTheDocument();
    expect(screen.getByLabelText('Justification')).toBeInTheDocument();
    expect(screen.getByLabelText('Role')).toBeInTheDocument();
    expect(screen.getByLabelText('startDate')).toBeInTheDocument();
    expect(screen.getByLabelText('endDate')).toBeInTheDocument();
  });

  it('shows modal on submit and calls API on confirm', async () => {
    render(<RequestTabPane {...defaultProps} />);
    // Fill out required fields
    fireEvent.change(screen.getByPlaceholderText('Enter email address(es)'), { target: { value: 'user@example.com' } });
    fireEvent.keyDown(screen.getByPlaceholderText('Enter email address(es)'), { key: 'Enter', code: 'Enter' });

    fireEvent.change(screen.getByPlaceholderText('Enter namespace(s)'), { target: { value: 'ns1' } });
    fireEvent.keyDown(screen.getByPlaceholderText('Enter namespace(s)'), { key: 'Enter', code: 'Enter' });

    fireEvent.change(screen.getByPlaceholderText('Enter a reason or reference (max 100 chars)'), { target: { value: 'test justification' } });

    // Select cluster
    fireEvent.keyDown(screen.getByLabelText('Cluster'), { key: 'ArrowDown' });
    fireEvent.click(await screen.findByText('dev-cluster'));

    // Select role
    fireEvent.keyDown(screen.getByLabelText('Role'), { key: 'ArrowDown' });
    fireEvent.click(await screen.findByText('edit'));

    // Pick dates (simulate by setting value directly)
    fireEvent.change(screen.getByLabelText('startDate'), { target: { value: '2025-05-10T10:00' } });
    fireEvent.change(screen.getByLabelText('endDate'), { target: { value: '2025-05-10T12:00' } });

    // Submit form
    fireEvent.click(screen.getByRole('button', { name: /submit request/i }));

    // Modal should appear
    await waitFor(() => {
      const modalTitle = document.body.querySelector('.modal-title');
      expect(modalTitle).toBeInTheDocument();
      expect(modalTitle?.textContent).toBe('Confirm Request');
    });

    // Mock API response for submit
    mockedAxios.post = vi.fn().mockResolvedValue({ data: { message: 'Request submitted!' } });

    // Confirm in modal
    fireEvent.click(screen.getByRole('button', { name: /confirm/i }));

    await waitFor(() => {
      expect(mockedAxios.post).toHaveBeenCalled();
      expect(screen.getByText('Request submitted!')).toBeInTheDocument();
    });
  });
});