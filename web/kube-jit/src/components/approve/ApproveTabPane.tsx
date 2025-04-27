import axios from 'axios';
import { useEffect, useState } from 'react';
import { Tab, Button, Col, ToggleButton } from 'react-bootstrap';
import RequestTable from '../requestTable/RequestTable';
import refreshLogo from '../../assets/refresh.svg';
import './ApproveTabPane.css';
import { Request } from '../../types'; // Import the shared Request type

type ApproveTabPaneProps = {
    userId: string;
    username: string;
};

const ApproveTabPane = ({ userId, username }: ApproveTabPaneProps) => {
    const [pendingRequests, setPendingRequests] = useState<Request[]>([]);
    const [selectedRequests, setSelectedRequests] = useState<number[]>([]);
    const [variant, setVariant] = useState<'light' | 'dark'>('light');
    const [toggleVariantColour, setToggleVariant] = useState<'secondary' | 'dark'>('secondary');

    const fetchPendingRequests = async () => {
        try {
            const response = await axios.get('/kube-jit-api/approvals', {
                withCredentials: true
            });
            setPendingRequests(response.data.pendingRequests);
        } catch (error) {
            console.error('Error fetching pending requests:', error);
        }
    };

    useEffect(() => {
        fetchPendingRequests();
    }, []);

    const handleSelectRequest = (id: number) => {
        setSelectedRequests(prevSelected =>
            prevSelected.includes(id)
                ? prevSelected.filter(requestId => requestId !== id)
                : [...prevSelected, id]
        );
    };

    const handleSelected = async (status: string) => {
        try {
            const selectedRequestData = pendingRequests.filter(request => selectedRequests.includes(request.ID));
            await axios.post('/kube-jit-api/approve-reject', {
                requests: selectedRequestData,
                approverID: userId,
                approverName: username,
                status: status
            }, {
                withCredentials: true
            });
            // Optionally, refresh the pending requests list
            setPendingRequests(prevRequests =>
                prevRequests.filter(request => !selectedRequests.includes(request.ID))
            );
            setSelectedRequests([]);
        } catch (error) {
            console.error('Error approving requests:', error);
        }
    };

    const toggleVariant = () => {
        setVariant(prevVariant => (prevVariant === 'light' ? 'dark' : 'light'));
        setToggleVariant(prevVariant => (prevVariant === 'secondary' ? 'dark' : 'secondary'));
    };

    return (
        <Tab.Pane eventKey="approve" className='text-start py-4'>
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
