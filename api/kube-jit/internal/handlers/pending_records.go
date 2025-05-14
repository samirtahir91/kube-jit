package handlers

import (
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Query the database for pending requests for non-admin users
type PendingRequestRow struct {
	ID            uint      `json:"ID"`
	ClusterName   string    `json:"clusterName"`
	RoleName      string    `json:"roleName"`
	Status        string    `json:"status"`
	UserID        string    `json:"userID"`
	Users         []string  `gorm:"type:jsonb;serializer:json" json:"users"`
	Username      string    `json:"username"`
	Justification string    `json:"justification"`
	StartDate     time.Time `json:"startDate"`
	EndDate       time.Time `json:"endDate"`
	Namespace     string    `json:"namespace"`
	GroupID       string    `json:"groupID"`
	GroupName     string    `json:"groupName"`
	Approved      bool      `json:"approved"`
	CreatedAt     time.Time `json:"CreatedAt"`
}

type PendingRequest struct {
	ID            uint      `json:"ID"`
	ClusterName   string    `json:"clusterName"`
	RoleName      string    `json:"roleName"`
	Status        string    `json:"status"`
	UserID        string    `json:"userID"`
	Users         []string  `json:"users"`
	Username      string    `json:"username"`
	Justification string    `json:"justification"`
	StartDate     time.Time `json:"startDate"`
	EndDate       time.Time `json:"endDate"`
	Namespaces    []string  `json:"namespaces"`
	GroupIDs      []string  `json:"groupIDs"`
	GroupNames    []string  `json:"groupNames"`
	ApprovedList  []bool    `json:"approvedList"`
	CreatedAt     time.Time `json:"CreatedAt"`
}

// PendingApprovalsResponse is used for Swagger docs
type PendingApprovalsResponse struct {
	PendingRequests []PendingRequest `json:"pendingRequests"`
}

// GetPendingApprovals godoc
// @Summary Get pending JIT requests for approver groups
// @Description Returns the pending JIT requests for the authenticated user's approver groups.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags records
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Success 200 {object} handlers.PendingApprovalsResponse "List of pending requests"
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: no approver groups in session"
// @Failure 500 {object} models.SimpleMessageResponse "Failed to fetch pending requests"
// @Router /approvals [get]
func GetPendingApprovals(c *gin.Context) {
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	reqLogger.Debug("GetPendingApprovals: Got sessionData", zap.Any("sessionData", sessionData))

	isAdmin, isAdminOk := sessionData["isAdmin"].(bool)
	isPlatformApprover, isPlatformApproverOk := sessionData["isPlatformApprover"].(bool)
	reqLogger.Debug("GetPendingApprovals: Admin/PlatformApprover check", zap.Bool("isAdmin", isAdmin), zap.Bool("isAdminOk", isAdminOk), zap.Bool("isPlatformApprover", isPlatformApprover), zap.Bool("isPlatformApproverOk", isPlatformApproverOk))

	if (isAdminOk && isAdmin) || (isPlatformApproverOk && isPlatformApprover) {
		reqLogger.Debug("GetPendingApprovals: Admin or Platform Approver path")
		var pendingRequests []models.RequestData // This is a slice of models.RequestData

		if err := db.DB.
			Where("status = ?", "Requested").
			Find(&pendingRequests).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
			return
		}

		// Ensure pendingRequests is an empty slice if nil, before returning
		if pendingRequests == nil {
			pendingRequests = []models.RequestData{}
		}

		c.JSON(http.StatusOK, gin.H{"pendingRequests": pendingRequests})
		return
	}

	reqLogger.Debug("GetPendingApprovals: Non-admin path")
	rawApproverGroups, ok := sessionData["approverGroups"]
	reqLogger.Debug("GetPendingApprovals: ApproverGroups check", zap.Any("rawApproverGroups", rawApproverGroups), zap.Bool("ok", ok))

	if !ok {
		reqLogger.Info("GetPendingApprovals: No approver groups in session, returning 401")
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: no approver groups in session"})
		return
	}

	// Convert approverGroups to a slice of group IDs
	reqLogger.Debug("GetPendingApprovals: Processing approver groups")
	approverGroupIDs := []string{}
	if rawGroups, ok := rawApproverGroups.([]models.Team); ok {
		for _, group := range rawGroups {
			approverGroupIDs = append(approverGroupIDs, group.ID)
		}
	} else if rawGroups, ok := rawApproverGroups.([]any); ok {
		for _, group := range rawGroups {
			if groupMap, ok := group.(map[string]any); ok {
				if id, ok := groupMap["id"].(string); ok {
					approverGroupIDs = append(approverGroupIDs, id)
				}
			}
		}
	}

	if len(approverGroupIDs) == 0 {
		reqLogger.Debug("GetPendingApprovals: No approver groups in session, returning 401")
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: no approver groups in session"})
		return
	}

	var rows []PendingRequestRow

	if err := db.DB.
		Table("request_data").
		Select(
			"request_data.id, "+
				"request_data.cluster_name, "+
				"request_data.role_name, "+
				"request_data.user_id, "+
				"request_data.username, "+
				"request_data.justification, "+
				"request_data.start_date, "+
				"request_data.end_date, "+
				"request_data.created_at, "+
				"request_data.users, "+
				"request_namespaces.namespace, "+
				"request_namespaces.group_id, "+
				"request_namespaces.group_name, "+
				"request_namespaces.approved",
		).
		Joins("JOIN request_namespaces ON request_namespaces.request_id = request_data.id").
		Where("request_namespaces.group_id IN (?) AND request_data.status = ? AND request_namespaces.approved = false", approverGroupIDs, "Requested").
		Scan(&rows).Error; err != nil {
		reqLogger.Error("GetPendingApprovals: Error fetching pending requests", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	// Group by request ID
	grouped := map[uint]*PendingRequest{}
	for _, row := range rows {
		if _, exists := grouped[row.ID]; !exists {
			grouped[row.ID] = &PendingRequest{
				ID:            row.ID,
				ClusterName:   row.ClusterName,
				RoleName:      row.RoleName,
				Status:        row.Status,
				UserID:        row.UserID,
				Users:         row.Users,
				Username:      row.Username,
				Justification: row.Justification,
				StartDate:     row.StartDate,
				EndDate:       row.EndDate,
				CreatedAt:     row.CreatedAt,
				Namespaces:    []string{},
				GroupIDs:      []string{},
				GroupNames:    []string{},
				ApprovedList:  []bool{},
			}
		}
		grouped[row.ID].Namespaces = append(grouped[row.ID].Namespaces, row.Namespace)
		grouped[row.ID].GroupIDs = append(grouped[row.ID].GroupIDs, row.GroupID)
		grouped[row.ID].GroupNames = append(grouped[row.ID].GroupNames, row.GroupName)
		grouped[row.ID].ApprovedList = append(grouped[row.ID].ApprovedList, row.Approved)
	}

	var pendingRequests []PendingRequest
	for _, v := range grouped {
		pendingRequests = append(pendingRequests, *v)
	}

	// Sort pendingRequests by ID to ensure consistent order for the API response
	sort.Slice(pendingRequests, func(i, j int) bool {
		return pendingRequests[i].ID < pendingRequests[j].ID
	})

	if pendingRequests == nil {
		pendingRequests = []PendingRequest{}
	}

	reqLogger.Debug("GetPendingApprovals: Returning pending requests", zap.Int("count", len(pendingRequests)))
	c.JSON(http.StatusOK, gin.H{"pendingRequests": pendingRequests})
}
