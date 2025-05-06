import 'bootstrap/dist/css/bootstrap.css';
import './Profile.css';
import Container from 'react-bootstrap/Container';
import Navbar from 'react-bootstrap/Navbar';
import Nav from 'react-bootstrap/Nav';
import NavDropdown from 'react-bootstrap/NavDropdown';
import NavBrand from '../navBrand/NavBrand';
import { UserData } from '../../types';
import microsoftLogo from '../../assets/azure-icon.svg';
import { useState } from 'react';

const Profile = ({ user, onSignOut }: { user: UserData; onSignOut: () => void }) => {
    const [showDropdown, setShowDropdown] = useState(false);

    const handleMouseEnter = () => setShowDropdown(true);
    const handleMouseLeave = () => setShowDropdown(false);

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
                                    <NavDropdown.Item onClick={onSignOut}>Sign Out</NavDropdown.Item>
                                </NavDropdown>
                            </div>
                        </Navbar.Collapse>
                    </div>
            </Navbar>
        </div>
    );
};

export default Profile;
