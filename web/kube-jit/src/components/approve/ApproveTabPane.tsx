import axios from 'axios';
import { useEffect, useState } from 'react';
import { Tab } from 'react-bootstrap';
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
            const selectedRequestData = pendingRequests.filter(request => selectedRequests.includes(request.ID));
            await axios.post(`${config.apiBaseUrl}/kube-jit-api/approve-reject`, {
                requests: selectedRequestData,
                approverID: userId,
                approverName: username,
                status: status
            }, {
                withCredentials: true
            });
            setPendingRequests(prevRequests =>
                prevRequests.filter(request => !selectedRequests.includes(request.ID))
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
        <Tab.Pane eventKey="approve" className="text-start py-3">
            <div className="request-page-container">
            {errorMessage && (
                <div className="error-message mt-3">
                    <i className="bi bi-exclamation-circle-fill me-2"></i>
                    {errorMessage}
                    <button className="error-message-close" onClick={() => setErrorMessage('')}>
                        &times;
                    </button>
                </div>
            )}
            <div className="d-flex align-items-center">
                <div className="form-description mx-2 flex-grow-1">
                    <h2 className="form-title">Approve or reject access request</h2>
                    <p className="form-subtitle mb-0">
                        Approve or reject one or many access requests.<br />
                        <br /><strong>Select requests:</strong> Click on the checkbox next to each request to select it.<br />
                        <br /><strong>Approve/Reject:</strong> Click the "Approve" or "Reject" button to approve or reject the selected requests.<br />
                        <br />
                    </p>
                    <div className="d-flex align-items-center mt-2">
                        <p className="mb-0 me-2 form-subtitle"><strong>Refresh:</strong> Click the refresh button to check for new requests.</p>
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
                </div>
            </div>
            {pendingRequests && pendingRequests.length === 0 && (
                <p>No pending requests (hit refresh to check again).</p>
            )}
            {pendingRequests && pendingRequests.length > 0 && (
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
                    <div className="mt-3">
                        <button
                            className="approver-button approve"
                            disabled={selectedRequests.length === 0}
                            onClick={() => handleSelected('Approved')}
                        >
                            <i className="bi bi-check-circle me-1"></i> Approve
                        </button>
                        <button
                            className="approver-button reject mx-2"
                            disabled={selectedRequests.length === 0}
                            onClick={() => handleSelected('Rejected')}
                        >
                            <i className="bi bi-x-circle me-1"></i> Reject
                        </button>
                    </div>
                </>
            )}
            </div>
        </Tab.Pane>
    );
};

export default ApproveTabPane;
