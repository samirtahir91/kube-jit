import 'bootstrap/dist/css/bootstrap.css';
import './Profile.css'; // Ensure the custom CSS is imported
import Container from 'react-bootstrap/Container';
import Navbar from 'react-bootstrap/Navbar';
import Nav from 'react-bootstrap/Nav';
import NavDropdown from 'react-bootstrap/NavDropdown';
import NavBrand from '../navBrand/NavBrand';
import { UserData } from '../../types';
import microsoftLogo from '../../assets/azure-icon.svg';

const Profile = ({ user, onSignOut }: { user: UserData; onSignOut: () => void }) => {
    return (
        <div className="py-5">
            {/* Nav bar with sign in info and links */}
            <Navbar className="navbar" fixed="top" expand="lg">
                <Container>
                    <Navbar.Brand className="d-flex align-items-center">
                        <NavBrand /> {/* Main nav brand/logo */}
                        <Nav.Link href="/" className="text-light ms-3">Home</Nav.Link> {/* Add margin to separate from brand */}
                    </Navbar.Brand>
                    <Navbar.Toggle aria-controls="custom-navbar-collapse" />
                    <Navbar.Collapse id="custom-navbar-collapse" bsPrefix="custom-navbar-collapse" className="justify-content-end">
                        <NavDropdown
                            title={
                                <>
                                    <img
                                        alt=""
                                        src={user.avatar_url || microsoftLogo} // Use avatar_url if available, otherwise fallback to microsoftLogo
                                        width="30"
                                        height="30"
                                        className="d-inline-block align-top me-2"
                                    />
                                    {user.name}
                                </>
                            }
                            id="user-dropdown"
                            align="end"
                            className="text-light"
                        >
                            <NavDropdown.Item onClick={onSignOut}>Sign Out</NavDropdown.Item>
                        </NavDropdown>
                    </Navbar.Collapse>
                </Container>
            </Navbar>
        </div>
    );
};

export default Profile;
