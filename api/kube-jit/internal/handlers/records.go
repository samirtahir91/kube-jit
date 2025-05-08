package handlers

import (
	"fmt"
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"net/http"
	"strconv"
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

// GetRecords godoc
// @Summary Get JIT requests for a user
// @Description Returns the latest JIT requests for a user with optional limit and date range.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies:
// @Description Login required to test via browser, else test via curl
// @Tags records
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Param   userID     query    string  false  "User ID"
// @Param   username   query    string  false  "Username"
// @Param   limit      query    int     false  "Limit"
// @Success 200 {array} models.RequestWithNamespaceApprovers
// @Failure 500 {object} models.SimpleMessageResponse
// @Router /kube-jit-api/history [get]
func GetRecords(c *gin.Context) {
	// Check if the user is logged in and get logger
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	isAdmin, _ := sessionData["isAdmin"].(bool)
	isPlatformApprover, _ := sessionData["isPlatformApprover"].(bool)
	userID := c.Query("userID")
	username := c.Query("username")
	limit := c.Query("limit")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt <= 0 {
		limitInt = 1
	}

	var requests []models.RequestData
	query := db.DB.Order("created_at desc").Limit(limitInt)
	if isAdmin || isPlatformApprover {
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if username != "" {
			query = query.Where("username = ?", username)
		}
	} else {
		if userID != "" {
			query = query.Where("user_id = ? OR approver_ids @> ?", userID, fmt.Sprintf(`["%s"]`, userID))
		} else if username != "" {
			query = query.Where("username = ? OR approver_names @> ?", username, fmt.Sprintf(`["%s"]`, username))
		}
	}
	if startDate != "" && endDate != "" {
		query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
	}
	if err := query.Find(&requests).Error; err != nil {
		reqLogger.Error("Error fetching records in GetRecords", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch records"})
		return
	}

	var enriched []models.RequestWithNamespaceApprovers
	for _, req := range requests {
		var nsApprovals []models.NamespaceApprovalInfo
		if err := db.DB.
			Table("request_namespaces").
			Select("namespace, group_name, group_id, approved, approver_id, approver_name").
			Where("request_id = ?", req.ID).
			Scan(&nsApprovals).Error; err != nil {
			reqLogger.Error("Error fetching namespace approvals in GetRecords", zap.Uint("requestID", req.ID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch namespace approvals"})
			return
		}
		enriched = append(enriched, models.RequestWithNamespaceApprovers{
			RequestData:        req,
			NamespaceApprovals: nsApprovals,
		})
	}

	c.JSON(http.StatusOK, enriched)
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
// @Router /kube-jit-api/approvals [get]
func GetPendingApprovals(c *gin.Context) {
	// Check if the user is logged in
	sessionData := GetSessionData(c)

	// Check if the user is an admin or platform approver
	isAdmin, isAdminOk := sessionData["isAdmin"].(bool)
	isPlatformApprover, isPlatformApproverOk := sessionData["isPlatformApprover"].(bool)
	if (isAdminOk && isAdmin) || (isPlatformApproverOk && isPlatformApprover) {
		// If the user is an admin or platform approver, return all pending requests
		var pendingRequests []models.RequestData

		// Fetch all pending requests
		if err := db.DB.
			Where("status = ?", "Requested").
			Find(&pendingRequests).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pendingRequests": pendingRequests})
		return
	}

	// Retrieve approverGroups from the session for non-admin users
	rawApproverGroups, ok := sessionData["approverGroups"]
	if !ok {
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: no approver groups in session"})
		return
	}

	// Convert approverGroups to a slice of group IDs
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
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	// Group by request ID
	grouped := map[uint]*PendingRequest{}
	for _, row := range rows {
		req, exists := grouped[row.ID]
		if !exists {
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
			req = grouped[row.ID]
		}
		req.Namespaces = append(req.Namespaces, row.Namespace)
		req.GroupIDs = append(req.GroupIDs, row.GroupID)
		req.GroupNames = append(req.GroupNames, row.GroupName)
		req.ApprovedList = append(req.ApprovedList, row.Approved)
	}

	var pendingRequests []PendingRequest
	for _, v := range grouped {
		pendingRequests = append(pendingRequests, *v)
	}

	c.JSON(http.StatusOK, gin.H{"pendingRequests": pendingRequests})
}
