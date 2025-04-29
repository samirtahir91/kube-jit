import axios from 'axios';
import { useEffect, useState } from 'react';
import { Tab, Button, Col, ToggleButton, Alert } from 'react-bootstrap'; // Import Alert
import RequestTable from '../requestTable/RequestTable';
import refreshLogo from '../../assets/refresh.svg';
import './ApproveTabPane.css';
import { Request } from '../../types';

type ApproveTabPaneProps = {
    userId: string;
    username: string;
    setLoadingInCard: (loading: boolean) => void;
};

const ApproveTabPane = ({ userId, username, setLoadingInCard }: ApproveTabPaneProps) => {
    const [pendingRequests, setPendingRequests] = useState<Request[]>([]);
    const [selectedRequests, setSelectedRequests] = useState<number[]>([]);
    const [variant, setVariant] = useState<'light' | 'dark'>('light');
    const [toggleVariantColour, setToggleVariant] = useState<'secondary' | 'dark'>('secondary');
    const [errorMessage, setErrorMessage] = useState('');

    const fetchPendingRequests = async () => {
        setLoadingInCard(true); // Start loading
        try {
            const response = await axios.get('http://localhost:8589/kube-jit-api/approvals', {
                withCredentials: true
            });
            setPendingRequests(response.data.pendingRequests);
            setErrorMessage(''); // Clear error message on success
        } catch (error) {
            console.error('Error fetching pending requests:', error);
            setErrorMessage('Error fetching pending requests. Please try again.');
        } finally {
            setLoadingInCard(false); // Stop loading
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
        setLoadingInCard(true); // Start loading
        try {
            const selectedRequestData = pendingRequests.filter(request => selectedRequests.includes(request.ID));
            await axios.post('http://localhost:8589/kube-jit-api/approve-reject', {
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
            setErrorMessage(''); // Clear error message on success
        } catch (error) {
            console.error('Error approving/rejecting requests:', error);
            setErrorMessage('Error approving/rejecting requests. Please try again.');
        } finally {
            setLoadingInCard(false); // Stop loading
        }
    };

    const toggleVariant = () => {
        setVariant(prevVariant => (prevVariant === 'light' ? 'dark' : 'light'));
        setToggleVariant(prevVariant => (prevVariant === 'secondary' ? 'dark' : 'secondary'));
    };

    useEffect(() => {
        fetchPendingRequests();
    }, []);

    return (
        <Tab.Pane eventKey="approve" className='text-start py-4'>
            {errorMessage && (
                <Alert variant="danger" className="mt-3">
                    {errorMessage}
                </Alert>
            )}
            <p>Approve or reject one or many access requests.
                <button className="refresh-button" onClick={fetchPendingRequests}>
                    <img
                    alt="Refresh"
                    src={refreshLogo}
                    width="20"
                    height="20"
                    className="me-2"
                    />
                </button>
            </p>
            {pendingRequests.length === 0 && (
                <p>No pending requests (hit refresh to check again).</p>
            )}
            {pendingRequests.length > 0 ? (
            <Col className="d-flex mt-5">
            <ToggleButton variant={toggleVariantColour} onClick={toggleVariant} id={'light-dark'} value={variant}>
                light/dark
            </ToggleButton>
            </Col>
            ): ''
            }
            {pendingRequests.length > 0 && (
                <>
                    <RequestTable
                        variant={variant}
                        requests={pendingRequests}
                        selectable={true}
                        selectedRequests={selectedRequests}
                        handleSelectRequest={handleSelectRequest}
                    />
                    <Button variant="success" onClick={() => handleSelected("Approved")} disabled={selectedRequests.length === 0}>
                        Approve Selected
                    </Button>
                    <Button className="mx-2" variant="danger" onClick={() => handleSelected("Rejected")} disabled={selectedRequests.length === 0}>
                        Reject Selected
                    </Button>
                </>
            )}
        </Tab.Pane>
    );
};

export default ApproveTabPane;
