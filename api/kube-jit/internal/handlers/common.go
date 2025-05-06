package handlers

import (
	"fmt"
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"kube-jit/pkg/email"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/sessioncookie"
	"kube-jit/pkg/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	oauthProvider = utils.MustGetEnv("OAUTH_PROVIDER")
	clientID      = utils.MustGetEnv("OAUTH_CLIENT_ID")
	clientSecret  = utils.MustGetEnv("OAUTH_CLIENT_SECRET")
	redirectUri   = utils.MustGetEnv("OAUTH_REDIRECT_URI")
	adminEmail    string
	httpClient    = &http.Client{
		Timeout: 60 * time.Second,
	}
)

func init() {
	// Set the admin email for Google OAuth provider
	// Required for Domain-Wide Delegation and user impersonation vi GSA/Workload Identity
	if oauthProvider == "google" {
		adminEmail = utils.MustGetEnv("GOOGLE_ADMIN_EMAIL")
	}
}

// Logout clears all session cookies with the sessionPrefix
func Logout(c *gin.Context) {
	// Iterate through cookies with the session prefix
	for i := 0; ; i++ {
		cookieName := fmt.Sprintf("%s%d", sessioncookie.SessionPrefix, i)
		_, err := c.Cookie(cookieName)
		if err != nil {
			break // Stop when no more cookies are found
		}

		// Clear the cookie by setting its MaxAge to -1
		c.SetCookie(cookieName, "", -1, "/", "", true, true)
	}

	// Respond with a success message
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// checkLoggedIn verifies if the user is logged in by checking session data.
// Returns the session data if valid, or sends an unauthorized response and aborts the request.
func checkLoggedIn(c *gin.Context) (map[string]interface{}, bool) {
	session := sessions.Default(c)
	combinedData := session.Get("data")
	if combinedData == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no session data in cookies"})
		c.Abort() // Stop further processing of the request
		return nil, false
	}

	// Ensure the session data is a map[string]interface{}
	sessionData, ok := combinedData.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid session data format"})
		c.Abort() // Stop further processing of the request
		return nil, false
	}

	return sessionData, true
}

// K8sCallback is used by downstream controller to callback for status update
// It validates the signed URL and updates the request status in the database
// It also processes the callback data and returns a success message
// It is called by the K8s controller when the request is completed
// It is used to update the status of the request in the database
func K8sCallback(c *gin.Context) {
	var callbackData struct {
		TicketID string `json:"ticketID"`
		Status   string `json:"status"`
		Message  string `json:"message"`
	}

	if err := c.ShouldBindJSON(&callbackData); err != nil {
		logger.Warn("Failed to bind JSON in K8sCallback", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	callbackURL := c.Request.URL
	if !utils.ValidateSignedURL(callbackURL) {
		logger.Warn("Invalid or expired signed URL in K8sCallback")
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	if err := db.DB.Model(&models.RequestData{}).Where("id = ?", callbackData.TicketID).Updates(map[string]interface{}{
		"status": callbackData.Status,
		"notes":  callbackData.Message,
	}).Error; err != nil {
		logger.Error("Error updating request in K8sCallback",
			zap.String("ticketID", callbackData.TicketID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request (database error)"})
		return
	}

	logger.Info("Received callback for ticket",
		zap.String("ticketID", callbackData.TicketID),
		zap.String("status", callbackData.Status),
	)

	// Send status change email to user
	var req models.RequestData
	if err := db.DB.Where("id = ?", callbackData.TicketID).First(&req).Error; err == nil && req.Email != "" {
		body := email.BuildRequestEmail(email.EmailRequestDetails{
			Username:      req.Username,
			ClusterName:   req.ClusterName,
			Namespaces:    req.Namespaces,
			RoleName:      req.RoleName,
			Justification: req.Justification,
			StartDate:     req.StartDate,
			EndDate:       req.EndDate,
			Status:        callbackData.Status,
			Message:       callbackData.Message,
		})
		go func() {
			if err := email.SendMail(req.Email, fmt.Sprintf("Your JIT request #%s is now %s", callbackData.TicketID, callbackData.Status), body); err != nil {
				logger.Warn("Failed to send status change email (K8sCallback)", zap.String("email", req.Email), zap.Error(err))
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{"message": "Success"})
}

// ApproveOrRejectRequests approves pending requests in db - status = Approved
// or rejects them - status = Rejected
// It creates the k8s object for each request if status is Approved
// It updates the status of the requests in the database
func ApproveOrRejectRequests(c *gin.Context) {
	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}

	// Check if the user is an admin or platform approver
	isAdmin, _ := sessionData["isAdmin"].(bool)
	isPlatformApprover, _ := sessionData["isPlatformApprover"].(bool)

	var approverGroups []string
	if !isAdmin && !isPlatformApprover {
		// Only non-admins need approverGroups
		rawApproverGroups, ok := sessionData["approverGroups"]
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no approver groups in session"})
			return
		}
		if rawGroups, ok := rawApproverGroups.([]any); ok {
			for _, group := range rawGroups {
				if groupStr, ok := group.(string); ok {
					approverGroups = append(approverGroups, groupStr)
				}
			}
		} else if rawGroups, ok := rawApproverGroups.([]string); ok {
			approverGroups = rawGroups
		}
		if len(approverGroups) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no approver groups in session"})
			return
		}
	}

	if isAdmin || isPlatformApprover {
		// Admin/Platform Approver: expects Namespaces []string
		type AdminApproveRequest struct {
			Requests     []models.RequestData `json:"requests"`
			ApproverID   string               `json:"approverID"`
			ApproverName string               `json:"approverName"`
			Status       string               `json:"status"`
		}
		var req AdminApproveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		for _, r := range req.Requests {
			processApproval(r.ID, r, req.ApproverID, req.ApproverName, req.Status, nil, c)
		}
		c.JSON(http.StatusOK, gin.H{"message": "Admin/Platform requests processed successfully"})
		return
	} else {
		// Non-admin: expects Namespace string
		type UserApproveRequest struct {
			Requests []struct {
				ID            uint      `json:"id"`
				ApproverName  string    `json:"approverName"`
				ClusterName   string    `json:"clusterName"`
				RoleName      string    `json:"roleName"`
				Status        string    `json:"status"`
				UserID        string    `json:"userID"`
				Username      string    `json:"username"`
				Users         []string  `json:"users"`
				Justification string    `json:"justification"`
				StartDate     time.Time `json:"startDate"`
				EndDate       time.Time `json:"endDate"`
				FullyApproved bool      `gorm:"default:false"`
				Namespace     string    `json:"namespace"`
			} `json:"requests"`
			ApproverID   string `json:"approverID"`
			ApproverName string `json:"approverName"`
			Status       string `json:"status"`
		}
		var req UserApproveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		for _, r := range req.Requests {
			namespaces := []string{r.Namespace}
			// Convert to models.RequestData for downstream compatibility
			requestData := models.RequestData{
				ClusterName:   r.ClusterName,
				RoleName:      r.RoleName,
				Status:        r.Status,
				UserID:        r.UserID,
				Username:      r.Username,
				Users:         r.Users,
				Namespaces:    namespaces,
				Justification: r.Justification,
				StartDate:     r.StartDate,
				EndDate:       r.EndDate,
				FullyApproved: r.FullyApproved,
			}
			processApproval(r.ID, requestData, req.ApproverID, req.ApproverName, req.Status, approverGroups, c)
		}
		c.JSON(http.StatusOK, gin.H{"message": "User requests processed successfully"})
		return
	}
}

// Helper function to process approval logic for each request
// It updates the request status and approver information in the database
// It also creates the k8s object if all namespaces are approved
// It sends an email notification to the user if the request is approved
func processApproval(
	requestID uint,
	requestData models.RequestData,
	approverID string,
	approverName string,
	status string,
	approverGroups []string,
	c *gin.Context,
) {
	// Fetch namespaces for the request
	var dbNamespaces []models.RequestNamespace
	if err := db.DB.Where("request_id = ?", requestID).Find(&dbNamespaces).Error; err != nil {
		logger.Error("Error fetching namespaces for request", zap.Uint("requestID", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch namespaces"})
		return
	}

	// Approve all if admin (approverGroups == nil), else check group
	for i := range dbNamespaces {
		ns := &dbNamespaces[i]
		if approverGroups == nil || contains(approverGroups, ns.GroupID) {
			if status == "Approved" {
				ns.Approved = true
			} else if status == "Rejected" {
				ns.Approved = false
			}
			ns.ApproverID = approverID
			ns.ApproverName = approverName
			if err := db.DB.Save(ns).Error; err != nil {
				logger.Error("Error updating namespace approval", zap.Uint("requestID", requestID), zap.String("namespace", ns.Namespace), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update namespace approval"})
				return
			}
		} else {
			logger.Info("Skipping namespace - approver does not have permissions",
				zap.String("namespace", ns.Namespace),
				zap.String("groupID", ns.GroupID),
				zap.String("approverID", approverID),
			)
		}
	}

	// Check if all namespaces for the request are approved
	allApproved := true
	for _, ns := range dbNamespaces {
		if !ns.Approved {
			allApproved = false
			break
		}
	}

	// Only set status to "Approved" if all namespaces are approved, otherwise keep as "Requested"
	finalStatus := status
	if status == "Approved" && !allApproved {
		finalStatus = "Requested"
	}

	logger.Debug("Approval status check",
		zap.Bool("allApproved", allApproved),
		zap.String("status", status),
		zap.String("finalStatus", finalStatus),
		zap.Uint("requestID", requestID),
	)

	if allApproved && status == "Approved" {
		var namespacesToSpec []string
		for _, ns := range dbNamespaces {
			namespacesToSpec = append(namespacesToSpec, ns.Namespace)
		}
		requestData.Namespaces = namespacesToSpec
		requestData.ID = requestID
		if err := k8s.CreateK8sObject(requestData, approverName); err != nil {
			logger.Error("Error creating k8s object for request", zap.Uint("requestID", requestID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create k8s object"})
			return
		}
	}

	// Fetch the request record
	var req models.RequestData
	if err := db.DB.First(&req, requestID).Error; err != nil {
		logger.Error("Error fetching request for update", zap.Uint("requestID", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Append approver if not already present
	if !contains(req.ApproverIDs, approverID) {
		req.ApproverIDs = append(req.ApproverIDs, approverID)
	}
	if !contains(req.ApproverNames, approverName) {
		req.ApproverNames = append(req.ApproverNames, approverName)
	}

	// Update the request status and approvers using struct update
	req.Status = finalStatus
	req.FullyApproved = allApproved

	if err := db.DB.Model(&req).Select("Status", "ApproverIDs", "ApproverNames", "FullyApproved").Updates(req).Error; err != nil {
		logger.Error("Error updating request after approval", zap.Uint("requestID", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
		return
	}

	if req.Email != "" {
		body := email.BuildRequestEmail(email.EmailRequestDetails{
			Username:      req.Username,
			ClusterName:   req.ClusterName,
			Namespaces:    req.Namespaces,
			RoleName:      req.RoleName,
			Justification: req.Justification,
			StartDate:     req.StartDate,
			EndDate:       req.EndDate,
			Status:        req.Status,
			Message:       "", // Reserved for controller messages
		})
		go func() {
			if err := email.SendMail(req.Email, fmt.Sprintf("Your JIT request #%d is now %s", req.ID, req.Status), body); err != nil {
				logger.Warn("Failed to send status change email", zap.String("email", req.Email), zap.Error(err))
			}
		}()
	}
}

// GetRecords returns the latest jit requests for a user with optional limit and date range
// It fetches the records from the database and returns them as JSON
// It checks if the user is an admin or platform approver to determine the query parameters
func GetRecords(c *gin.Context) {
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}
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
		logger.Error("Error fetching records in GetRecords", zap.Error(err))
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
			logger.Error("Error fetching namespace approvals in GetRecords", zap.Uint("requestID", req.ID), zap.Error(err))
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

// GetOauthClientId checks the oauthProvider and returns the appropriate client_id, provider and redirect url
func GetOauthClientId(c *gin.Context) {
	response := map[string]interface{}{
		"client_id":    clientID,
		"provider":     oauthProvider,
		"redirect_uri": redirectUri,
		"auth_url":     azureOAuthConfig.Endpoint.AuthURL,
	}

	c.JSON(http.StatusOK, response)
}

// GetClustersAndRoles returns the list of clusters and roles available
func GetClustersAndRoles(c *gin.Context) {

	// Check if the user is logged in
	_, ok := checkLoggedIn(c)
	if !ok {
		return
	}

	// Return the clusters and roles
	response := map[string]interface{}{
		"clusters": k8s.ClusterNames,
		"roles":    k8s.AllowedRoles,
	}
	c.JSON(http.StatusOK, response)
}

// GetApprovingGroups returns the list of platform approving groups
func GetApprovingGroups(c *gin.Context) {
	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Return the platform approving groups
	c.JSON(http.StatusOK, k8s.PlatformApproverTeams)
}

// GetPendingApprovals returns the pending requests for the user based on their approver groups
// It uses the session to retrieve the user's approver groups
// It queries the database for requests with status "Requested" and matching approver groups
func GetPendingApprovals(c *gin.Context) {

	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}

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

	// Convert approverGroups to a slice of strings
	approverGroups := []string{}
	if rawGroups, ok := rawApproverGroups.([]any); ok {
		for _, group := range rawGroups {
			if groupStr, ok := group.(string); ok {
				approverGroups = append(approverGroups, groupStr)
			}
		}
	} else if rawGroups, ok := rawApproverGroups.([]string); ok {
		approverGroups = rawGroups
	}

	if len(approverGroups) == 0 {
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
		Where("request_namespaces.group_id IN (?) AND request_data.status = ? AND request_namespaces.approved = false", approverGroups, "Requested").
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

// HealthCheck used for status checking the api
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// SubmitRequest creates the new jit record in postgress
// It validates the request data and checks if the user is logged in
// It also validates the namespaces and sends an email notification
// It returns a success message or an error message
func SubmitRequest(c *gin.Context) {
	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}
	// Check if the email is present in the session data
	emailAddress, _ := sessionData["email"].(string)

	// Process the request data
	var requestData struct {
		Role          models.Roles   `json:"role"`
		ClusterName   models.Cluster `json:"cluster"`
		UserID        string         `json:"requestorId"`
		Username      string         `json:"requestorName"`
		Users         []string       `json:"users"`
		Namespaces    []string       `json:"namespaces"`
		Justification string         `json:"justification"`
		StartDate     time.Time      `json:"startDate"`
		EndDate       time.Time      `json:"endDate"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Validate namespaces and fetch group IDs and names
	namespaceGroups, err := k8s.ValidateNamespaces(requestData.ClusterName.Name, requestData.Namespaces)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Namespace validation failed: %v", err)})
		return
	}

	// Create a new RequestData in database
	dbRequestData := models.RequestData{
		ClusterName:   requestData.ClusterName.Name,
		RoleName:      requestData.Role.Name,
		Status:        "Requested",
		UserID:        requestData.UserID,
		Username:      requestData.Username,
		Users:         requestData.Users,
		Namespaces:    requestData.Namespaces,
		Justification: requestData.Justification,
		StartDate:     requestData.StartDate,
		EndDate:       requestData.EndDate,
		Email:         emailAddress,
	}

	// Insert the request data into the database
	if err := db.DB.Create(&dbRequestData).Error; err != nil {
		logger.Error("Error inserting data in SubmitRequest", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit request (database error)"})
		return
	}

	// Insert namespaces into the request_namespaces table
	for namespace, groupInfo := range namespaceGroups {
		namespaceEntry := models.RequestNamespace{
			RequestID: dbRequestData.ID,
			Namespace: namespace,
			GroupID:   groupInfo.GroupID,
			GroupName: groupInfo.GroupName,
			Approved:  false,
		}
		if err := db.DB.Create(&namespaceEntry).Error; err != nil {
			logger.Error("Error inserting namespace data in SubmitRequest", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit request (namespace error)"})
			return
		}
	}

	// Send submission email
	if dbRequestData.Email != "" {
		body := email.BuildRequestEmail(email.EmailRequestDetails{
			Username:      dbRequestData.Username,
			ClusterName:   dbRequestData.ClusterName,
			Namespaces:    dbRequestData.Namespaces,
			RoleName:      dbRequestData.RoleName,
			Justification: dbRequestData.Justification,
			StartDate:     dbRequestData.StartDate,
			EndDate:       dbRequestData.EndDate,
			Status:        "submitted",
			Message:       "",
		})
		go func() {
			if err := email.SendMail(dbRequestData.Email, fmt.Sprintf("Your JIT request #%d has been submitted", dbRequestData.ID), body); err != nil {
				logger.Warn("Failed to send submission email", zap.String("email", dbRequestData.Email), zap.Error(err))
			}
		}()
	}

	// Respond with success message
	c.JSON(http.StatusOK, gin.H{"message": "Request submitted successfully"})
}

func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

// CommonPermissions checks if the user has common permissions
// It checks if the user is logged in and retrieves their permissions
// It fetches the user's groups from the specified provider (GitHub, Google, Azure)
// It matches the user groups to the approver and admin teams
// It updates the session with the user's permissions
// It returns the permissions as JSON
func CommonPermissions(c *gin.Context) {
	session := sessions.Default(c)

	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}

	// Parse provider from payload
	var payload struct {
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid provider"})
		return
	}

	// Check if cached in session
	isApprover, isApproverOk := sessionData["isApprover"].(bool)
	isAdmin, isAdminOk := sessionData["isAdmin"].(bool)
	isPlatformApprover, isPlatformApproverOk := sessionData["isPlatformApprover"].(bool)
	approverGroups, approverGroupsOk := sessionData["approverGroups"]
	adminGroups, adminGroupsOk := sessionData["adminGroups"]
	if isApproverOk && isAdminOk && isPlatformApprover && isPlatformApproverOk && approverGroupsOk && adminGroupsOk {
		c.JSON(http.StatusOK, gin.H{
			"isApprover":         isApprover,
			"approverGroups":     approverGroups,
			"isAdmin":            isAdmin,
			"isPlatformApprover": isPlatformApprover,
			"adminGroups":        adminGroups,
		})
		return
	}

	// Get token from session
	token, _ := sessionData["token"].(string)

	var userGroups []models.Team
	var err error

	// Fetch user groups based on the provider
	switch payload.Provider {
	case "github": // GitHub provider
		userGroups, err = GetGithubTeams(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch GitHub teams"})
			return
		}
	case "google": // Google provider
		userEmail, _ := sessionData["email"].(string)
		userGroups, err = GetGoogleGroupsWithWorkloadIdentity(userEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Google groups"})
			return
		}
	case "azure": // Azure provider
		userGroups, err = GetAzureGroups(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Azure groups"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown provider"})
		return
	}

	// Match user groups to approver/admin teams
	isAdmin, isPlatformApprover, matchedAdminGroups := MatchUserGroups(
		userGroups,
		k8s.PlatformApproverTeams,
		k8s.AdminTeams,
	)

	// Check and append if user is in any JitGroup for any cluster
	var matchedApproverGroups []string
	for _, clusterName := range k8s.ClusterNames {
		jitGroups, err := k8s.GetJitGroups(clusterName)
		if err != nil {
			logger.Error("Error fetching JitGroups for cluster", zap.String("clusterName", clusterName), zap.Error(err))
			continue
		}
		groups, _, _ := unstructured.NestedSlice(jitGroups.Object, "spec", "groups")
		for _, group := range groups {
			groupMap, ok := group.(map[string]any)
			if !ok {
				continue
			}
			groupID, ok := groupMap["groupID"].(string)
			if ok {
				for _, userGroup := range userGroups {
					if userGroup.ID == groupID {
						matchedApproverGroups = append(matchedApproverGroups, groupID)
					}
				}
			}
		}
	}

	// Check if the user is an approver
	isApprover = len(matchedApproverGroups) > 0

	// Update session
	sessionData["isApprover"] = isApprover
	sessionData["approverGroups"] = matchedApproverGroups
	sessionData["isAdmin"] = isAdmin
	sessionData["isPlatformApprover"] = isPlatformApprover
	sessionData["adminGroups"] = matchedAdminGroups
	session.Set("data", sessionData)
	sessioncookie.SplitSessionData(c)

	c.JSON(http.StatusOK, gin.H{
		"isApprover":         isApprover,
		"approverGroups":     matchedApproverGroups,
		"isAdmin":            isAdmin,
		"isPlatformApprover": isPlatformApprover,
		"adminGroups":        matchedAdminGroups,
	})
}

// MatchUserGroups checks if the user belongs to any approver or admin groups
// It returns boolean flags indicating if the user is an approver or admin
// along with the matched approver and admin groups
func MatchUserGroups(
	userGroups []models.Team,
	platformTeams []models.Team,
	adminTeams []models.Team,
) (isAdmin bool, isPlatformApprover bool, matchedAdminGroups []string) {
	var matchedPlatformApproverGroups []string
	for _, group := range userGroups {
		for _, approverGroup := range platformTeams {
			if group.ID == approverGroup.ID && group.Name == approverGroup.Name {
				matchedPlatformApproverGroups = append(matchedPlatformApproverGroups, group.ID)
			}
		}
		for _, adminGroup := range adminTeams {
			if group.ID == adminGroup.ID && group.Name == adminGroup.Name {
				matchedAdminGroups = append(matchedAdminGroups, group.ID)
			}
		}
	}
	isAdmin = len(matchedAdminGroups) > 0
	isPlatformApprover = len(matchedPlatformApproverGroups) > 0
	return
}
