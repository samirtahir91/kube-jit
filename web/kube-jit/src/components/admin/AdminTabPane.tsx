import { useState } from "react";
import axios from "axios";
import { Modal, Button } from "react-bootstrap";
import config from "../../config/config";
import "./AdminTabPane.css";

const AdminTabPane = ({ setLoadingInCard }: { setLoadingInCard: (b: boolean) => void }) => {
    const [result, setResult] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [showConfirm, setShowConfirm] = useState(false);

    const handleCleanExpired = async () => {
        setLoadingInCard(true);
        try {
            const res = await axios.post(
                `${config.apiBaseUrl}/kube-jit-api/admin/clean-expired`,
                {},
                { withCredentials: true }
            );
            setResult(`${res.data.message}. Deleted: ${res.data.deleted}`);
            setTimeout(() => setResult(null), 4000); // Clear after 4 seconds
        } catch (err: any) {
            setError("Error cleaning expired requests.");
            setTimeout(() => setError(null), 5000);
        } finally {
            setLoadingInCard(false);
        }
    };

    return (
        <div className="admin-page-container">
            <div className="form-description">
                <h2 className="form-title">Admin Actions</h2>
                <p className="form-subtitle">
                    Perform administrative tasks such as cleaning up expired, non-approved requests.
                </p>
            </div>
            <button
                className="action-button reject mb-3"
                onClick={() => setShowConfirm(true)}
            >
                Clean Expired Non-Approved Requests
            </button>
            {result && (
                <div className="success-message">
                    <i className="bi bi-check-circle-fill me-2"></i>
                    {result}
                    <button className="success-message-close" onClick={() => setResult(null)}>
                        &times;
                    </button>
                </div>
            )}
            {error && (
                <div className="error-message">
                    <i className="bi bi-exclamation-circle-fill me-2"></i>
                    {error}
                    <button className="error-message-close" onClick={() => setError(null)}>
                        &times;
                    </button>
                </div>
            )}
            <Modal show={showConfirm} onHide={() => setShowConfirm(false)} centered>
                <Modal.Header closeButton>
                    <Modal.Title>Confirm Cleanup</Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    Are you sure you want to delete all expired, non-approved requests? This action cannot be undone.
                </Modal.Body>
                <Modal.Footer>
                    <Button variant="secondary" onClick={() => setShowConfirm(false)}>
                        Cancel
                    </Button>
                    <Button
                        variant="danger"
                        onClick={async () => {
                            setShowConfirm(false);
                            await handleCleanExpired();
                        }}
                    >
                        Yes, Clean Up
                    </Button>
                </Modal.Footer>
            </Modal>
        </div>
    );
};

export default AdminTabPane;