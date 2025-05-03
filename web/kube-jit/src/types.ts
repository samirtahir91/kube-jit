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
    clusterName: string;
    roleName: string;
    status: string;
    userID: string;
    users: string[];
    username: string;
    justification: string;
    startDate: string;
    endDate: string;
    namespaces: string[]; // <-- now an array
    groupIDs: string[];   // <-- now an array
    approvedList: boolean[];
    CreatedAt: string;
};