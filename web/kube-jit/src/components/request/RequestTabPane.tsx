import "./RequestTabPane.css"
import { useEffect, useRef, useState } from 'react';
import axios from 'axios';
import Select, { SingleValue } from 'react-select';
import { Form, Button, Tab, Row, Col, Modal } from 'react-bootstrap';
import InputTag from "../inputTag/InputTag";
import { Tag as ReactTag } from 'react-tag-input';
import DatePicker from 'react-datepicker';
import config from '../../config/config';
import yaml from 'js-yaml';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';

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
    userId: string;
    username: string;
};


const RequestTabPane = ({ username, userId, setLoadingInCard, setActiveTab, setOriginTab }: RequestTabPaneProps) => {
    const [roles, setRoles] = useState<Role[]>([]);
    const [clusters, setClusters] = useState<string[]>([]);
    const [showModal, setShowModal] = useState(false);
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
    const [clusterError, setClusterError] = useState('');
    const [roleError, setRoleError] = useState('');
    const nsInputTagRef = useRef<{ resetTags: () => void; setTagsFromStrings: (tags: string[]) => void }>(null);
    const userInputTagRef = useRef<{ resetTags: () => void; setTagsFromStrings: (tags: string[]) => void }>(null);
    const [showInfoBox, setShowInfoBox] = useState(false);
    
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

    // Handle bulk upload
    const handleBulkUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = (event) => {
            try {
                let data: any;
                if (file.name.endsWith(".json")) {
                    data = JSON.parse(event.target?.result as string);
                } else if (file.name.endsWith(".yaml") || file.name.endsWith(".yml")) {
                    data = yaml.load(event.target?.result as string);
                } else {
                    throw new Error("Unsupported file type. Please upload a .yaml, .yml, or .json file.");
                }

                // Users and Namespaces (already validated by InputTag)
                if (data.users && userInputTagRef.current) {
                    const userArr = Array.isArray(data.users)
                        ? data.users
                        : data.users.split(/[;, ]/).filter(Boolean);
                    userInputTagRef.current.setTagsFromStrings(userArr);
                }
                if (data.namespaces && nsInputTagRef.current) {
                    const nsArr = Array.isArray(data.namespaces)
                        ? data.namespaces
                        : data.namespaces.split(/[;, ]/).filter(Boolean);
                    nsInputTagRef.current.setTagsFromStrings(nsArr);
                }

                // Justification: enforce 100 char limit
                if (data.justification) {
                    if (data.justification.length <= 100) {
                        setJustification(data.justification);
                    } else {
                        setErrorMessage("Justification exceeds 100 characters.");
                    }
                }

                // Cluster: only set if value is in allowed clusters
                if (data.cluster && clusters.includes(data.cluster)) {
                    setSelectedCluster({ label: data.cluster, value: data.cluster });
                    setClusterError('');
                } else if (data.cluster) {
                    setClusterError(`Cluster "${data.cluster}" in upload is not a valid option.`);
                    setSelectedCluster(null);
                }

                // Role: only set if value is in allowed roles
                const allowedRole = roles.find(role => role.name === data.role);
                if (data.role && allowedRole) {
                    setSelectedRole({ label: data.role, value: data.role });
                    setRoleError('');
                } else if (data.role) {
                    setRoleError(`Role "${data.role}" in upload is not a valid option.`);
                    setSelectedRole(null);
                }

                // Do NOT set startDate or endDate from bulk upload

            } catch (err) {
                setErrorMessage("Failed to parse file: " + (err as Error).message);
            }
        };
        reader.readAsText(file);
        // Reset the file input so the same file can be uploaded again
        event.target.value = "";
    };

    // Clear form fields
    const handleClearForm = () => {
        setSelectedRole(null);
        setSelectedCluster(null);
        setNamespaces([]);
        setUsers([]);
        setJustification('');
        setStartDate(null);
        setEndDate(null);
        setSuccessMessage('');
        setErrorMessage('');
        setNsTagError('');
        setUserTagError('');
        setClusterError('');
        setRoleError('');

        // Reset InputTags
        if (nsInputTagRef.current) {
            nsInputTagRef.current.resetTags();
        }
        if (userInputTagRef.current) {
            userInputTagRef.current.resetTags();
        }
    };

    return (
        <Tab.Pane eventKey="request" className="text-start py-3">
            <div className="request-page-container">
                {/* Description Section */}
                <div className="form-description">
                    <h2 className="form-title">Submit Access Request</h2>
                    <p className="form-subtitle">
                        Use this form to request levels of access to specific namespaces.<br />
                        <br></br>Please ensure all required fields are filled out accurately to avoid delays in processing your request.<br></br>
                        <br></br><strong>Bulk Upload:</strong> You can quickly fill out the form by uploading a YAML or JSON file via <b>Upload YAML/JSON</b>. The file should contain fields for user emails, namespaces, justification, cluster, and role. Start Date and End Date must still be selected manually.<br />
                        <br></br>For more information on how to fill out the form, click on the <b>Help</b> button to the bottom right.
                    </p>
                </div>

                <Row>
                    <Col md={6}>
                        <Form onSubmit={handleSubmit} className="text-start py-4 position-relative">
                            <div className="form-top-buttons d-flex align-items-center mb-3" style={{ gap: "1rem" }}>
                                <Button
                                    className="top-action-button clear-form-button"
                                    onClick={handleClearForm}
                                    type="button"
                                >
                                    <i className="bi bi-x-circle me-1"></i>Clear Form
                                </Button>
                                <input
                                    type="file"
                                    accept=".yaml,.yml,application/json"
                                    style={{ display: "none" }}
                                    id="bulk-upload-input"
                                    onChange={handleBulkUpload}
                                />
                                <Button
                                    className="top-action-button"
                                    onClick={() => document.getElementById("bulk-upload-input")?.click()}
                                    type="button"
                                >
                                    Upload YAML/JSON
                                </Button>
                            </div>
                            <Form.Group className="mb-3" controlId="users">
                                <Form.Label>User Emails</Form.Label>
                                <InputTag
                                    id="users"
                                    regexPattern={/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/}
                                    tagError={"Subjects must be valid email addresses"}
                                    ref={userInputTagRef}
                                    onTagsChange={handleUserTagsChange}
                                    setTagError={setUserTagError}
                                    placeholder="Enter email address(es)"
                                />
                                {userTagError && <Form.Text className="text-danger">{userTagError}</Form.Text>}
                            </Form.Group>
                            {/* Cluster field */}
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
                                    onChange={(selectedOption) => {
                                        setSelectedCluster(selectedOption);
                                        setClusterError('');
                                    }}
                                    value={selectedCluster}
                                />
                                {clusterError && <Form.Text className="text-danger">{clusterError}</Form.Text>}
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
                            {/* Role field */}
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
                                    onChange={(selectedOption) => {
                                        setSelectedRole(selectedOption);
                                        setRoleError('');
                                    }}
                                    value={selectedRole}
                                />
                                {roleError && <Form.Text className="text-danger">{roleError}</Form.Text>}
                            </Form.Group>
                            <Col md={6}>
                                <Form.Group controlId="startDate" className="mb-3">
                                    <Form.Label>Start Date</Form.Label>
                                    <div>
                                        <DatePicker
                                            id="startDate"
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
                                            id="endDate"
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
                                className="w-100 submit-button"
                                disabled={
                                    !selectedRole ||
                                    !selectedCluster ||
                                    users.length < 1 ||
                                    namespaces.length < 1 ||
                                    !justification ||
                                    !startDate ||
                                    !endDate
                                }
                            >
                                Submit Request
                            </Button>
                        </Form>
                    </Col>
                    <Col md={6}>
                        {!showInfoBox ? (
                            <button className="help-button" onClick={() => setShowInfoBox(true)}>
                                Help
                            </button>
                        ) : (
                            <div className="info-box">
                                <button className="info-box-close" onClick={() => setShowInfoBox(false)}>
                                    &times;
                                </button>
                                <h5 className="info-box-title">
                                    <i className="bi bi-info-circle me-2"></i>
                                    How to Submit a Request
                                </h5>
                                <ul className="info-box-list">
                                    <li><strong>User Emails:</strong> Enter the email addresses you are requesting access for (use comma/enter/space for a new email).</li>
                                    <li><strong>Cluster:</strong> Select the cluster you are requesting access for.</li>
                                    <li><strong>Namespaces:</strong> Enter the Namespaces you are requesting access for (use comma/enter/space for a new namespace).</li>
                                    <li><strong>Justification:</strong> Enter the reason/ticket reference for the access request.</li>
                                    <li><strong>Role:</strong> Select the role you are requesting access for.</li>
                                    <li><strong>Start Date:</strong> Select the date/time you want the access to begin.</li>
                                    <li><strong>End Date:</strong> Select the date/time you want the access to end.</li>
                                </ul>
                                <div style={{ marginTop: "1.5rem" }}>
                                    <h6 className="info-box-sample-title">Sample YAML bulk upload</h6>
                                    <SyntaxHighlighter language="yaml" style={oneLight} customStyle={{ borderRadius: 8, fontSize: "0.95em" }}>
{`users:
  - user1@example.com
  - user2@example.com
namespaces:
  - ns1
  - ns2
justification: Access for project
role: edit
cluster: dev-cluster`}
                                    </SyntaxHighlighter>
                                    <h6 className="info-box-sample-title">Sample JSON bulk upload</h6>
                                    <SyntaxHighlighter language="json" style={oneLight} customStyle={{ borderRadius: 8, fontSize: "0.95em" }}>
{`{
  "users": ["user1@example.com", "user2@example.com"],
  "namespaces": ["ns1", "ns2"],
  "justification": "Access for project",
  "role": "edit",
  "cluster": "dev-cluster"
}`}
                                    </SyntaxHighlighter>
                                </div>
                            </div>
                        )}
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
                    <div className="success-message">
                        <i className="bi bi-check-circle-fill me-2"></i> {/* Bootstrap success icon */}
                        {successMessage}
                        <button className="success-message-close" onClick={() => setSuccessMessage('')}>
                            &times; {/* Close icon */}
                        </button>
                    </div>
                )}
                {errorMessage && (
                    <div className="error-message">
                        <i className="bi bi-exclamation-circle-fill me-2"></i> {/* Bootstrap error icon */}
                        {errorMessage}
                        <button className="error-message-close" onClick={() => setErrorMessage('')}>
                            &times; {/* Close icon */}
                        </button>
                    </div>
                )}
            </div>
        </Tab.Pane>
    );
};

export default RequestTabPane;
