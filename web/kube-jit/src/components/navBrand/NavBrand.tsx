import { Navbar } from 'react-bootstrap';
import loginLogo from '../../assets/login-logo-newnew.png';

const NavBrand = () => {
    return (
        <Navbar.Brand className="text-light fw-bold">
        <img
          alt="Login Logo"
          src={loginLogo}
          width="30"
          height="30"
          className="me-2"
        />
        Kube JIT
      </Navbar.Brand>
    );
};

export default NavBrand;
