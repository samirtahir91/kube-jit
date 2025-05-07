import 'bootstrap/dist/css/bootstrap.css';
import './Profile.css';
import Navbar from 'react-bootstrap/Navbar';
import Nav from 'react-bootstrap/Nav';
import NavDropdown from 'react-bootstrap/NavDropdown';
import NavBrand from '../navBrand/NavBrand';
import { UserData } from '../../types';
import microsoftLogo from '../../assets/azure-icon.svg';
import { useState } from 'react';
import axios from 'axios';
import Modal from 'react-bootstrap/Modal';
import Button from 'react-bootstrap/Button';
import config from '../../config/config';

const Profile = ({ user, onSignOut }: { user: UserData; onSignOut: () => void }) => {
    const [showDropdown, setShowDropdown] = useState(false);
    const [showPermissions, setShowPermissions] = useState(false);
    const [permissions, setPermissions] = useState<any>(null);
    const [loading, setLoading] = useState(false);

    const handleMouseEnter = () => setShowDropdown(true);
    const handleMouseLeave = () => setShowDropdown(false);

    const handleShowPermissions = async () => {
        setLoading(true);
        setShowPermissions(true);
        try {
            const provider = localStorage.getItem("loginMethod");
            const res = await axios.post(
                `${config.apiBaseUrl}/kube-jit-api/permissions`,
                { provider },
                { withCredentials: true }
            );
            setPermissions(res.data);
        } catch (err) {
            setPermissions({ error: "Failed to fetch permissions" });
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="py-5">
            <Navbar className="navbar" fixed="top" expand="lg">
                <div className="navbar-inner d-flex align-items-center justify-content-between">
                    <Navbar.Brand className="d-flex align-items-center">
                        <NavBrand />
                        <Nav.Link href="/" className="text-light ms-3">Home</Nav.Link>
                    </Navbar.Brand>
                    <Navbar.Toggle aria-controls="custom-navbar-collapse" />
                    <Navbar.Collapse id="custom-navbar-collapse" bsPrefix="custom-navbar-collapse" className="justify-content-end">
                        <div
                            id="user-dropdown"
                            onMouseEnter={handleMouseEnter}
                            onMouseLeave={handleMouseLeave}
                            className="position-relative"
                        >
                            <NavDropdown
                                title={
                                    <>
                                        <img
                                            alt=""
                                            src={user.avatar_url || microsoftLogo}
                                            width="30"
                                            height="30"
                                            className="d-inline-block align-top me-2"
                                        />
                                        {user.name}
                                    </>
                                }
                                id="user-dropdown-toggle"
                                align="end"
                                className="text-light"
                                show={showDropdown}
                            >
                                <NavDropdown.Item onClick={handleShowPermissions}>
                                    My Permissions
                                </NavDropdown.Item>
                                <NavDropdown.Item onClick={onSignOut}>Sign Out</NavDropdown.Item>
                            </NavDropdown>
                        </div>
                    </Navbar.Collapse>
                </div>
            </Navbar>

            {/* Permissions Modal */}
            <Modal show={showPermissions} onHide={() => setShowPermissions(false)} centered>
                <Modal.Header closeButton>
                    <Modal.Title>My Permissions</Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    {loading && <div>Loading...</div>}
                    {permissions && !loading && (
                        permissions.error ? (
                            <div className="text-danger">{permissions.error}</div>
                        ) : (
                            <div>
                                {permissions.isAdmin !== null && (
                                    <div><strong>Is Admin:</strong> {permissions.isAdmin ? "Yes" : "No"}</div>
                                )}
                                {permissions.isPlatformApprover !== null && (
                                    <div><strong>Is Platform Approver:</strong> {permissions.isPlatformApprover ? "Yes" : "No"}</div>
                                )}
                                {permissions.isApprover !== null && (
                                    <div><strong>Is Approver:</strong> {permissions.isApprover ? "Yes" : "No"}</div>
                                )}
                                {Array.isArray(permissions.approverGroups) && permissions.approverGroups.length > 0 && (
                                    <div>
                                        <strong>Approver Groups:</strong>
                                        <ul>
                                            {permissions.approverGroups.map((g: string) => (
                                                <li key={g}>{g}</li>
                                            ))}
                                        </ul>
                                    </div>
                                )}
                                {Array.isArray(permissions.adminGroups) && permissions.adminGroups.length > 0 && (
                                    <div>
                                        <strong>Admin Groups:</strong>
                                        <ul>
                                            {permissions.adminGroups.map((g: string) => (
                                                <li key={g}>{g}</li>
                                            ))}
                                        </ul>
                                    </div>
                                )}
                            </div>
                        )
                    )}
                </Modal.Body>
                <Modal.Footer>
                    <Button variant="secondary" onClick={() => setShowPermissions(false)}>
                        Close
                    </Button>
                </Modal.Footer>
            </Modal>
        </div>
    );
};

export default Profile;
