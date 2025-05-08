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

// GetRecords returns the latest jit requests for a user with optional limit and date range
// It fetches the records from the database and returns them as JSON
// It checks if the user is an admin or platform approver to determine the query parameters
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch records"})
		return
	}

	type NamespaceApprovalInfo struct {
		Namespace    string `json:"namespace"`
		GroupID      string `json:"groupID"`
		GroupName    string `json:"groupName"`
		Approved     bool   `json:"approved"`
		ApproverID   string `json:"approverID"`
		ApproverName string `json:"approverName"`
	}
	type RequestWithNamespaceApprovers struct {
		models.RequestData
		NamespaceApprovals []NamespaceApprovalInfo `json:"namespaceApprovals"`
	}

	var enriched []RequestWithNamespaceApprovers
	for _, req := range requests {
		var nsApprovals []NamespaceApprovalInfo
		if err := db.DB.
			Table("request_namespaces").
			Select("namespace, group_name, group_id, approved, approver_id, approver_name").
			Where("request_id = ?", req.ID).
			Scan(&nsApprovals).Error; err != nil {
			reqLogger.Error("Error fetching namespace approvals in GetRecords", zap.Uint("requestID", req.ID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch namespace approvals"})
			return
		}
		enriched = append(enriched, RequestWithNamespaceApprovers{
			RequestData:        req,
			NamespaceApprovals: nsApprovals,
		})
	}

	c.JSON(http.StatusOK, enriched)
}

// GetPendingApprovals returns the pending requests for the user based on their approver groups
// It uses the session to retrieve the user's approver groups
// It queries the database for requests with status "Requested" and matching approver groups
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pendingRequests": pendingRequests})
		return
	}

	// Retrieve approverGroups from the session for non-admin users
	rawApproverGroups, ok := sessionData["approverGroups"]
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no approver groups in session"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no approver groups in session"})
		return
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
