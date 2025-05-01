import "./RequestTabPane.css"
import { useEffect, useRef, useState } from 'react';
import axios from 'axios';
import Select, { SingleValue } from 'react-select';
import { Form, Button, Tab, Row, Col, Alert, Modal } from 'react-bootstrap';
import InputTag from "../inputTag/InputTag";
import { Tag as ReactTag } from 'react-tag-input';
import DatePicker from 'react-datepicker';
import config from '../../config/config';

type Group = {
    id: number;
    name: string;
};

type Role = {
    name: string;
};

type OptionType = {
    value: string | number;
    label: string;
};

type RequestTabPaneProps = {
    setActiveTab: (tab: string) => void;
    setOriginTab: (tab: string) => void;
    setLoadingInCard: (tab: boolean) => void;
    approverGroups: Group[];
    userId: string;
    username: string;
};

const RequestTabPane = ({ username, userId, approverGroups, setLoadingInCard, setActiveTab, setOriginTab }: RequestTabPaneProps) => {
    const [roles, setRoles] = useState<Role[]>([]);
    const [clusters, setClusters] = useState<string[]>([]);
    const [showModal, setShowModal] = useState(false);
    const [selectedGroup, setSelectedGroup] = useState<SingleValue<OptionType>>(null);
    const [selectedRole, setSelectedRole] = useState<SingleValue<OptionType>>(null);
    const [selectedCluster, setSelectedCluster] = useState<SingleValue<OptionType>>(null);
    const [startDate, setStartDate] = useState<Date | null>(null);
    const [endDate, setEndDate] = useState<Date | null>(null);
    const [namespaces, setNamespaces] = useState<string[]>([]);
    const [users, setUsers] = useState<string[]>([]);
    const [justification, setJustification] = useState<string>('');
    const [successMessage, setSuccessMessage] = useState('');
    const [errorMessage, setErrorMessage] = useState('');
    const [nsTagError, setNsTagError] = useState('');
    const [userTagError, setUserTagError] = useState('');
    const nsInputTagRef = useRef<{ resetTags: () => void }>(null);
    const userInputTagRef = useRef<{ resetTags: () => void }>(null);
        
    useEffect(() => {
        const fetchRoles = async () => {
            try {
                const response = await axios.get(`${config.apiBaseUrl}/kube-jit-api/roles-and-clusters`, {
                    withCredentials: true
                });
                setRoles(response.data.roles);
                setClusters(response.data.clusters);
            } catch (error) {
                console.error('Error fetching roles and clusters:', error);
            }
        };

        fetchRoles();
    }, []);

    // Handle namespace tags
    const handleNsTagsChange = (tags: ReactTag[]) => {
        setNamespaces(tags.map((tag: ReactTag) => tag.text));
    };

    // Handle user/email tags
    const handleUserTagsChange = (tags: ReactTag[]) => {
        setUsers(tags.map((tag: ReactTag) => tag.text));
    };
    
    // Handle form submission
    const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        setShowModal(true);
    };

    const clearMessagesAfterTimeout = (duration: number) => {
        setTimeout(() => {
            setSuccessMessage('');
            setErrorMessage('');
        }, duration); // Clear messages after x seconds
    };

    const handleConfirmSubmit = () => {
        setLoadingInCard(true);
        const timeoutDuration: number = 1000; // set timeout
        const payload = {
            justification,
            users,
            cluster: selectedCluster ? { name: selectedCluster.label } : null,
            namespaces,
            approvingTeam: selectedGroup ? { id: selectedGroup.value, name: selectedGroup.label } : null,
            role: selectedRole ? { name: selectedRole.label } : null,
            requestorId: userId.toString(),
            requestorName: username,
            status: "Requested",
            startDate,
            endDate
        };

        axios.post(`${config.apiBaseUrl}/kube-jit-api/submit-request`, payload, {
            withCredentials: true,
        })
        .then(response => {
            setLoadingInCard(false);
            setShowModal(false);
            setSuccessMessage(response.data.message);
            setErrorMessage(''); // Clear any previous error message
            setSelectedGroup(null); // Clear the selected group
            setSelectedRole(null); // Clear the selected role
            setSelectedCluster(null); // Clear the cluster
            setNamespaces([]); // Clear the namespace
            // Clear the namespace and email tags
            if (nsInputTagRef.current) {
                nsInputTagRef.current.resetTags();
            }
            if (userInputTagRef.current) {
                userInputTagRef.current.resetTags();
            }
            setJustification('') // Clear justification
            clearMessagesAfterTimeout(timeoutDuration); // Clear alert after some time
            setTimeout(() => {
                setOriginTab('request');
                setActiveTab('history'); // Redirect to the History tab after the alert message clears
                setTimeout(() => {
                    setOriginTab(''); // Reset originTab after switching to history tab
                }, 0); // Reset immediately after switching tabs
            }, timeoutDuration);
        })
        .catch(error => {
            console.error('Error:', error);
            setShowModal(false);
            setLoadingInCard(false);
            setErrorMessage(error.response ? error.response.data.error : 'Error submitting request');
            clearMessagesAfterTimeout(5000);
        });
    };

    return (
        <Tab.Pane eventKey="request">
            <Row>
                <Col md={6}>
                    <Form onSubmit={handleSubmit} className="text-start py-4">
                        <Form.Group className="mb-3" controlId="users">
                            <Form.Label>User Emails</Form.Label>
                            <InputTag
                                id="users"
                                regexPattern={/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/}
                                tagError={
                                    "Subjects must be valid email addresses"
                                }
                                ref={userInputTagRef}
                                onTagsChange={handleUserTagsChange}
                                setTagError={setUserTagError}
                                placeholder="Enter email address(es)"
                            />
                            {userTagError && <Form.Text className="text-danger">{userTagError}</Form.Text>}
                        </Form.Group>
                        <Form.Group controlId="cluster" className="mb-3">
                            <Form.Label>Cluster</Form.Label>
                            <Select
                                inputId="cluster"
                                name="cluster"
                                options={clusters.map(cluster => ({
                                    value: cluster,
                                    label: cluster
                                }))}
                                isSearchable
                                onChange={(selectedOption) => setSelectedCluster(selectedOption)}
                                value={selectedCluster}
                            />
                        </Form.Group>
                        <Form.Group className="mb-3" controlId="namespace">
                            <Form.Label>Namespace(s)</Form.Label>
                            <InputTag
                                id="namespace"
                                regexPattern={/^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9]{1})$/}
                                tagError={
                                    "Namespace must contain only lowercase "+
                                    "alphanumeric characters or '-', "+
                                    "start and end with an alphanumeric character, "+
                                    "and be no more than 63 characters long."
                                }
                                ref={nsInputTagRef}
                                onTagsChange={handleNsTagsChange}
                                setTagError={setNsTagError}
                                placeholder="Enter namespace(s)"
                            />
                            {nsTagError && <Form.Text className="text-danger">{nsTagError}</Form.Text>}
                        </Form.Group>
                        <Form.Group className="mb-3" controlId="justification">
                            <Form.Label>Justification</Form.Label>
                            <Form.Control
                                rows={2}
                                maxLength={100}
                                as="textarea"
                                required
                                name="justification"
                                placeholder="Enter a reason or reference (max 100 chars)"
                                value={justification}
                                onChange={(e) => setJustification(e.target.value)}
                            />
                        </Form.Group>
                        <Form.Group controlId="approvingTeam" className="mb-3">
                            <Form.Label>Approving Team</Form.Label>
                            <Select
                                inputId="approvingTeam"
                                name="approvingTeam"
                                options={approverGroups.map(group => ({
                                    value: group.id,
                                    label: group.name
                                }))}
                                isSearchable
                                onChange={(selectedOption) => setSelectedGroup(selectedOption)}
                                value={selectedGroup}
                            />
                        </Form.Group>
                        <Form.Group controlId="role" className="mb-3">
                            <Form.Label>Role</Form.Label>
                            <Select
                                inputId="role"
                                name="role"
                                options={roles.map(role => ({
                                    value: role.name,
                                    label: role.name
                                }))}
                                isSearchable
                                onChange={(selectedOption) => setSelectedRole(selectedOption)}
                                value={selectedRole}
                            />
                        </Form.Group>
                        <Col md={6}>
                            <Form.Group controlId="startDate" className="mb-3">
                                <Form.Label>Start Date</Form.Label>
                                <div>
                                    <DatePicker
                                        onKeyDown={(e) => {
                                            e.preventDefault();
                                        }}
                                        selected={startDate}
                                        onChange={(date: Date | null) => {
                                            if (date) {
                                                setStartDate(date);
                                                // Reset endDate if it is before the new startDate
                                                if (endDate && date > endDate) {
                                                    setEndDate(null);
                                                }
                                            }
                                        }}
                                        showTimeSelect
                                        timeFormat="HH:mm"
                                        timeIntervals={15}
                                        dateFormat="yyyy-MM-dd HH:mm"
                                        className="form-control"
                                        placeholderText="Select Start Date"
                                        minDate={new Date()} // Prevent selecting a date before today
                                        minTime={new Date(new Date().setSeconds(0, 0))} // Prevent selecting a time before now if today
                                        maxTime={new Date(new Date().setHours(23, 59, 59, 999))} // Allow selecting up to the end of the day
                                    />
                                </div>
                            </Form.Group>
                        </Col>
                        <Col md={6}>
                            <Form.Group controlId="endDate" className="mb-3">
                                <Form.Label>End Date</Form.Label>
                                <div>
                                    <DatePicker
                                        onKeyDown={(e) => {
                                            e.preventDefault();
                                        }}
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
                                        minDate={startDate || new Date()} // Prevent selecting a date before startDate
                                        minTime={
                                            startDate && startDate.toDateString()
                                                ? new Date(startDate.getTime() + 60 * 60 * 1000) // 1hr after startDate
                                                : new Date(new Date().setHours(0, 0, 0, 0)) // Default to midnight
                                        }
                                        maxTime={new Date(new Date().setHours(23, 59, 59, 999))} // Allow selecting up to the end of the day
                                    />
                                </div>
                            </Form.Group>
                        </Col>

                        <Button
                            variant="primary"
                            type="submit"
                            className="w-100"
                            disabled={
                                !selectedGroup ||
                                !selectedRole ||
                                !selectedCluster ||
                                users.length<1 ||
                                namespaces.length<1 ||
                                !justification
                            }
                        >
                            Submit Request
                        </Button>
                    </Form>
                </Col>
                <Col md={6}>
                    <div className="info-box">
                        <h5 className="info-box-title">How to Submit a Request</h5>
                        <ul className="info-box-list">
                            <li><strong>User Emails:</strong> Enter the email addresses you are requesting access for (use comma/enter/space for a new email).</li>
                            <li><strong>Cluster:</strong> Select the cluster you are requesting access for.</li>
                            <li><strong>Namespaces:</strong> Enter the Namespaces you are requesting access for (use comma/enter/space for a new namespace).</li>
                            <li><strong>Justification:</strong> Enter the reason/ticket reference for the access request.</li>
                            <li><strong>Approving Team:</strong> Select the team responsible for approving your request.</li>
                            <li><strong>Role:</strong> Select the role you are requesting access for.</li>
                            <li><strong>Start Date:</strong> Select the date/time you want the access to begin.</li>
                            <li><strong>End Date:</strong> Select the date/time you want the access to end.</li>
                        </ul>
                    </div>
                </Col>
            </Row>
            <Modal show={showModal} onHide={() => setShowModal(false)}>
                <Modal.Header closeButton>
                    <Modal.Title>Confirm Request</Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    <p>Are you sure you want to submit this request?</p>
                    <div>
                        <strong>User Emails:</strong>
                        {users.map((user) => (
                            <div key={user}>{user}</div>
                        ))}
                    </div>
                    <div>
                        <strong>Cluster:</strong> {selectedCluster ? selectedCluster.label : 'Not selected'}
                    </div>
                    <div>
                        <strong>Namespaces:</strong>
                        {namespaces.map((namespace) => (
                            <div key={namespace}>{namespace}</div>
                        ))}
                    </div>
                    <div>
                        <strong>Approving Team:</strong> {selectedGroup ? selectedGroup.label : 'Not selected'}
                    </div>
                    <div>
                        <strong>Role:</strong> {selectedRole ? selectedRole.label : 'Not selected'}
                    </div>
                    <div>
                        <strong>Start Date:</strong> {startDate ? startDate.toLocaleString() : 'Not selected'}
                    </div>
                    <div>
                        <strong>End Date:</strong> {endDate ? endDate.toLocaleString() : 'Not selected'}
                    </div>
                </Modal.Body>
                <Modal.Footer>
                    <Button variant="secondary" onClick={() => setShowModal(false)}>
                        Cancel
                    </Button>
                    <Button variant="primary" onClick={handleConfirmSubmit}>
                        Confirm
                    </Button>
                </Modal.Footer>
            </Modal>
            {successMessage && (
                <Alert variant="success" className="mt-3">
                    {successMessage}
                </Alert>
            )}
            {errorMessage && (
                <Alert variant="danger" className="mt-3">
                    {errorMessage}
                </Alert>
            )}
        </Tab.Pane>
    );
};

export default RequestTabPane;
