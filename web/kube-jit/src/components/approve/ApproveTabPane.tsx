import axios from 'axios';
import { useEffect, useState, useCallback } from 'react';
import { Tab, Modal, Button } from 'react-bootstrap';
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
    const [successMessage, setSuccessMessage] = useState('');
    const [isRefreshing, setIsRefreshing] = useState(false);
    const [showConfirmModal, setShowConfirmModal] = useState(false);
    const [confirmAction, setConfirmAction] = useState<'Approved' | 'Rejected' | null>(null);

    const fetchPendingRequests = useCallback(async () => {
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
    }, [setLoadingInCard]);

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
            setSuccessMessage(`Request(s) ${status === 'Approved' ? 'approved' : 'rejected'} successfully.`);
            setTimeout(() => {
                console.log('SUCCESS TIMEOUT FIRED - Clearing success message');
                setSuccessMessage('');
            }, 5000);
        } catch (error: unknown) {
            console.error('Error approving/rejecting requests:', error);
            let apiError = 'Error approving/rejecting requests. Please try again.';
            if (error && typeof error === 'object' && 'response' in error) {
                const err = error as { response?: { data?: { error?: string } }; message?: string };
                apiError = err.response?.data?.error || err.message || apiError;
            } else if (error instanceof Error) {
                apiError = error.message || apiError;
            }
            setErrorMessage(apiError);
            setTimeout(() => setErrorMessage(''), 5000);
        } finally {
            setLoadingInCard(false);
        }
    };

    useEffect(() => {
        fetchPendingRequests();
    }, [fetchPendingRequests]);

    return (
        <Tab.Pane eventKey="approve" className="text-start py-3">
            <div className="request-page-container">
            {(errorMessage || successMessage) && (
                <div className="sticky-message">
                    {errorMessage && (
                        <div className="error-message mt-3">
                            <i className="bi bi-exclamation-circle-fill me-2"></i>
                            {errorMessage}
                            <button className="error-message-close" onClick={() => setErrorMessage('')}>
                                &times;
                            </button>
                        </div>
                    )}
                    {successMessage && (
                        <div className="success-message mt-3">
                            <i className="bi bi-check-circle-fill me-2"></i>
                            {successMessage}
                            <button className="success-message-close" onClick={() => setSuccessMessage('')}>
                                &times;
                            </button>
                        </div>
                    )}
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
                        mode="pending"
                        selectable={true}
                        selectedRequests={selectedRequests}
                        handleSelectRequest={handleSelectRequest}
                    />
                    <div className="mt-3">
                        <button
                            className="approver-button approve"
                            disabled={selectedRequests.length === 0}
                            onClick={() => {
                                setConfirmAction('Approved');
                                setShowConfirmModal(true);
                            }}
                        >
                            <i className="bi bi-check-circle me-1"></i> Approve
                        </button>
                        <button
                            className="approver-button reject mx-2"
                            disabled={selectedRequests.length === 0}
                            onClick={() => {
                                setConfirmAction('Rejected');
                                setShowConfirmModal(true);
                            }}
                        >
                            <i className="bi bi-x-circle me-1"></i> Reject
                        </button>
                    </div>
                </>
            )}
            <Modal show={showConfirmModal} onHide={() => setShowConfirmModal(false)}>
                <Modal.Header closeButton>
                    <Modal.Title>
                        Confirm {confirmAction === 'Approved' ? 'Approval' : 'Rejection'}
                    </Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    <p>Are you sure you want to <b>{confirmAction === 'Approved' ? 'approve' : 'reject'}</b> the following request(s)?</p>
                    <ul>
                        {pendingRequests
                            .filter(request => selectedRequests.includes(request.ID))
                            .map(request => (
                                <li key={request.ID}>
                                    Request ID: {request.ID}
                                    {/* Add more details if needed */}
                                </li>
                            ))}
                    </ul>
                </Modal.Body>
                <Modal.Footer>
                    <Button variant="secondary" onClick={() => setShowConfirmModal(false)}>
                        Cancel
                    </Button>
                    <Button
                        variant={confirmAction === 'Approved' ? 'success' : 'danger'}
                        onClick={async () => {
                            setShowConfirmModal(false);
                            await handleSelected(confirmAction!);
                        }}
                    >
                        Confirm
                    </Button>
                </Modal.Footer>
            </Modal>
            </div>
        </Tab.Pane>
    );
};

export default ApproveTabPane;
