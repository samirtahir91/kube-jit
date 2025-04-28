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

    useEffect(() => {
        const tokenExpiry = localStorage.getItem("tokenExpiry");

        const fetchGroups = async () => {
            try {
                const response = await axios.get("http://localhost:8589/kube-jit-api/approving-groups", {
                    withCredentials: true,
                });
                setApproverGroups(response.data);
            } catch (error) {
                console.error("Error fetching approver groups:", error);
            }
        };

        const checkIsApprover = async (provider: string | null) => {
            try {
                const response = await axios.get(`http://localhost:8589/kube-jit-api/${provider}/is-approver`, {
                    withCredentials: true,
                });
                setIsApprover(response.data.isApprover);
            } catch (error) {
                console.error("Error checking approver status:", error);
            }
        };

        if (tokenExpiry && new Date(tokenExpiry) > new Date()) {
            setLoading(true);
            const provider = localStorage.getItem("loginMethod");
            fetch(`http://localhost:8589/kube-jit-api/${provider}/profile`, {
                credentials: "include",
            })
                .then((res) => {
                    if (res.ok) {
                        return res.json();
                    } else {
                        throw new Error("Not logged in");
                    }
                })
                .then((profileData) => {
                    if (profileData && profileData.name) {
                        setData({
                            userData: profileData,
                            expiresIn: 0, // unused in profile fetch
                        });
                        navigate(window.location.pathname); // Clear the URL parameters
                    } else {
                        console.error("Invalid profile data structure:", profileData);
                    }
                    setLoading(false);
                })
                .catch((error) => {
                    console.error("Error fetching profile data:", error);
                    setLoading(false);
                    setLogin(true);
                });
            fetchGroups();
            checkIsApprover(provider);
        } else {
            setLogin(true);
        }
    }, [navigate]);

    if (loading) {
        return (
            <div className="loader-container">
                <SyncLoader color="#0494ba" size={20} />
            </div>
        );
    }

    if (data && data.userData) {
        return (
            <div>
                <Profile user={data.userData} />
                <SyncLoader color="#0494ba" size={20} loading={loadingInCard} />
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
                                    />
                                )}
                                <HistoryTabPane
                                    activeTab={activeTab}
                                    originTab={originTab}
                                    userId={data.userData.id}
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
                    setData(data);
                    setLogin(false);
                }}
            />
        );
    }

    return null;
}

export default App;
