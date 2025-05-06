import { useEffect, useState, useCallback } from 'react';
import axios from 'axios';
import { Button, Form, Col, Row } from 'react-bootstrap';
import DatePicker from 'react-datepicker';
import 'react-datepicker/dist/react-datepicker.css';
import './HistoryTabPane.css';
import RequestTable from '../requestTable/RequestTable';
import { Request } from '../../types';
import config from '../../config/config';

type HistoryTabPaneProps = {
    isAdmin: boolean;
    isPlatformApprover: boolean;
    activeTab: string;
    originTab: string;
    userId: string;
    setLoadingInCard: (loading: boolean) => void;
};

const HistoryTabPane = ({ isAdmin, isPlatformApprover, activeTab, originTab, userId, setLoadingInCard }: HistoryTabPaneProps) => {
    const [requests, setRequests] = useState<Request[]>([]);
    const [errorMessage, setErrorMessage] = useState('');
    const [limit, setLimit] = useState(1);
    const [startDate, setStartDate] = useState<Date | null>(null);
    const [endDate, setEndDate] = useState<Date | null>(null);
    const [searchUserId, setSearchUserId] = useState('');
    const [searchUsername, setSearchUsername] = useState('');
    const [hasSearched, setHasSearched] = useState(false);
    const [variant, setVariant] = useState<'light' | 'dark'>('light');

    const fetchRequests = useCallback(async (limit: number, startDate: Date | null, endDate: Date | null) => {
        setLoadingInCard(true); // Start loading
        try {
            const response = await axios.get(`${config.apiBaseUrl}/kube-jit-api/history`, {
                params:
                    {
                        userID: (isAdmin || isPlatformApprover) ? searchUserId : userId,
                        username: (isAdmin || isPlatformApprover) ? searchUsername : undefined,
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
    }, [userId, isAdmin, isPlatformApprover, searchUserId, searchUsername, setLoadingInCard]);

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

    return (
        <>
            {errorMessage && (
                <div className="error-message mt-3">
                    <i className="bi bi-exclamation-circle-fill me-2"></i>
                    {errorMessage}
                    <button className="error-message-close" onClick={() => setErrorMessage('')}>
                        &times;
                    </button>
                </div>
            )}
            <Row className="mt-4 align-items-end history-search-row">
                {(isAdmin || isPlatformApprover) && (
                    <>
                        <Col md={2} xs={6} className="mb-3">
                            <Form.Group controlId="searchUserId" className="text-start">
                                <Form.Label>User ID</Form.Label>
                                <Form.Control
                                    type="text"
                                    value={searchUserId}
                                    onChange={(e) => setSearchUserId(e.target.value)}
                                    placeholder="Enter User ID"
                                />
                            </Form.Group>
                        </Col>
                        <Col md={2} xs={6} className="mb-3">
                            <Form.Group controlId="searchUsername" className="text-start">
                                <Form.Label>Username</Form.Label>
                                <Form.Control
                                    type="text"
                                    value={searchUsername}
                                    onChange={(e) => setSearchUsername(e.target.value)}
                                    placeholder="Enter Username"
                                />
                            </Form.Group>
                        </Col>
                    </>
                )}
                <Col md={2} xs={12} className="mb-3">
                    <Form.Group controlId="startDate" className="text-start">
                        <Form.Label>Start Date</Form.Label>
                        <DatePicker
                            selected={startDate}
                            onChange={(date: Date | null) => date && setStartDate(date)}
                            showTimeSelect
                            timeFormat="HH:mm"
                            timeIntervals={15}
                            dateFormat="yyyy-MM-dd HH:mm"
                            className="form-control"
                            placeholderText="Select Start Date"
                        />
                    </Form.Group>
                </Col>
                <Col md={2} xs={12} className="mb-3">
                    <Form.Group controlId="endDate" className="text-start">
                        <Form.Label>End Date</Form.Label>
                        <DatePicker
                            selected={endDate}
                            onChange={(date: Date | null) => date && setEndDate(date)}
                            showTimeSelect
                            timeFormat="HH:mm"
                            timeIntervals={15}
                            dateFormat="yyyy-MM-dd HH:mm"
                            className="form-control"
                            placeholderText="Select End Date"
                        />
                    </Form.Group>
                </Col>
                <Col md={2} xs={6} className="mb-3">
                    <Form.Group controlId="limit" className="text-start">
                        <Form.Label>Limit (max {(isAdmin || isPlatformApprover) ? 100 : 20})</Form.Label>
                        <Form.Control
                            type="number"
                            value={limit}
                            max={(isAdmin || isPlatformApprover) ? 100 : 20}
                            onChange={(e) => {
                                const value = Number(e.target.value);
                                if (value <= ((isAdmin || isPlatformApprover) ? 100 : 20)) {
                                    setLimit(value);
                                }
                            }}
                            placeholder={`Enter Limit (max ${(isAdmin || isPlatformApprover) ? 100 : 20})`}
                        />
                    </Form.Group>
                </Col>
                <Col md="auto" xs={12} className="mb-3">
                    <Button
                        size="sm"
                        className="search-button"
                        onClick={handleSearch}
                    >
                        Search
                    </Button>
                </Col>
            </Row>
            {requests && requests.length > 0 ? (
                <RequestTable
                    variant={variant}
                    setVariant={setVariant}
                    requests={requests}
                    mode="history"
                    selectable={false}
                    selectedRequests={[]}
                    handleSelectRequest={() => {}}
                />
            ) : (
                hasSearched && (
                    <div className="success-message mt-3">
                        <i className="bi bi-info-circle me-2"></i>
                        No records found.
                        <button className="success-message-close" onClick={() => setHasSearched(false)}>
                            &times;
                        </button>
                    </div>
                )
            )}
        </>
    );
};

export default HistoryTabPane;
