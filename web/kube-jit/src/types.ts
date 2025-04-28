export type Request = {
    ID: number;
    userID: string;
    username: string;
    CreatedAt: string;
    UpdatedAt: string;
    startDate: string;
    endDate: string;
    DeletedAt: string | null;
    approvingTeamID: string; // Use string for consistency
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

export type UserData = {
	avatar_url: string;
	name: string;
	id: string;
	provider: string;
	email: string
};