import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import "./App.css";
import Profile from "./components/profile/Profile";
import RequestTabPane from "./components/request/RequestTabPane";
import Login from "./components/login/Login";
import { Card, Nav, Tab } from "react-bootstrap";
import ApproveTabPane from "./components/approve/ApproveTabPane";
import HistoryTabPane from "./components/history/HistoryTabPane";
import axios from "axios";
import { SyncLoader } from "react-spinners";
import { UserData } from "./types";
import config from "./config/config";

type ApiResponse = {
    userData: UserData;
    expiresIn: number;
};

type Group = {
    id: number;
    name: string;
};

function App() {
    const [data, setData] = useState<ApiResponse | null>(null);
    const [loading, setLoading] = useState(false);
    const [loadingInCard, setLoadingInCard] = useState(false);
    const [login, setLogin] = useState(false);
    const [activeTab, setActiveTab] = useState<string>("request");
    const [originTab, setOriginTab] = useState<string>("");
    const [approverGroups, setApproverGroups] = useState<Group[]>([]);
    const [isApprover, setIsApprover] = useState<boolean>(false);
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

    // Define fetchGroups outside useEffect
    const fetchGroups = async () => {
        try {
            const response = await axios.get(`${config.apiBaseUrl}/kube-jit-api/approving-groups`, {
                withCredentials: true,
            });
            setApproverGroups(response.data);
        } catch (error) {
            console.error("Error fetching approver groups:", error);
        }
    };

    // Define checkIsApprover outside useEffect
    const checkIsApprover = async (provider: string | null) => {
        try {
            const response = await axios.get(`${config.apiBaseUrl}/kube-jit-api/${provider}/is-approver`, {
                withCredentials: true,
            });
            setIsApprover(response.data.isApprover);
        } catch (error) {
            console.error("Error checking approver status:", error);
        }
    };

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

    // Simplified useEffect to dynamically check approver status and fetch groups after login
    useEffect(() => {
        if (data && data.userData) {
            const provider = localStorage.getItem("loginMethod");
            if (provider) {
                const fetchApproverAndGroups = async () => {
                    await checkIsApprover(provider);
                    await fetchGroups();
                };
                fetchApproverAndGroups();
            }
        }
    }, [data]);

    if (loading) {
        return (
            <div className="card-loader-container">
                <SyncLoader color="#0494ba" size={20} />
            </div>
        );
    }

    if (data && data.userData && approverGroups.length > 0) {
        return (
            <div>
                <SyncLoader className="card-loader-container" color="#0494ba" size={20} loading={loadingInCard} />
                <Profile user={data.userData} onSignOut={handleSignOut} />
                <Card className="d-flex justify-content-center align-items-start">
                    <Card.Body className="container">
                        <Tab.Container
                            id="left-tabs-example"
                            activeKey={activeTab}
                            onSelect={(selectedKey) => setActiveTab(selectedKey || "request")}
                        >
                            <Nav variant="tabs">
                                <Nav.Item>
                                    <Nav.Link href="#requestJit" eventKey="request">
                                        Request
                                    </Nav.Link>
                                </Nav.Item>
                                {isApprover && (
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
                            </Nav>
                            <Tab.Content>
                                <RequestTabPane
                                    username={data.userData.name}
                                    userId={data.userData.id}
                                    approverGroups={approverGroups}
                                    setActiveTab={setActiveTab}
                                    setOriginTab={setOriginTab}
                                    setLoadingInCard={setLoadingInCard}
                                />
                                {isApprover && (
                                    <ApproveTabPane
                                        username={data.userData.name}
                                        userId={data.userData.id}
                                        setLoadingInCard={setLoadingInCard}
                                    />
                                )}
                                <HistoryTabPane
                                    activeTab={activeTab}
                                    originTab={originTab}
                                    userId={data.userData.id}
                                    setLoadingInCard={setLoadingInCard}
                                />
                            </Tab.Content>
                        </Tab.Container>
                    </Card.Body>
                </Card>
            </div>
        );
    } else if (login) {
        return (
            <Login
                onLoginSuccess={(data) => {
                    setData(data)
                    setLogin(false)
                }}
                setLoading={setLoading}
            />
        );
    }

    return null;
}

export default App;
