import { useEffect, useState, useCallback } from 'react';
import axios from 'axios';
import { Alert, Button, Form, Col, Row, ToggleButton } from 'react-bootstrap';
import DatePicker from 'react-datepicker';
import 'react-datepicker/dist/react-datepicker.css';
import './HistoryTabPane.css';
import RequestTable from '../requestTable/RequestTable';
import { Request } from '../../types'; // Import the shared Request type
import config from '../../config/config';

type HistoryTabPaneProps = {
    isAdmin: boolean;
    activeTab: string;
    originTab: string;
    userId: string;
    setLoadingInCard: (loading: boolean) => void;
};

const HistoryTabPane = ({ isAdmin, activeTab, originTab, userId, setLoadingInCard }: HistoryTabPaneProps) => {
    const [requests, setRequests] = useState<Request[]>([]);
    const [errorMessage, setErrorMessage] = useState('');
    const [limit, setLimit] = useState(1);
    const [startDate, setStartDate] = useState<Date | null>(null);
    const [endDate, setEndDate] = useState<Date | null>(null);
    const [hasSearched, setHasSearched] = useState(false);
    const [variant, setVariant] = useState<'light' | 'dark'>('light');
    const [toggleVariantColour, setToggleVariant] = useState<'secondary' | 'dark'>('secondary');

    const fetchRequests = useCallback(async (limit: number, startDate: Date | null, endDate: Date | null) => {
        setLoadingInCard(true); // Start loading
        try {
            const response = await axios.get(`${config.apiBaseUrl}/kube-jit-api/history`, {
                params: {
                    userID: userId,
                    limit: limit,
                    startDate: startDate ? startDate.toISOString() : undefined,
                    endDate: endDate ? endDate.toISOString() : undefined,
                },
                withCredentials: true
            });
            setRequests(response.data);
            setErrorMessage(''); // Clear error message on success
        } catch (error) {
            console.error('Error fetching requests:', error);
            setErrorMessage('Error fetching requests. Please try again.');
        } finally {
            setLoadingInCard(false); // Stop loading
        }
    }, [userId, setLoadingInCard]);

    useEffect(() => {
        if (activeTab === 'history' && originTab === 'request') {
            fetchRequests(limit, startDate, endDate);
        }
    }, [activeTab, originTab, userId, limit, startDate, endDate, fetchRequests]);

    const handleSearch = () => {
        setHasSearched(true);
        fetchRequests(limit, startDate, endDate);
    };

    if (activeTab !== 'history') {
        return null; // Do not render anything if the active tab is not 'history'
    }

    const toggleVariant = () => {
        setVariant(prevVariant => (prevVariant === 'light' ? 'dark' : 'light'));
        setToggleVariant(prevVariant => (prevVariant === 'secondary' ? 'dark' : 'secondary'));
    };

    return (
        <>
            {errorMessage && (
                <Alert variant="danger" className="mt-3">
                    {errorMessage}
                </Alert>
            )}
            <Row className="mt-4">
                <Col md={3}>
                    <Form.Group controlId="startDate" className="text-start">
                        <Form.Label>Start Date</Form.Label>
                        <DatePicker
                            selected={startDate}
                            onChange={(date: Date | null) => {
                                if (date) {
                                    setStartDate(date);
                                }
                            }}
                            showTimeSelect
                            timeFormat="HH:mm"
                            timeIntervals={15}
                            dateFormat="yyyy-MM-dd HH:mm"
                            className="form-control"
                            placeholderText="Select Start Date"
                        />
                    </Form.Group>
                </Col>
                <Col md={3}>
                    <Form.Group controlId="endDate" className="text-start">
                        <Form.Label>End Date</Form.Label>
                        <DatePicker
                            selected={endDate}
                            onChange={(date: Date | null) => {
                                if (date) {
                                    setEndDate(date);
                                }
                            }}
                            showTimeSelect
                            timeFormat="HH:mm"
                            timeIntervals={15}
                            dateFormat="yyyy-MM-dd HH:mm"
                            className="form-control"
                            placeholderText="Select End Date"
                        />
                    </Form.Group>
                </Col>
                <Col md={2}>
                    <Form className="text-start">
                        <Form.Group controlId="limit">
                            <Form.Label>Limit (max 20)</Form.Label>
                            <Form.Control
                                type="number"
                                value={limit}
                                max={20}
                                onChange={(e) => {
                                    const value = Number(e.target.value);
                                    if (value <= 20) { // Set the maximum limit here
                                        setLimit(value);
                                    }
                                }}
                                placeholder="Enter Limit"
                            />
                        </Form.Group>
                    </Form>
                </Col>
                <Col className="d-flex align-items-end">
                    <Button variant="primary" onClick={handleSearch} className="mt-2">
                        Search
                    </Button>
                </Col>
            </Row>
            {requests.length > 0 ? (
            <Col className="d-flex align-items-center mt-5">
                <ToggleButton
                    variant={toggleVariantColour}
                    onClick={toggleVariant}
                    id={'light-dark'}
                    value={variant}
                >
                    {/* Remove the text from inside the button */}
                </ToggleButton>
                <span className="ms-2">Light/Dark</span> {/* Add the text next to the button */}
            </Col>
            ): ''
            }
            {requests.length > 0 ? (
                <RequestTable
                    variant={variant}
                    requests={requests} 
                    selectable={false} // No select column in history tab
                    selectedRequests={[]}
                    handleSelectRequest={() => {}} // No-op handler
                /> 
            ) : (
                hasSearched && (
                    <Alert variant="info" className="mt-3">
                        No records found.
                    </Alert>
                )
            )}
        </>
    );
};

export default HistoryTabPane;
