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
    namespaces: string[];
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

export type PendingRequest = {
    ID: number;
    userID: string;
    username: string;
    startDate: string;
    endDate: string;
    justification: string;
    clusterName: string;
    roleName: string;
    namespaces: string[] | string; // Can be a single string or an array of strings
    groupID: string; // Group ID for the namespace
    approved: boolean; // Approval status for the namespace
    users: string[];
    CreatedAt: string;
};