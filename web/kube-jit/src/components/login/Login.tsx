import { Navbar, Container, Card } from 'react-bootstrap';
import githubLogo from '../../assets/github.svg';
import loginLogo from '../../assets/login-logo-newnew.png';
import NavBrand from '../navBrand/NavBrand';
import axios from 'axios';
import { useState, useEffect } from 'react';

const Login = () => {

  const [clientID, setClientID] = useState('');

  useEffect(() => {
      axios.get('/kube-jit-api/github/client_id')
          .then(response => {
              setClientID(response.data.client_id);
          })
          .catch(error => {
              console.error('Error fetching Github App client ID:', error);
          });
  }, []);

  // Function to redirect the user to the GitHub OAuth authorization page
  function redirectToGitHub() {
    // const redirect_uri = "http://localhost:5173/"; // if unset default to github app callback
    // else add &redirect_uri=${redirect_uri} into authUrl
    const scope = "read:user"; // only used in github oauth apps (uses permissions of app and user in github apps)

    const authUrl = `https://github.com/login/oauth/authorize?client_id=${clientID}&scope=${scope}`;

    window.location.href = authUrl;
  }

  return (
    <div>
      <div className="py-5">
        <Navbar className="navbar" fixed="top">
          <Container>
            {/* Main nav brand/logo */}
            <NavBrand/>
          </Container>
        </Navbar>
      </div>

      <div className="d-flex justify-content-center align-items-center" style={{ height: '50vh' }}>
        {/* Main card with tabs and forms */}
        <div className="d-flex" style={{ width: '50rem', height: '20rem' }}>
          <Card bg="light" style={{ flex: 1 }}>
            <Card.Body className="d-flex flex-column justify-content-between">
              <Card.Title className="text-dark text-start fw-bold" style={{ fontSize: '28px' }}>Sign in</Card.Title>
              <Card.Subtitle className="mb-2 text-secondary text-start" style={{ fontSize: '18px' }}>to get started.</Card.Subtitle>
              <button className="text-light login-button w-100 mt-auto" onClick={redirectToGitHub}>
                <img
                  alt="GitHub Logo"
                  src={githubLogo}
                  width="20"
                  height="20"
                  className="me-2"
                />
                Log in with GitHub
              </button>
            </Card.Body>
          </Card>
          <img
            alt="Logo"
            src={loginLogo}
            style={{ flex: 1, objectFit: 'cover', height: '100%' }}
          />
        </div>
      </div>
    </div>
  );
};

export default Login;
