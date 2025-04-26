import React from 'react';
import { Table } from 'react-bootstrap';
import './RequestTable.css';

type Request = {
    ID: number;
    userID: string;
    username: string;
    CreatedAt: string;
    UpdatedAt: string;
    startDate: string;
    endDate: string;
    DeletedAt: string | null;
    approvingTeamID: number;
    users: string[];
    namespaces: string[];
    justification: string;
    approvingTeamName: string;
    clusterName: string;
    roleName: string;
    status: string;
    approverID: number;
    approverName: string;
    notes: string;
};

type RequestTableProps = {
    requests: Request[];
    selectable: boolean;
    selectedRequests: number[];
    handleSelectRequest: (id: number) => void;
    variant: 'light' | 'dark';
};

const RequestTable: React.FC<RequestTableProps> = ({ requests, selectable, selectedRequests, handleSelectRequest, variant }) => {
    return (
        <div className="table-container">
            <Table variant={variant} size="sm" striped bordered hover responsive className="mt-3">
                <thead>
                    <tr>
                        {selectable && <th className="table-colour">Select</th>}
                        <th className="table-colour">ID</th>
                        <th className="table-colour">Period requested</th>
                        <th className="table-colour">Requestee</th>
                        <th className="table-colour">Approving Team</th>
                        <th className="table-colour">Approver</th>
                        <th className="table-colour">Users</th>
                        <th className="table-colour">Cluster</th>
                        <th className="table-colour">Namespaces</th>
                        <th className="table-colour">Justification</th>
                        <th className="table-colour">Role</th>
                        <th className="table-colour">Created At</th>
                        <th className="table-colour">Status</th>
                        <th className="table-colour">Notes</th>
                    </tr>
                </thead>
                <tbody>
                    {requests.map(request => (
                        <tr key={request.ID}>
                            {selectable && (
                                <td>
                                    <input
                                        type="checkbox"
                                        checked={selectedRequests.includes(request.ID)}
                                        onChange={() => handleSelectRequest(request.ID)}
                                    />
                                </td>
                            )}
                            <td>{request.ID}</td>
                            <td>
                                {new Date(request.startDate).toLocaleString(undefined, { year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric' })} -{' '}
                                {new Date(request.endDate).toLocaleString(undefined, { year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric' })}
                            </td>
                            <td>{request.username}</td>
                            <td>{request.approvingTeamName}</td>
                            <td>{request.approverName}</td>
                            <td>
                                {request.users.map(user => (
                                    <div key={user}>{user}</div>
                                ))}
                            </td>
                            <td>{request.clusterName}</td>
                            <td>
                                {request.namespaces.map(namespace => (
                                    <div key={namespace}>{namespace}</div>
                                ))}
                            </td>
                            <td>{request.justification}</td>
                            <td>{request.roleName}</td>
                            <td>{new Date(request.CreatedAt).toLocaleString(undefined, { year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric' })}</td>
                            <td>{request.status}</td>
                            <td>{request.notes}</td>
                        </tr>
                    ))}
                </tbody>
            </Table>
        </div>
    );
};

export default RequestTable;
