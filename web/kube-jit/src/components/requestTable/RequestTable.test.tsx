import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import RequestTable from './RequestTable';
import { PendingRequest, Request } from '../../types';

if (!global.URL.createObjectURL) {
  global.URL.createObjectURL = vi.fn(() => 'blob:url');
}

const pendingRequests: PendingRequest[] = [
  {
    ID: 1,
    username: 'alice',
    users: ['alice@example.com'],
    clusterName: 'dev-cluster',
    namespaces: ['ns1', 'ns2'],
    roleName: 'edit',
    startDate: new Date('2025-05-10T10:00:00Z').toISOString(),
    endDate: new Date('2025-05-10T12:00:00Z').toISOString(),
    justification: 'Need access',
    CreatedAt: new Date('2025-05-09T09:00:00Z').toISOString(),
    status: 'Pending',
    userID: "101",
    groupIDs: ["201", "202"],
    approvedList: [],
  },
];

const historyRequests: Request[] = [
  {
      ID: 2,
      username: 'bob',
      users: ['bob@example.com'],
      clusterName: 'prod-cluster',
      namespaces: ['prod-ns'],
      roleName: 'view',
      startDate: new Date('2025-05-08T08:00:00Z').toISOString(),
      endDate: new Date('2025-05-08T10:00:00Z').toISOString(),
      justification: 'Audit',
      CreatedAt: new Date('2025-05-07T07:00:00Z').toISOString(),
      UpdatedAt: '',
      DeletedAt: null,
      status: 'Approved',
      approverIDs: [],
      approverNames: ['admin'],
      notes: 'All good',
      namespaceApprovals: [
          {
              namespace: 'prod-ns',
              groupName: 'prod-group',
              groupID: "301",
              approved: true,
              approverName: 'admin',
              approverID: "401",
          },
      ],
      userID: '101'
  },
];

const baseProps = {
  selectable: false,
  selectedRequests: [],
  handleSelectRequest: vi.fn(),
  variant: 'light' as const,
  setVariant: vi.fn(),
};

describe('RequestTable', () => {
  it('renders pending requests table and filters', () => {
    render(
      <RequestTable
        {...baseProps}
        mode="pending"
        requests={pendingRequests}
      />
    );
    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('dev-cluster')).toBeInTheDocument();
    expect(screen.getByText('edit')).toBeInTheDocument();
    expect(screen.getAllByPlaceholderText('Filter')[0]).toBeInTheDocument();

    // Filter by username (first filter input is for username)
    fireEvent.change(screen.getAllByPlaceholderText('Filter')[0], { target: { value: 'bob' } });
    expect(screen.queryByText('alice')).not.toBeInTheDocument();
  });

  it('renders history requests table and filters', () => {
    render(
      <RequestTable
        {...baseProps}
        mode="history"
        requests={historyRequests}
      />
    );
    expect(screen.getByText('bob')).toBeInTheDocument();
    expect(screen.getByText('prod-cluster')).toBeInTheDocument();
    expect(screen.getByText('view')).toBeInTheDocument();
    expect(screen.getAllByText('Approved').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Approved').some(el => el.tagName === 'TD')).toBe(true);
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('All good')).toBeInTheDocument();
    expect(screen.getAllByText('prod-ns').length).toBeGreaterThan(0);
    // or:
    // expect(screen.getAllByText('prod-ns').some(el => el.tagName === 'TD' || el.tagName === 'STRONG')).toBe(true);

    // Filter by cluster (cluster filter is after username/users filters)
    fireEvent.change(screen.getAllByPlaceholderText('Filter')[2], { target: { value: 'dev' } });
    expect(screen.queryByText('prod-cluster')).not.toBeInTheDocument();
  });

  it('calls setVariant when toggling dark mode', () => {
    render(
      <RequestTable
        {...baseProps}
        mode="pending"
        requests={pendingRequests}
      />
    );
    // The toggle is the first div with class "toggle-button"
    fireEvent.click(document.querySelector('.toggle-button')!);
    expect(baseProps.setVariant).toHaveBeenCalled();
  });

  it('calls exportToCSV when clicking export', () => {
    render(
      <RequestTable
        {...baseProps}
        mode="pending"
        requests={pendingRequests}
      />
    );
    // Mock URL.createObjectURL and link.click
    const urlSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:url');
    const appendSpy = vi.spyOn(document.body, 'appendChild');
    const removeSpy = vi.spyOn(document.body, 'removeChild');
    fireEvent.click(screen.getByText(/export to csv/i));
    expect(urlSpy).toHaveBeenCalled();
    expect(appendSpy).toHaveBeenCalled();
    expect(removeSpy).toHaveBeenCalled();
    urlSpy.mockRestore();
    appendSpy.mockRestore();
    removeSpy.mockRestore();
  });
});