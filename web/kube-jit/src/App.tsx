import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import "./App.css";
import Profile from "./components/profile/Profile";
import RequestTabPane from "./components/request/RequestTabPane";
import GitHubLogin from "./components/login/GitHubLogin";
import { Card, Nav, Tab } from "react-bootstrap";
import ApproveTabPane from "./components/approve/ApproveTabPane";
import HistoryTabPane from "./components/history/HistoryTabPane";
import axios from "axios";
import { SyncLoader } from "react-spinners";
import { GoogleLogin } from 'react-google-login';

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
    const [googleClientId, setGoogleClientId] = useState<string | null>(null);
    const [data, setData] = useState<ApiResponse | null>(null);
    const [loading, setLoading] = useState(false);
    const [loadingInCard, setLoadingInCard] = useState(false);
    const [login, setLogin] = useState(false);
    const [activeTab, setActiveTab] = useState<string>('request');
    const [originTab, setOriginTab] = useState<string>('');
    const [approverGroups, setApproverGroups] = useState<Group[]>([]);
    const [isApprover, setIsApprover] = useState<boolean>(false);
    const [loginMethod, setLoginMethod] = useState<'github' | 'google'>('github');
    const navigate = useNavigate();

    // Fetch Google client ID only when loginMethod is 'google'
    useEffect(() => {
        if (loginMethod === 'google' && !googleClientId) {
            axios.get('/kube-jit-api/oauth/google/client_id')
                .then((res) => {
                    setGoogleClientId(res.data.clientId);
                })
                .catch((error) => {
                    console.error('Error fetching Google client ID:', error);
                });
        }
    }, [loginMethod, googleClientId]);

    const handleGoogleLoginSuccess = (response: any) => {
        console.log('Google Login Success:', response);

        // Send the token to your backend for verification
        axios.post('/kube-jit-api/oauth/google/callback', { token: response.tokenId }, { withCredentials: true })
            .then((res) => {
                const data = res.data;
                if (data && data.userData) {
                    setData(data);
                    localStorage.setItem(
                        "tokenExpiry",
                        new Date(new Date().getTime() + data.expiresIn * 1000).toString()
                    );
                    navigate(window.location.pathname); // Clear the URL parameters
                } else {
                    console.error("Invalid response data:", data);
                }
            })
            .catch((error) => {
                console.error("Error during Google login:", error);
            });
    };

    const handleGoogleLoginFailure = (response: any) => {
        console.error('Google Login Failed:', response);
        alert('Google login failed. Please try again.');
    };

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

        if (code) {
            setLoading(true);
            axios.get(`/kube-jit-api/oauth/redirect`, {
                params: { code: code, provider: loginMethod },
                withCredentials: true,
            })
            .then((res) => {
                const data = res.data;
                if (data && data.userData) {
                    setData(data);
                    localStorage.setItem(
                        "tokenExpiry",
                        new Date(new Date().getTime() + data.expiresIn * 1000).toString()
                    );
                    navigate(window.location.pathname); // Clear the URL parameters
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
                credentials: 'include',
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
            checkIsApprover();
        } else {
            setLogin(true);
        }
    }, [code, navigate, loginMethod]);
        

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
                <button onClick={() => setLoginMethod('github')}>Login with GitHub</button>
                <button onClick={() => setLoginMethod('google')}>Login with Google</button>

                {loginMethod === 'github' && <GitHubLogin />}
                {loginMethod === 'google' && googleClientId && (
                    <GoogleLogin
                        clientId={googleClientId}
                        buttonText="Login with Google"
                        onSuccess={handleGoogleLoginSuccess}
                        onFailure={handleGoogleLoginFailure}
                        cookiePolicy={'single_host_origin'}
                    />
                )}
                {loginMethod === 'google' && !googleClientId && (
                    <div>Loading Google Login...</div>
                )}
			</div>
		);
	}
}

export default App;
