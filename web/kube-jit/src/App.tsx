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

type ApiResponse = {
    userData: UserData;
    expiresIn: number;
};

type UserData = {
    avatar_url: string;
    login: string;
    id: string;
    type: string;
};

type Group = {
    id: number;
    name: string;
};

function App() {
    const urlParams = new URLSearchParams(window.location.search);
    const code = urlParams.get("code");
    const [data, setData] = useState<ApiResponse | null>(null);
    const [loading, setLoading] = useState(false);
	const [loadingInCard, setLoadingInCard] = useState(false);
	const [login, setLogin] = useState(false);
    const [activeTab, setActiveTab] = useState<string>('request');
    const [originTab, setOriginTab] = useState<string>('');
    const [approverGroups, setApproverGroups] = useState<Group[]>([]);
    const [isApprover, setIsApprover] = useState<boolean>(false);
    const navigate = useNavigate();

    useEffect(() => {
        const tokenExpiry = localStorage.getItem("tokenExpiry");

        const fetchGroups = async () => {
            try {
                const response = await axios.get('/kube-jit-api/approving-groups');
                setApproverGroups(response.data);
            } catch (error) {
                console.error('Error fetching approver groups:', error);
            }
        };

        const checkIsApprover = async () => {
            try {
                const response = await axios.get('/kube-jit-api/is-approver', { withCredentials: true });
                setIsApprover(response.data.isApprover);
            } catch (error) {
                console.error('Error checking approver status:', error);
            }
        };

        // generate random state for oauth
        function generateRandomState() {
            return Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15);
        }        

        if (code) {
            setLoading(true);
            const state = generateRandomState();
            axios.get(`/kube-jit-api/oauth/redirect`, {
                params: {
                    code: code,
                    state: state
                },
                withCredentials: true
            })
            .then((res) => {
                const data = res.data;
                if (data && data.userData) {
                    setData(data);
                    localStorage.setItem(
                        "tokenExpiry",
                        new Date(new Date().getTime() + data.expiresIn * 1000).toString()
                    );
                    // Clear the URL parameters
                    navigate(window.location.pathname);
                } else {
                    console.error("Invalid data structure:", data);
                }
                setLoading(false);
            })
            .catch((error) => {
                console.error("Error fetching data:", error);
                setLoading(false);
                setLogin(true);
            });
        } else if (tokenExpiry && new Date(tokenExpiry) > new Date()) {
            setLoading(true);
            fetch("/kube-jit-api/profile", {
                credentials: 'include'
            })
            .then((res) => {
                if (res.ok) {
                    return res.json();
                } else {
                    throw new Error('Not logged in');
                }
            })
            .then((profileData) => {
                if (profileData && profileData.login) {
                    setData({
                        userData: profileData,
                        expiresIn: 0 // unused in profile fetch
                    });
                    // Clear the URL parameters
                    navigate(window.location.pathname);
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
            checkIsApprover();
        } else {
            setLogin(true);
        }
    }, [code, navigate]);
        

	if (loading) {
		return (
			<div className="loader-container">
				<SyncLoader
					color="#0494ba"
					size={20}
				/>
			</div>
		);
	}


    if (data && data.userData) {
        return (
            <div>
                <Profile user={data.userData} />
				<SyncLoader
					color="#0494ba"
					size={20}
					loading={loadingInCard}
				/>
                <Card className="d-flex justify-content-center align-items-start">
                    <Card.Body className="container">
                        <Tab.Container id="left-tabs-example" activeKey={activeTab} onSelect={(selectedKey) => setActiveTab(selectedKey || 'request')}>
                            <Nav variant="tabs">
                                <Nav.Item>
                                    <Nav.Link href="#requestJit" eventKey="request">Request</Nav.Link>
                                </Nav.Item>
                                {isApprover && (
                                    <Nav.Item>
                                        <Nav.Link href="#approveJit" eventKey="approve">Approve</Nav.Link>
                                    </Nav.Item>
                                )}
                                <Nav.Item>
                                    <Nav.Link href="#jitRecords" eventKey="history">History</Nav.Link>
                                </Nav.Item>
                            </Nav>
                            <Tab.Content>
                                <RequestTabPane
                                    username={data.userData.login}
                                    userId={data.userData.id}
                                    approverGroups={approverGroups}
                                    setActiveTab={setActiveTab}
                                    setOriginTab={setOriginTab}
									setLoadingInCard={setLoadingInCard}
                                />
                                {isApprover && (
                                    <ApproveTabPane
                                        username={data.userData.login}
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
			<div>
				<Login />
			</div>
		);
	}
}

export default App;
