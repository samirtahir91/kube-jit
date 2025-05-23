import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import "./App.css";
import Profile from "./components/profile/Profile";
import RequestTabPane from "./components/request/RequestTabPane";
import Login from "./components/login/Login";
import { Card, Nav, Tab, Badge } from "react-bootstrap";
import ApproveTabPane from "./components/approve/ApproveTabPane";
import HistoryTabPane from "./components/history/HistoryTabPane";
import AdminTabPane from "./components/admin/AdminTabPane";
import Footer from "./components/footer/Footer";
import axios from "axios";
import { SyncLoader } from "react-spinners";
import config from "./config/config";
import { ApiResponse } from "./types";


// 401 interceptio redirect to logout and clear cookies
axios.interceptors.response.use(
    response => response,
    async error => {
        if (error.response && error.response.status === 401) {
            // Call logout endpoint to clear cookies/session
            try {
                await axios.post(`${config.apiBaseUrl}/kube-jit-api/logout`, {}, { withCredentials: true });
            } catch {
                // Ignore logout errors
            }
            // Redirect to login page
            window.location.href = "/";
            return;
        }
        return Promise.reject(error);
    }
);

function App() {
    const [data, setData] = useState<ApiResponse | null>(null);
    const [loading, setLoading] = useState(false);
    const [loadingInCard, setLoadingInCard] = useState(false);
    const [login, setLogin] = useState(false);
    const [activeTab, setActiveTab] = useState<string>("request");
    const [originTab, setOriginTab] = useState<string>("");
    const [isApprover, setIsApprover] = useState<boolean>(false);
    const [isAdmin, setIsAdmin] = useState<boolean>(false);
    const [isPlatformApprover, setIsPlatformApprover] = useState<boolean>(false);
    const [buildSha, setBuildSha] = useState<string | undefined>();
    const navigate = useNavigate();

    // Handle sign out
    const handleSignOut = async () => {
        // Clear local storage
        localStorage.removeItem("tokenExpiry");
        localStorage.removeItem("loginMethod");
    
        try {
            // Send a request to the server to clear HTTP-only cookies
            await axios.post(`${config.apiBaseUrl}/kube-jit-api/logout`, {}, { withCredentials: true });
        } catch (error) {
            console.error("Error during logout:", error);
        }
    
        // Reset state and navigate to login
        setData(null);
        setLogin(true);
        navigate("/"); // Redirect to the login page
    };

    // checkPermissions function to check if the user is an admin or approver
    const checkPermissions = async (provider: string | null) => {
        try {
            const payload = {
                provider: provider,
            };
            axios.post(`${config.apiBaseUrl}/kube-jit-api/permissions`, payload, {
                withCredentials: true,
            })
            .then(response => {
                setIsApprover(response.data.isApprover);
                setIsAdmin(response.data.isAdmin);
                setIsPlatformApprover(response.data.isPlatformApprover);
            })
            .catch(error => {
                console.error("Error in permissions request:", error);
            });
        } catch (error) {
            console.error("Error checking approver status:", error);
        }
    };

    // Fetch profile data and check if the user is logged in
    useEffect(() => {
        const tokenExpiry = localStorage.getItem("tokenExpiry");

        const fetchAllData = async () => {
            setLoading(true);
            const provider = localStorage.getItem("loginMethod");
            try {
                // Fetch profile data
                const res = await fetch(`${config.apiBaseUrl}/kube-jit-api/${provider}/profile`, {
                    credentials: "include",
                });
                if (!res.ok) {
                    throw new Error("Not logged in");
                }
                const profileData = await res.json();
                if (profileData && profileData.name) {
                    setData({
                        userData: profileData,
                        expiresIn: 0, // unused in profile fetch
                    });
                    navigate(window.location.pathname); // Clear the URL parameters
                } else {
                    console.error("Invalid profile data structure:", profileData);
                }
            } catch (error) {
                console.error("Error fetching profile data:", error);
                setLogin(true);
            } finally {
                setLoading(false);
            }
        };

        if (tokenExpiry && new Date(tokenExpiry) > new Date()) {
            fetchAllData();
        } else {
            setLogin(true);
        }
    }, [navigate]);

    // Check approver status and fetch groups after login
    useEffect(() => {
        if (data && data.userData) {
            const provider = localStorage.getItem("loginMethod");
            if (provider) {
                const fetchApproverAndGroups = async () => {
                    await checkPermissions(provider);
                };
                fetchApproverAndGroups();
            }
        }
    }, [data]);

    // Fetch build SHA once on mount
    useEffect(() => {
        // Fetch build SHA once on mount
        axios.get(`${config.apiBaseUrl}/kube-jit-api/build-sha`)
            .then(res => setBuildSha(res.data.sha))
            .catch(() => setBuildSha(undefined));
    }, []);

    const badgeTitle = isAdmin
    ? "Admin"
    : isPlatformApprover
    ? "Platform Approver"
    : isApprover
    ? "Approver"
    : null;

    if (loading) {
        return (
            <div className="app-content">
                <div className="card-loader-container">
                    <SyncLoader color="#0494ba" size={20} />
                </div>
                <Footer buildSha={buildSha} />
            </div>
        );
    }

    // Render the main card if logged in
    if (data && data.userData) {
        return (
            <div className="app-content">
                <SyncLoader className="card-loader-container" color="#0494ba" size={20} loading={loadingInCard} />
                <div className="d-flex justify-content-between align-items-center mb-">
                    <Profile user={data.userData} onSignOut={handleSignOut} />
                </div>
                <Card className="main-card d-flex justify-content-center align-items-start">
                    <Card.Body className="container">
                        <Tab.Container
                            id="left-tabs-example"
                            activeKey={activeTab}
                            onSelect={(selectedKey) => setActiveTab(selectedKey || "request")}
                        >
                            <Nav variant="tabs" className="d-flex align-items-center">
                                <Nav.Item>
                                    <Nav.Link href="#requestJit" eventKey="request">
                                        Request
                                    </Nav.Link>
                                </Nav.Item>
                                {(isApprover || isAdmin || isPlatformApprover) && (
                                    <Nav.Item>
                                        <Nav.Link href="#approveJit" eventKey="approve">
                                            Approve
                                        </Nav.Link>
                                    </Nav.Item>
                                )}
                                <Nav.Item>
                                    <Nav.Link href="#jitRecords" eventKey="history">
                                        History
                                    </Nav.Link>
                                </Nav.Item>
                                {isAdmin && (
                                    <Nav.Item>
                                        <Nav.Link href="#admin" eventKey="admin">
                                            Admin
                                        </Nav.Link>
                                    </Nav.Item>
                                )}
                                {badgeTitle && (
                                    <Badge
                                        bg="success"
                                        className="ms-auto"
                                        style={{
                                            fontSize: "0.9rem",
                                            padding: "0.3em 0.6em",
                                            borderRadius: "0.5em",
                                            height: "fit-content",
                                        }}
                                    >
                                        {badgeTitle}
                                    </Badge>
                                )}
                            </Nav>
                            <Tab.Content>
                                <RequestTabPane
                                    username={data.userData.name}
                                    userId={data.userData.id}
                                    setActiveTab={setActiveTab}
                                    setOriginTab={setOriginTab}
                                    setLoadingInCard={setLoadingInCard}
                                />
                                {(isApprover || isAdmin || isPlatformApprover) && (
                                    <ApproveTabPane
                                        username={data.userData.name}
                                        userId={data.userData.id}
                                        setLoadingInCard={setLoadingInCard}
                                    />
                                )}
                                <HistoryTabPane
                                    isAdmin={isAdmin}
                                    isPlatformApprover={isPlatformApprover}
                                    activeTab={activeTab}
                                    originTab={originTab}
                                    userId={data.userData.id}
                                    setLoadingInCard={setLoadingInCard}
                                />
                                {isAdmin && (
                                    <Tab.Pane eventKey="admin">
                                        <AdminTabPane setLoadingInCard={setLoadingInCard} />
                                    </Tab.Pane>
                                )}
                            </Tab.Content>
                        </Tab.Container>
                    </Card.Body>
                </Card>
                <Footer buildSha={buildSha} />
            </div>
        );
    // If not logged in, show the login page
    } else if (login) {
        return (
            <div className="app-content login-page">
                <Login
                    onLoginSuccess={(data) => {
                        setData(data)
                        setLogin(false)
                    }}
                    setLoading={setLoading}
                />
                <Footer buildSha={buildSha} />
            </div>
        );
    }

    return (
        <div className="app-content">
            <Footer buildSha={buildSha} />
        </div>
    );
}

export default App;
