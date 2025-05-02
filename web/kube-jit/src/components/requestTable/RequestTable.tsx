import React, { useState } from 'react';
import { Table, Button } from 'react-bootstrap';
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
    approvingTeamID: string;
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
    setVariant: (variant: 'light' | 'dark') => void; // Add a setter for the variant
};

const RequestTable: React.FC<RequestTableProps> = ({ requests, selectable, selectedRequests, handleSelectRequest, variant, setVariant }) => {
    const [filters, setFilters] = useState({
        username: '',
        approvingTeamName: '',
        status: '',
        approverName: '',
        users: '',
        clusterName: '',
        namespaces: '',
        roleName: '',
    });

    const [isExpanded, setIsExpanded] = useState(false); // State to track expanded view

    const handleExpandToggle = () => {
        setIsExpanded(!isExpanded); // Toggle expanded state
    };

    const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>, column: string) => {
        setFilters({ ...filters, [column]: e.target.value });
    };

    const filteredRequests = requests.filter(request => {
        return (
            (filters.username === '' || request.username.toLowerCase().includes(filters.username.toLowerCase())) &&
            (filters.approvingTeamName === '' || request.approvingTeamName.toLowerCase().includes(filters.approvingTeamName.toLowerCase())) &&
            (filters.status === '' || request.status.toLowerCase().includes(filters.status.toLowerCase())) &&
            (filters.approverName === '' || request.approverName.toLowerCase().includes(filters.approverName.toLowerCase())) &&
            (filters.users === '' || request.users.some(user => user.toLowerCase().includes(filters.users.toLowerCase()))) &&
            (filters.clusterName === '' || request.clusterName.toLowerCase().includes(filters.clusterName.toLowerCase())) &&
            (filters.namespaces === '' || request.namespaces.some(namespace => namespace.toLowerCase().includes(filters.namespaces.toLowerCase()))) &&
            (filters.roleName === '' || request.roleName.toLowerCase().includes(filters.roleName.toLowerCase()))
        );
    });

    const exportToCSV = () => {
        const headers = [
            'ID',
            'Username',
            'Approving Team',
            'Approver',
            'Users',
            'Cluster',
            'Namespaces',
            'Role',
            'Status',
            'Start Date',
            'End Date',
            'Justification',
            'Notes',
        ];

        const rows = filteredRequests.map(request => [
            request.ID,
            request.username,
            request.approvingTeamName,
            request.approverName,
            request.users.join(', '),
            request.clusterName,
            request.namespaces.join(', '),
            request.roleName,
            request.status,
            new Date(request.startDate).toLocaleString(),
            new Date(request.endDate).toLocaleString(),
            request.justification,
            request.notes,
        ]);

        const csvContent = [headers, ...rows]
            .map(row => row.map(cell => `"${cell}"`).join(',')) // Escape cells with quotes
            .join('\n');

        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.setAttribute('download', 'requests.csv');
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    };

    return (
        <div className={`table-container py-5 ${isExpanded ? 'expanded' : ''}`}>
            <div className="d-flex justify-content-between align-items-center mb-3">
                <div className="d-flex align-items-center">
                    <div
                        className={`toggle-button ${variant === 'dark' ? 'dark' : ''}`}
                        onClick={() => setVariant(variant === 'light' ? 'dark' : 'light')}
                    >
                        <div className="toggle-circle"></div>
                    </div>
                    <span className="ms-2">{variant === 'dark' ? 'Dark Mode' : 'Light Mode'}</span>
                </div>
                <div>
                    <button
                        className="action-button clear-filters-button me-2"
                        onClick={() =>
                            setFilters({
                                username: '',
                                approvingTeamName: '',
                                status: '',
                                approverName: '',
                                users: '',
                                clusterName: '',
                                namespaces: '',
                                roleName: '',
                            })
                        }
                    >
                        Clear Filters
                    </button>
                    <button
                        className="action-button export-csv-button me-2"
                        onClick={exportToCSV}
                    >
                        Export to CSV
                    </button>
                    <button
                        className="action-button expand-view-button"
                        onClick={handleExpandToggle}
                    >
                        {isExpanded ? 'Collapse View' : 'Expand View'}
                    </button>
                </div>
            </div>
            {/* Display the number of results */}
            <div className="results-message">
                Showing <strong>{filteredRequests.length}</strong> result{filteredRequests.length !== 1 ? 's' : ''}
            </div>
            <Table variant={variant} size="sm" striped bordered hover responsive className="mt-3">
                <thead>
                    <tr>
                        {selectable && <th className="table-colour">Select</th>}
                        <th className="table-colour">ID</th>
                        <th className="table-colour">Period requested</th>
                        <th className="table-colour">
                            Requestee
                            <input
                                type="text"
                                placeholder="Filter"
                                className="form-control form-control-sm mt-1"
                                value={filters.username}
                                onChange={(e) => handleFilterChange(e, 'username')}
                            />
                        </th>
                        <th className="table-colour">
                            Approving Team
                            <input
                                type="text"
                                placeholder="Filter"
                                className="form-control form-control-sm mt-1"
                                value={filters.approvingTeamName}
                                onChange={(e) => handleFilterChange(e, 'approvingTeamName')}
                            />
                        </th>
                        <th className="table-colour">
                            Approver
                            <input
                                type="text"
                                placeholder="Filter"
                                className="form-control form-control-sm mt-1"
                                value={filters.approverName}
                                onChange={(e) => handleFilterChange(e, 'approverName')}
                            />
                        </th>
                        <th className="table-colour">
                            Users
                            <input
                                type="text"
                                placeholder="Filter"
                                className="form-control form-control-sm mt-1"
                                value={filters.users}
                                onChange={(e) => handleFilterChange(e, 'users')}
                            />
                        </th>
                        <th className="table-colour">
                            Cluster
                            <input
                                type="text"
                                placeholder="Filter"
                                className="form-control form-control-sm mt-1"
                                value={filters.clusterName}
                                onChange={(e) => handleFilterChange(e, 'clusterName')}
                            />
                        </th>
                        <th className="table-colour">
                            Namespaces
                            <input
                                type="text"
                                placeholder="Filter"
                                className="form-control form-control-sm mt-1"
                                value={filters.namespaces}
                                onChange={(e) => handleFilterChange(e, 'namespaces')}
                            />
                        </th>
                        <th className="table-colour">Justification</th>
                        <th className="table-colour">
                            Role
                            <input
                                type="text"
                                placeholder="Filter"
                                className="form-control form-control-sm mt-1"
                                value={filters.roleName}
                                onChange={(e) => handleFilterChange(e, 'roleName')}
                            />
                        </th>
                        <th className="table-colour">Created At</th>
                        <th className="table-colour">
                            Status
                            <select
                                className="form-select form-select-sm mt-1"
                                value={filters.status}
                                onChange={(e) => handleFilterChange(e, 'status')}
                            >
                                <option value="">All</option>
                                <option value="Requested">Requested</option>
                                <option value="Approved">Approved</option>
                                <option value="Rejected">Rejected</option>
                                <option value="Pending">Pending</option>
                                <option value="Succeeded">Succeeded</option>
                            </select>
                        </th>
                        <th className="table-colour">Notes</th>
                    </tr>
                </thead>
                <tbody>
                    {filteredRequests.map(request => (
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
                            <td>{request.users.join(', ')}</td>
                            <td>{request.clusterName}</td>
                            <td>{request.namespaces.join(', ')}</td>
                            <td>{request.justification}</td>
                            <td>{request.roleName}</td>
                            <td>{new Date(request.CreatedAt).toLocaleString()}</td>
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
