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
    approverIDs: string[];
    approverNames: string[];
    notes: string;
    namespaceApprovals?: NamespaceApprovalInfo[];
};

export type ApiResponse = {
    userData: UserData;
    expiresIn: number;
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
    namespaces: string[];
    groupIDs: string[];
    approvedList: boolean[];
    CreatedAt: string;
};

export type NamespaceApprovalInfo = {
    namespace: string;
    groupID: string;
    groupName: string;
    approved: boolean;
    approverID: string;
    approverName: string;
};