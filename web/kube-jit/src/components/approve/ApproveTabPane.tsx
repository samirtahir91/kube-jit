import axios from 'axios';
import { useEffect, useState } from 'react';
import { Tab, Button, Alert } from 'react-bootstrap';
import RequestTable from '../requestTable/RequestTable';
import refreshLogo from '../../assets/refresh.svg';
import './ApproveTabPane.css';
import { PendingRequest } from '../../types';
import config from '../../config/config';

type ApproveTabPaneProps = {
    userId: string;
    username: string;
    setLoadingInCard: (loading: boolean) => void;
};

const ApproveTabPane = ({ userId, username, setLoadingInCard }: ApproveTabPaneProps) => {
    const [pendingRequests, setPendingRequests] = useState<PendingRequest[]>([]);
    const [selectedRequests, setSelectedRequests] = useState<number[]>([]);
    const [variant, setVariant] = useState<'light' | 'dark'>('light');
    const [errorMessage, setErrorMessage] = useState('');
    const [isRefreshing, setIsRefreshing] = useState(false);

    const fetchPendingRequests = async () => {
        setLoadingInCard(true);
        setIsRefreshing(true);
        try {
            const response = await axios.get(`${config.apiBaseUrl}/kube-jit-api/approvals`, {
                withCredentials: true
            });
            setPendingRequests(response.data.pendingRequests);
            setErrorMessage('');
        } catch (error) {
            console.error('Error fetching pending requests:', error);
            setErrorMessage('Error fetching pending requests. Please try again.');
        } finally {
            setLoadingInCard(false);
            setIsRefreshing(false);
        }
    };

    const handleSelectRequest = (id: number) => {
        setSelectedRequests(prevSelected =>
            prevSelected.includes(id)
                ? prevSelected.filter(requestId => requestId !== id)
                : [...prevSelected, id]
        );
    };

    const handleSelected = async (status: string) => {
        setLoadingInCard(true);
        try {
            const selectedRequestData = pendingRequests.filter(request => selectedRequests.includes(request.id));
            await axios.post(`${config.apiBaseUrl}/kube-jit-api/approve-reject`, {
                requests: selectedRequestData,
                approverID: userId,
                approverName: username,
                status: status
            }, {
                withCredentials: true
            });
            setPendingRequests(prevRequests =>
                prevRequests.filter(request => !selectedRequests.includes(request.id))
            );
            setSelectedRequests([]);
            setErrorMessage('');
        } catch (error) {
            console.error('Error approving/rejecting requests:', error);
            setErrorMessage('Error approving/rejecting requests. Please try again.');
        } finally {
            setLoadingInCard(false);
        }
    };

    useEffect(() => {
        fetchPendingRequests();
    }, []);

    return (
        <Tab.Pane eventKey="approve" className="text-start py-4">
            {errorMessage && (
                <Alert variant="danger" className="mt-3">
                    {errorMessage}
                </Alert>
            )}
            <div className="d-flex align-items-center">
                <p className="mb-0 me-1">Approve or reject one or many access requests.</p>
                <button
                    className={`refresh-button ${isRefreshing ? 'loading' : ''}`}
                    onClick={fetchPendingRequests}
                    disabled={isRefreshing}
                >
                    <img
                        alt="Refresh"
                        src={refreshLogo}
                        className="refresh-icon"
                    />
                </button>
            </div>
            {pendingRequests.length === 0 && (
                <p>No pending requests (hit refresh to check again).</p>
            )}
            {pendingRequests.length > 0 && (
                <>
                    <RequestTable
                        setVariant={setVariant}
                        variant={variant}
                        requests={pendingRequests}
                        mode="pending" // Specify mode for pending requests
                        selectable={true}
                        selectedRequests={selectedRequests}
                        handleSelectRequest={handleSelectRequest}
                    />
                    <Button
                        variant="success"
                        onClick={() => handleSelected('Approved')}
                        disabled={selectedRequests.length === 0}
                    >
                        Approve Selected
                    </Button>
                    <Button
                        className="mx-2"
                        variant="danger"
                        onClick={() => handleSelected('Rejected')}
                        disabled={selectedRequests.length === 0}
                    >
                        Reject Selected
                    </Button>
                </>
            )}
        </Tab.Pane>
    );
};

export default ApproveTabPane;
