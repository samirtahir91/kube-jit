import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import Profile from './Profile';
import axios from 'axios';

vi.mock('axios');
const mockedAxios = axios as unknown as { post: ReturnType<typeof vi.fn> };

const user = {
  id: 'test-id',
  name: 'Test User',
  avatar_url: '',
  provider: 'github',
  email: 'testuser@example.com',
};

const onSignOut = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
});

describe('Profile', () => {
  it('renders user name and Home link', () => {
    render(<Profile user={user} onSignOut={onSignOut} />);
    expect(screen.getByText('Test User')).toBeInTheDocument();
    expect(screen.getByText('Home')).toBeInTheDocument();
  });

  it('shows dropdown on mouse enter and hides on mouse leave', async () => {
    render(<Profile user={user} onSignOut={onSignOut} />);
    const dropdown = screen.getByTestId('user-dropdown') || screen.getByText('Test User').closest('div');
    fireEvent.mouseEnter(dropdown!);

    // Should be visible after mouse enter
    const dropdownMenu = document.querySelector('.dropdown-menu');
    expect(dropdownMenu).toBeTruthy();
    expect(dropdownMenu?.classList.contains('show')).toBe(true);

    fireEvent.mouseLeave(dropdown!);

    // Wait for the 'show' class to be removed (hidden)
    await waitFor(() => {
      expect(dropdownMenu?.classList.contains('show')).toBe(false);
    });
  });

  it('opens permissions modal and displays permissions', async () => {
    mockedAxios.post = vi.fn().mockResolvedValue({
      data: {
        isAdmin: true,
        isPlatformApprover: false,
        isApprover: true,
        approverGroups: [{ id: '1', name: 'Group A' }],
        adminGroups: [],
        platformApproverGroups: [],
      },
    });

    const { container } = render(<Profile user={user} onSignOut={onSignOut} />);
    fireEvent.mouseEnter(screen.getByTestId('user-dropdown'));

    // Click the dropdown item, not the modal title
    fireEvent.click(screen.getAllByText('My Permissions')[0]);

    // Wait for the modal title to appear using querySelector
    await waitFor(() => {
      const modalTitle = document.body.querySelector('.modal-title');
      expect(modalTitle).toBeInTheDocument();
      expect(modalTitle?.textContent).toBe('My Permissions');
      expect(screen.getByText('Is Admin:')).toBeInTheDocument();
      expect(screen.getByText('Is Platform Approver:')).toBeInTheDocument();
      expect(screen.getByText('No')).toBeInTheDocument();
      expect(screen.getByText('Is Approver:')).toBeInTheDocument();
      expect(screen.getByText('Approver Groups:')).toBeInTheDocument();
      expect(screen.getByText('Group A')).toBeInTheDocument();

      // Check both "Yes" values are present
      const yesElements = screen.getAllByText('Yes');
      expect(yesElements.length).toBe(2);
    });
  });

  it('shows error if permissions fetch fails', async () => {
    mockedAxios.post = vi.fn().mockRejectedValue(new Error('fail'));
    render(<Profile user={user} onSignOut={onSignOut} />);
    fireEvent.mouseEnter(screen.getByTestId('user-dropdown') || screen.getByText('Test User').closest('div')!);
    fireEvent.click(screen.getByText('My Permissions'));
    await waitFor(() => {
      expect(screen.getByText('Failed to fetch permissions')).toBeInTheDocument();
    });
  });

  it('calls onSignOut when Sign Out is clicked', () => {
    render(<Profile user={user} onSignOut={onSignOut} />);
    fireEvent.mouseEnter(screen.getByTestId('user-dropdown') || screen.getByText('Test User').closest('div')!);
    fireEvent.click(screen.getByText('Sign Out'));
    expect(onSignOut).toHaveBeenCalled();
  });
});