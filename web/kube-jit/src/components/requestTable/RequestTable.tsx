import React, { useState } from 'react';
import { Table } from 'react-bootstrap';
import './RequestTable.css';
import { PendingRequest, Request } from '../../types';


type RequestTableProps = {
    requests: Request[] | PendingRequest[];
    mode: 'pending' | 'history'; // Add mode to differentiate between tabs
    selectable: boolean;
    selectedRequests: number[];
    handleSelectRequest: (id: number) => void;
    variant: 'light' | 'dark';
    setVariant: (variant: 'light' | 'dark') => void;
};

const RequestTable: React.FC<RequestTableProps> = ({ mode, requests, selectable, selectedRequests, handleSelectRequest, variant, setVariant }) => {
    const [filters, setFilters] = useState({
        username: '',
        approvingTeamName: '',
        status: '',
        approverNames: '',
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
        if (mode === 'pending') {
            const pendingRequest = request as PendingRequest;
            return (
                (filters.username === '' || pendingRequest.username.toLowerCase().includes(filters.username.toLowerCase())) &&
                (filters.users === '' || pendingRequest.users.some(user => user.toLowerCase().includes(filters.users.toLowerCase()))) &&
                (filters.clusterName === '' || pendingRequest.clusterName.toLowerCase().includes(filters.clusterName.toLowerCase())) &&
                (filters.roleName === '' || pendingRequest.roleName.toLowerCase().includes(filters.roleName.toLowerCase())) &&
                (filters.namespaces === '' || (Array.isArray(pendingRequest.namespaces) && pendingRequest.namespaces.some(namespace => namespace.toLowerCase().includes(filters.namespaces.toLowerCase()))))
            );
        } else {
            const historicalRequest = request as Request;
            return (
                (filters.username === '' || historicalRequest.username.toLowerCase().includes(filters.username.toLowerCase())) &&
                (filters.status === '' || historicalRequest.status.toLowerCase().includes(filters.status.toLowerCase())) &&
                (filters.approverNames === '' || historicalRequest.approverNames.some(name => name.toLowerCase().includes(filters.approverNames.toLowerCase()))) &&
                (filters.users === '' || historicalRequest.users.some(user => user.toLowerCase().includes(filters.users.toLowerCase()))) &&
                (filters.clusterName === '' || historicalRequest.clusterName.toLowerCase().includes(filters.clusterName.toLowerCase())) &&
                (filters.roleName === '' || historicalRequest.roleName.toLowerCase().includes(filters.roleName.toLowerCase())) &&
                (filters.namespaces === '' || historicalRequest.namespaces.some(namespace => namespace.toLowerCase().includes(filters.namespaces.toLowerCase())))
            );
        }
    });

    const exportToCSV = () => {
        const headers = [
            'ID',
            'Username',
            'Approvers',
            'Users',
            'Cluster',
            'Namespaces',
            'Role',
            'Status',
            'Start Date',
            'End Date',
            'Justification',
            'Notes',
            'Namespace Approvals',
        ];

        const rows = filteredRequests.map(request => {
            if (mode === 'pending') {
                const pendingRequest = request as PendingRequest;
                return [
                    pendingRequest.ID,
                    pendingRequest.username,
                    '', // Approvers not relevant for pending
                    pendingRequest.users.join(', '),
                    pendingRequest.clusterName,
                    pendingRequest.namespaces,
                    pendingRequest.roleName,
                    new Date(pendingRequest.startDate).toLocaleString(),
                    new Date(pendingRequest.endDate).toLocaleString(),
                    pendingRequest.justification,
                    '', // Notes not relevant for pending
                    '', // Namespace Approvals not relevant for pending
                ];
            } else {
                const historicalRequest = request as Request;
                // Format namespace approvals as a string
                const nsApprovals = historicalRequest.namespaceApprovals && historicalRequest.namespaceApprovals.length > 0
                    ? historicalRequest.namespaceApprovals.map(ns =>
                        `${ns.namespace} (${ns.groupName}): ${ns.approved ? 'Approved' : 'Rejected'}${ns.approverName ? ` by ${ns.approverName}` : ''}`
                      ).join(' | ')
                    : 'N/A';
    
                return [
                    historicalRequest.ID,
                    historicalRequest.username,
                    historicalRequest.approverNames ? historicalRequest.approverNames.join(', ') : 'N/A',
                    historicalRequest.users.join(', '),
                    historicalRequest.clusterName,
                    historicalRequest.namespaces ? historicalRequest.namespaces.join(', ') : 'N/A',
                    historicalRequest.roleName,
                    historicalRequest.status,
                    new Date(historicalRequest.startDate).toLocaleString(),
                    new Date(historicalRequest.endDate).toLocaleString(),
                    historicalRequest.justification,
                    historicalRequest.notes,
                    nsApprovals,
                ];
            }
        });

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
        <>
            {/* Controls OUTSIDE the table-container */}
            <div className="py-5 table-controls d-flex justify-content-between align-items-center mb-3">
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
                                approverNames: '',
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
            <div className="results-message">
                Showing <strong>{filteredRequests.length}</strong> result{filteredRequests.length !== 1 ? 's' : ''}
            </div>
            {/* Table and results message */}
            <div className={`table-container ${isExpanded ? ' expanded' : ''}`}>
                <div className="table-outer-scroll-x">
                    <div className="table-inner-scroll-y">
                        <Table
                            variant={variant}
                            size="sm"
                            striped
                            bordered
                            hover
                            responsive={false}
                            className="mt-3"
                            style={{ marginBottom: 0, minWidth: 1200, width: '100%', tableLayout: 'fixed' }}
                        >
                            <thead>
                                <tr>
                                    {selectable && <th className="table-colour th-id">Select</th>}
                                    <th className="table-colour th-id">ID</th>
                                    <th className="table-colour th-date">Period requested</th>
                                    <th className="table-colour th-username">
                                        Requestee
                                        <input
                                            type="text"
                                            placeholder="Filter"
                                            className="form-control form-control-sm mt-1"
                                            value={filters.username}
                                            onChange={(e) => handleFilterChange(e, 'username')}
                                        />
                                    </th>
                                    {mode === 'history' && (
                                        <th className="table-colour th-approvers">
                                            Approvers
                                            <input
                                                type="text"
                                                placeholder="Filter"
                                                className="form-control form-control-sm mt-1"
                                                value={filters.approverNames}
                                                onChange={(e) => handleFilterChange(e, 'approverNames')}
                                            />
                                        </th>
                                    )}
                                    {mode === 'history' && (
                                        <th className="table-colour th-ns-approvals namespace-approvals-col">
                                            Namespace Approvals (with owning group name)
                                        </th>
                                    )}
                                    <th className="table-colour th-users">
                                        Users
                                        <input
                                            type="text"
                                            placeholder="Filter"
                                            className="form-control form-control-sm mt-1"
                                            value={filters.users}
                                            onChange={(e) => handleFilterChange(e, 'users')}
                                        />
                                    </th>
                                    <th className="table-colour th-cluster">
                                        Cluster
                                        <input
                                            type="text"
                                            placeholder="Filter"
                                            className="form-control form-control-sm mt-1"
                                            value={filters.clusterName}
                                            onChange={(e) => handleFilterChange(e, 'clusterName')}
                                        />
                                    </th>
                                    <th className="table-colour th-namespaces">
                                        Namespaces
                                        <input
                                            type="text"
                                            placeholder="Filter"
                                            className="form-control form-control-sm mt-1"
                                            value={filters.namespaces}
                                            onChange={(e) => handleFilterChange(e, 'namespaces')}
                                        />
                                    </th>
                                    <th className="table-colour th-just">Justification</th>
                                    <th className="table-colour th-role">
                                        Role
                                        <input
                                            type="text"
                                            placeholder="Filter"
                                            className="form-control form-control-sm mt-1"
                                            value={filters.roleName}
                                            onChange={(e) => handleFilterChange(e, 'roleName')}
                                        />
                                    </th>
                                    <th className="table-colour th-date">Created At</th>
                                    {mode === 'history' && (
                                        <th className="table-colour th-status">
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
                                    )}
                                    {mode === 'history' && (
                                        <th className="table-colour th-notes">Notes</th>
                                    )}
                                </tr>
                            </thead>
                            <tbody>
                                {filteredRequests.map(request => {
                                    if (mode === 'pending') {
                                        const pendingRequest = request as PendingRequest;
                                        return (
                                            <tr key={pendingRequest.ID}>
                                                {selectable && (
                                                    <td>
                                                        <label className="select-checkbox-label">
                                                            <input
                                                                type="checkbox"
                                                                checked={selectedRequests.includes(pendingRequest.ID)}
                                                                onChange={() => handleSelectRequest(pendingRequest.ID)}
                                                                className="select-checkbox"
                                                            />
                                                            <span className="custom-checkbox"></span>
                                                        </label>
                                                    </td>
                                                )}
                                                <td>{pendingRequest.ID}</td>
                                                <td>
                                                    {new Date(pendingRequest.startDate).toLocaleString(undefined, {
                                                        year: 'numeric',
                                                        month: 'numeric',
                                                        day: 'numeric',
                                                        hour: 'numeric',
                                                        minute: 'numeric',
                                                    })}{' '}
                                                    -{' '}
                                                    {new Date(pendingRequest.endDate).toLocaleString(undefined, {
                                                        year: 'numeric',
                                                        month: 'numeric',
                                                        day: 'numeric',
                                                        hour: 'numeric',
                                                        minute: 'numeric',
                                                    })}
                                                </td>
                                                <td>{pendingRequest.username}</td>
                                                <td>{pendingRequest.users.join(', ')}</td>
                                                <td>{pendingRequest.clusterName}</td>
                                                <td>{Array.isArray(pendingRequest.namespaces) ? pendingRequest.namespaces.join(', ') : pendingRequest.namespaces}</td>
                                                <td>{pendingRequest.justification}</td>
                                                <td>{pendingRequest.roleName}</td>
                                                <td>
                                                    {new Date(pendingRequest.CreatedAt).toLocaleString(undefined, {
                                                            year: 'numeric',
                                                            month: 'numeric',
                                                            day: 'numeric',
                                                            hour: 'numeric',
                                                            minute: 'numeric',
                                                    })}
                                                </td>
                                            </tr>
                                        );
                                    } else {
                                        const historicalRequest = request as Request;
                                        return (
                                            <tr key={historicalRequest.ID}>
                                                <td>{historicalRequest.ID}</td>
                                                <td>
                                                    {new Date(historicalRequest.startDate).toLocaleString(undefined, {
                                                        year: 'numeric',
                                                        month: 'numeric',
                                                        day: 'numeric',
                                                        hour: 'numeric',
                                                        minute: 'numeric',
                                                    })}{' '}
                                                    -{' '}
                                                    {new Date(historicalRequest.endDate).toLocaleString(undefined, {
                                                        year: 'numeric',
                                                        month: 'numeric',
                                                        day: 'numeric',
                                                        hour: 'numeric',
                                                        minute: 'numeric',
                                                    })}
                                                </td>
                                                <td>{historicalRequest.username}</td>
                                                <td>{historicalRequest.approverNames ? historicalRequest.approverNames.join(', ') : 'N/A'}</td>
                                                {mode === 'history' && (
                                                    <td className="namespace-approvals-col">
                                                        {historicalRequest.namespaceApprovals && historicalRequest.namespaceApprovals.length > 0 ? (
                                                            <ul style={{ paddingLeft: 16, marginBottom: 0 }}>
                                                                {historicalRequest.namespaceApprovals.map((ns, idx) => {
                                                                    let statusIcon = '🟡'; // Default: pending
                                                                    if (ns.approved === true && ns.approverName) statusIcon = '✅';
                                                                    else if (ns.approved === false && ns.approverName) statusIcon = '❌';
                                                                    return (
                                                                        <li key={idx}>
                                                                            <strong>{ns.namespace}</strong>
                                                                            {' '}(<span style={{ color: '#888' }}>{ns.groupName}</span>)
                                                                            :
                                                                            {ns.approverName ? (
                                                                                <div style={{ display: 'block', marginLeft: 0 }}>
                                                                                    {ns.approverName} {statusIcon}
                                                                                </div>
                                                                            ) : (
                                                                                <div style={{ display: 'block', marginLeft: 0 }}>
                                                                                    Pending {statusIcon}
                                                                                </div>
                                                                            )}
                                                                        </li>
                                                                    );
                                                                })}
                                                            </ul>
                                                        ) : (
                                                            'N/A'
                                                        )}
                                                    </td>
                                                )}
                                                <td>{historicalRequest.users.join(', ')}</td>
                                                <td>{historicalRequest.clusterName}</td>
                                                <td>{historicalRequest.namespaces ? historicalRequest.namespaces.join(', ') : 'N/A'}</td>
                                                <td>{historicalRequest.justification}</td>
                                                <td>{historicalRequest.roleName}</td>
                                                <td>{new Date(historicalRequest.CreatedAt).toLocaleString(undefined, {
                                                            year: 'numeric',
                                                            month: 'numeric',
                                                            day: 'numeric',
                                                            hour: 'numeric',
                                                            minute: 'numeric',
                                                    })}
                                                </td>
                                                <td>{historicalRequest.status}</td>
                                                <td>{historicalRequest.notes}</td>
                                            </tr>
                                        );
                                    }
                                })}
                            </tbody>
                        </Table>
                    </div>
                </div>
            </div>
        </>
    );
};

export default RequestTable;
