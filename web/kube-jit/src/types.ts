export type Request = {
    ID: number;
    userID: string;
    username: string;
    CreatedAt: string;
    UpdatedAt: string;
    startDate: string;
    endDate: string;
    DeletedAt: string | null;
    users: string[];
    namespaces: Namespace[];
    justification: string;
    clusterName: string;
    roleName: string;
    status: string;
    approverID: number;
    approverName: string;
    notes: string;
};

export type UserData = {
	avatar_url: string;
	name: string;
	id: string;
	provider: string;
	email: string
};

type Namespace = {
    namespace: string;
    groupID: string;
    approved: boolean;
};

export type PendingRequest = {
    ID: number;
    userID: string;
    username: string;
    CreatedAt: string;
    UpdatedAt: string;
    startDate: string;
    endDate: string;
    justification: string;
    clusterName: string;
    roleName: string;
    status: string;
    namespaces: string; // Single namespace
    groupID: string; // Group ID for the namespace
    approved: boolean; // Approval status for the namespace
    approverName: string;
    users: string[];
    notes: string;
};