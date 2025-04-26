import 'bootstrap/dist/css/bootstrap.css';
import Container from 'react-bootstrap/Container';
import Navbar from 'react-bootstrap/Navbar';
import Nav from 'react-bootstrap/Nav'
import NavBrand from '../navBrand/NavBrand';

type UserType = {
	avatar_url: string;
	login: string;
	id: string;
	type: string;
};

const Profile = ({ user }: { user: UserType }) => {
	return (
		<div className="py-5">
			{/* Nav bar with sign in info and links */}
			<Navbar className="navbar" fixed="top">
			<Container>
				<NavBrand/>{/* Main nav brand/logo */}
				<Nav.Link href="/" className="text-light">Home</Nav.Link>
				<Navbar.Collapse className="justify-content-end">
				<Navbar.Text className="text-light fw-bold">
					<img
						alt=""
						src={user.avatar_url}
						width="30"
						height="30"
						className="d-inline-block align-top"
					/>{" " + user.login}
				</Navbar.Text>
				</Navbar.Collapse>
			</Container>
			</Navbar>
		</div>
	);
};

export default Profile;
