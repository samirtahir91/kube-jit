import React, { useEffect, useState } from "react";
import { Navbar, Container, Card } from "react-bootstrap";
import githubLogo from "../../assets/github.svg";
import loginLogo from "../../assets/login-logo.png";
import microsoftLoginLogo from "../../assets/azure-icon.svg";
import NavBrand from "../navBrand/NavBrand";
import axios from "axios";
import config from "../../config/config";
import "./Login.css";
import { UserData } from '../../types';


type LoginProps = {
  onLoginSuccess: (data: UserData) => void;
  setLoading: React.Dispatch<React.SetStateAction<boolean>>;
};

const Login: React.FC<LoginProps> = ({ onLoginSuccess, setLoading }) => {
  const [loginMethod, setLoginMethod] = useState<"github" | "google" | "azure" | "">("");
  const [clientID, setClientID] = useState<string | null>(null);
  const [redirectUri, setRedirectUrl] = useState<string | null>(null);
  const [auth_url, setAuthUrl] = useState<string | null>(null);

  // Fetch the client ID and set provider
  useEffect(() => {
    axios
      .get(`${config.apiBaseUrl}/kube-jit-api/client_id`)
      .then((response) => {
        setClientID(response.data.client_id);
        setRedirectUrl(response.data.redirect_uri);
        setAuthUrl(response.data.auth_url);
        localStorage.setItem("loginMethod", response.data.provider);
        setLoginMethod(response.data.provider);
      })
      .catch((error) => {
        console.error("Error fetching Oauth client ID:", error);
      });
  }, []);

  // Handle OAuth callback logic
  useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    const code = urlParams.get("code");
    const method = urlParams.get("state");

    if (code && method) {
      setLoading(true); // Use setLoading from App.tsx
      axios
        .get(`${config.apiBaseUrl}/kube-jit-api/oauth/${method}/callback`, {
          params: { code },
          withCredentials: true,
        })
        .then((res) => {
          const data = res.data;
          if (data && data.userData) {
            localStorage.setItem(
              "tokenExpiry",
              new Date(new Date().getTime() + data.expiresIn * 1000).toString()
            );
            onLoginSuccess(data); // Notify App.tsx about the successful login
            window.history.replaceState({}, document.title, window.location.pathname); // Clear the URL parameters
          } else {
            console.error("Invalid data structure:", data);
            window.history.replaceState({}, document.title, window.location.pathname); // Clear the URL parameters
          }
          setLoading(false); // Stop loading
        })
        .catch((error) => {
          console.error("Error during OAuth callback:", error);
          window.history.replaceState({}, document.title, window.location.pathname); // Clear the URL parameters
          setLoading(false); // Stop loading
        });
    }
  }, [onLoginSuccess, setLoading]);

  // Redirect to GitHub OAuth
  const redirectToGitHub = () => {
    if (clientID && redirectUri) {
      const scope = "read:user user:email";
      const authUrl = `https://github.com/login/oauth/authorize?client_id=${clientID}&redirect_uri=${encodeURIComponent(
        redirectUri
      )}&scope=${scope}&state=github`;
      window.location.href = authUrl;
    } else {
      console.error("Client ID or Redirect URI is missing.");
    }
  };

  // Redirect to Google OAuth
  const redirectToGoogle = () => {
    if (clientID && redirectUri) {
      const authUrl = `https://accounts.google.com/o/oauth2/auth?client_id=${clientID}&redirect_uri=${encodeURIComponent(
        redirectUri
      )}&response_type=code&scope=openid%20email%20profile&state=google`;
      window.location.href = authUrl;
    } else {
      console.error("Client ID or Redirect URI is missing.");
    }
  };

  // Redirect to Azure OAuth
  const redirectToAzure = () => {
    if (clientID && redirectUri && auth_url) {
      const authUrl = `${auth_url}?client_id=${clientID}&redirect_uri=${encodeURIComponent(
        redirectUri
      )}&response_type=code&scope=openid%20email%20profile&state=azure`;
      window.location.href = authUrl;
    } else {
      console.error("Client ID or Redirect URI is missing.");
    }
  };

  return (
    <div>
      <div className="py-5">
        <Navbar className="navbar" fixed="top">
          <Container>
            <NavBrand />
          </Container>
        </Navbar>
      </div>

      <div
        className="d-flex justify-content-center align-items-center"
        style={{ height: "50vh" }}
      >
        <div className="d-flex" style={{ width: "50rem", height: "20rem" }}>
          <Card bg="light" style={{ flex: 1 }}>
            <Card.Body className="d-flex flex-column justify-content-between">
              <Card.Title
                className="text-dark text-start fw-bold"
                style={{ fontSize: "28px" }}
              >
                Sign in
              </Card.Title>
              <Card.Subtitle
                className="mb-2 text-secondary text-start"
                style={{ fontSize: "18px" }}
              >
                to get started.
              </Card.Subtitle>

              {/* Login buttons */}
              {loginMethod === "github" && clientID && redirectUri && (
                <button
                  className="text-light login-button w-100 mt-auto"
                  onClick={redirectToGitHub}
                >
                  <img
                    alt="GitHub Logo"
                    src={githubLogo}
                    width="20"
                    height="20"
                    className="me-2"
                  />
                  Log in with GitHub
                </button>
              )}
              {loginMethod === "google" && clientID && redirectUri && (
                <button
                  className="gsi-material-button w-100 mt-auto"
                  onClick={redirectToGoogle}
                >
                  <div className="gsi-material-button-state"></div>
                  <div className="gsi-material-button-content-wrapper">
                    <div className="gsi-material-button-icon">
                      <svg
                        version="1.1"
                        xmlns="http://www.w3.org/2000/svg"
                        viewBox="0 0 48 48"
                        xmlnsXlink="http://www.w3.org/1999/xlink"
                        style={{ display: "block" }}
                      >
                        <path
                          fill="#EA4335"
                          d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"
                        ></path>
                        <path
                          fill="#4285F4"
                          d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"
                        ></path>
                        <path
                          fill="#FBBC05"
                          d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"
                        ></path>
                        <path
                          fill="#34A853"
                          d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"
                        ></path>
                        <path fill="none" d="M0 0h48v48H0z"></path>
                      </svg>
                    </div>
                    <span className="gsi-material-button-contents">
                      Sign in with Google
                    </span>
                  </div>
                </button>
              )}
              {loginMethod === "azure" && clientID && redirectUri && auth_url && (
                <button
                  className="text-light azure-login-button w-100 mt-auto"
                  onClick={redirectToAzure}
                >
                  <img
                    alt="Azure Logo"
                    src={microsoftLoginLogo}
                          width="20"
                    height="20"
                    className="me-2"
                  />
                  Log in with Microsoft
                </button>
              )}
            </Card.Body>
          </Card>
          <img
            alt="Logo"
            src={loginLogo}
            style={{ flex: 1, objectFit: "cover", height: "100%" }}
          />
        </div>
      </div>
    </div>
  );
};

export default Login;
