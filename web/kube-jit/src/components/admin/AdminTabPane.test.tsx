import { render, screen, fireEvent, waitFor, act, within } from '@testing-library/react'; // Add 'within' here
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import axios from 'axios';
import AdminTabPane from './AdminTabPane';
import config from '../../config/config';

// Mock axios
vi.mock('axios');
const mockedAxios = axios as unknown as {
    post: ReturnType<typeof vi.fn>;
};

// Helper to get modal buttons (assuming react-bootstrap structure)
const getModalButton = (name: RegExp) => screen.getByRole('button', { name });

// Utility: advances timers and retries the callback until it passes or times out
async function waitForWithTimers<T>(cb: () => T | Promise<T>, step = 50, max = 5000) {
    const start = Date.now();
    let lastErr;
    while (Date.now() - start < max) {
        try {
            return await cb();
        } catch (err) {
            lastErr = err;
            await act(async () => {
                vi.advanceTimersByTime(step);
            });
        }
    }
    throw lastErr;
}

describe('AdminTabPane', () => {
    const mockSetLoadingInCard = vi.fn();

    beforeEach(() => {
        mockSetLoadingInCard.mockClear();
        mockedAxios.post.mockReset();
    });

    afterEach(() => {
        vi.useRealTimers(); // Ensure real timers are restored after each test
    });

    it('renders initial state correctly', () => {
        render(<AdminTabPane setLoadingInCard={mockSetLoadingInCard} />);
        expect(screen.getByText('Admin Actions')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /Clean Expired Non-Approved Requests/i })).toBeInTheDocument();
        expect(screen.queryByText(/Confirm Cleanup/i)).not.toBeInTheDocument(); // Modal not visible
        expect(screen.queryByRole('alert')).not.toBeInTheDocument(); // No success/error messages
    });

    it('shows and hides confirmation modal', async () => {
        render(<AdminTabPane setLoadingInCard={mockSetLoadingInCard} />);
        const cleanButton = screen.getByRole('button', { name: /Clean Expired Non-Approved Requests/i });

        fireEvent.click(cleanButton);
        expect(await screen.findByText('Confirm Cleanup')).toBeInTheDocument();
        expect(screen.getByText(/Are you sure you want to delete all expired, non-approved requests/i)).toBeInTheDocument();

        fireEvent.click(getModalButton(/Cancel/i));
        await waitFor(() => expect(screen.queryByText('Confirm Cleanup')).not.toBeInTheDocument());
        expect(mockedAxios.post).not.toHaveBeenCalled();
    });

    it('handles successful cleanup of expired requests', async () => {
        vi.useFakeTimers();
        const mockResponse = { data: { message: 'Cleanup successful', deleted: 5 } };
        mockedAxios.post.mockResolvedValue(mockResponse);

        render(<AdminTabPane setLoadingInCard={mockSetLoadingInCard} />);
        fireEvent.click(screen.getByRole('button', { name: /Clean Expired Non-Approved Requests/i }));

        await waitForWithTimers(() => expect(screen.getByText('Confirm Cleanup')).toBeInTheDocument());
        fireEvent.click(getModalButton(/Yes, Clean Up/i));
        expect(mockSetLoadingInCard).toHaveBeenCalledWith(true);

        await waitForWithTimers(() =>
            expect(mockedAxios.post).toHaveBeenCalledWith(
                `${config.apiBaseUrl}/kube-jit-api/admin/clean-expired`,
                {},
                { withCredentials: true }
            )
        );
        await waitForWithTimers(() =>
            expect(screen.queryByText('Confirm Cleanup')).not.toBeInTheDocument()
        );
        await waitForWithTimers(() =>
            expect(screen.getByText('Cleanup successful. Deleted: 5')).toBeInTheDocument()
        );
        expect(mockSetLoadingInCard).toHaveBeenCalledWith(false);

        // Test auto-dismissal
        await act(async () => {
            vi.advanceTimersByTime(4000);
        });
        await waitForWithTimers(() =>
            expect(screen.queryByText('Cleanup successful. Deleted: 5')).not.toBeInTheDocument()
        );
    });

    it('handles failed cleanup of expired requests', async () => {
        vi.useFakeTimers();
        mockedAxios.post.mockRejectedValue(new Error('API Error'));

        render(<AdminTabPane setLoadingInCard={mockSetLoadingInCard} />);
        fireEvent.click(screen.getByRole('button', { name: /Clean Expired Non-Approved Requests/i }));

        await waitForWithTimers(() => expect(screen.getByText('Confirm Cleanup')).toBeInTheDocument());
        fireEvent.click(getModalButton(/Yes, Clean Up/i));
        expect(mockSetLoadingInCard).toHaveBeenCalledWith(true);

        await waitForWithTimers(() =>
            expect(mockedAxios.post).toHaveBeenCalled()
        );
        await waitForWithTimers(() =>
            expect(screen.queryByText('Confirm Cleanup')).not.toBeInTheDocument()
        );
        await waitForWithTimers(() =>
            expect(screen.getByText('Error cleaning expired requests.')).toBeInTheDocument()
        );
        expect(mockSetLoadingInCard).toHaveBeenCalledWith(false);

        // Test auto-dismissal
        await act(async () => {
            vi.advanceTimersByTime(5000);
        });
        await waitForWithTimers(() =>
            expect(screen.queryByText('Error cleaning expired requests.')).not.toBeInTheDocument()
        );
    });

    it('allows manual dismissal of success message', async () => {
        const mockResponse = { data: { message: 'Cleanup done', deleted: 1 } };
        mockedAxios.post.mockResolvedValue(mockResponse);

        render(<AdminTabPane setLoadingInCard={mockSetLoadingInCard} />);
        fireEvent.click(screen.getByRole('button', { name: /Clean Expired Non-Approved Requests/i }));
        await screen.findByText('Confirm Cleanup');
        fireEvent.click(getModalButton(/Yes, Clean Up/i));

        const successMessage = await screen.findByText('Cleanup done. Deleted: 1');
        expect(successMessage).toBeInTheDocument();
        
        // Find close button within the success message
        const closeButton = within(successMessage.parentElement!).getByRole('button', { name: /×/i });
        fireEvent.click(closeButton);
        expect(screen.queryByText('Cleanup done. Deleted: 1')).not.toBeInTheDocument();
    });

    it('allows manual dismissal of error message', async () => {
        mockedAxios.post.mockRejectedValue(new Error('API Error'));

        render(<AdminTabPane setLoadingInCard={mockSetLoadingInCard} />);
        fireEvent.click(screen.getByRole('button', { name: /Clean Expired Non-Approved Requests/i }));
        await screen.findByText('Confirm Cleanup');
        fireEvent.click(getModalButton(/Yes, Clean Up/i));

        const errorMessage = await screen.findByText('Error cleaning expired requests.');
        expect(errorMessage).toBeInTheDocument();

        // Find close button within the error message
        const closeButton = within(errorMessage.parentElement!).getByRole('button', { name: /×/i });
        fireEvent.click(closeButton);
        expect(screen.queryByText('Error cleaning expired requests.')).not.toBeInTheDocument();
    });
});