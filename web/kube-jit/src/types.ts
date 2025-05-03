export type Request = {
    id: number;
    userID: string;
    username: string;
    CreatedAt: string;
    updatedAt: string;
    startDate: string;
    endDate: string;
    deletedAt: string | null;
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
    id: number;
    userID: string;
    username: string;
    startDate: string;
    endDate: string;
    justification: string;
    clusterName: string;
    roleName: string;
    namespace: string;
    groupID: string; // Group ID for the namespace
    approved: boolean; // Approval status for the namespace
    users: string[];
    createdAt: string;
};