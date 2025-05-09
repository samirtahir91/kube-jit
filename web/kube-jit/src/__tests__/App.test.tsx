import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from '../App';

// Mock axios to control loading state
import { vi } from 'vitest';

vi.mock('axios', () => ({
  default: {
    interceptors: { response: { use: vi.fn() } },
    get: vi.fn(() => new Promise(() => {})), // Never resolves, keeps loading=true
    post: vi.fn(),
  },
}));

describe('App', () => {
  it('shows login when not logged in', () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    expect(screen.getByText(/sign in/i)).toBeInTheDocument();
  });

  it('shows loading spinner and footer while loading', () => {
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    // Spinner should be present
    expect(screen.getByTestId('sync-loader')).toBeInTheDocument();
    // Footer should be present
    expect(screen.getByTestId('footer')).toBeInTheDocument();
  });
});