package handlers

import (
	"fmt"
	"kube-jit/internal/db"
	"kube-jit/internal/middleware"
	"kube-jit/internal/models"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/utils"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	oauthProvider = os.Getenv("OAUTH_PROVIDER")
	clientID      = os.Getenv("OAUTH_CLIENT_ID")
	clientSecret  = os.Getenv("OAUTH_CLIENT_SECRET")
	redirectUri   = os.Getenv("OAUTH_REDIRECT_URI")
	httpClient    = &http.Client{
		Timeout: 60 * time.Second, // Set a global timeout for all requests
	}
)

// Logout clears all session cookies with the sessionPrefix
func Logout(c *gin.Context) {
	// Iterate through cookies with the session prefix
	for i := 0; ; i++ {
		cookieName := fmt.Sprintf("%s%d", middleware.SessionPrefix, i)
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
		log.Printf("Failed to bind JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	// Validate the signed URL
	callbackURL := c.Request.URL
	if !utils.ValidateSignedURL(callbackURL) {
		log.Println("Invalid or expired signed URL")
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	// Update the status of the request in the database
	if err := db.DB.Model(&models.RequestData{}).Where("id = ?", callbackData.TicketID).Updates(map[string]interface{}{
		"status": callbackData.Status,
		"notes":  callbackData.Message,
	}).Error; err != nil {
		log.Printf("Error updating request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request (database error)"})
		return
	}

	// Process the callback data (e.g., update the status in your system)
	log.Printf("Received callback for ticket ID %s with status %s and message %s\n", callbackData.TicketID, callbackData.Status, callbackData.Message)

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
		return // The response has already been sent by CheckLoggedIn
	}

	isAdmin, _ := sessionData["isAdmin"].(bool)

	var approverGroups []string
	if !isAdmin {
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

	if isAdmin {
		// Admin: expects Namespaces []string
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
		c.JSON(http.StatusOK, gin.H{"message": "Admin requests processed successfully"})
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
		log.Printf("Error fetching namespaces for request ID %d: %v", requestID, err)
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
				log.Printf("Error updating namespace approval: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update namespace approval"})
				return
			}
		} else {
			log.Printf("Skipping namespace %s (GroupID: %s) - approver does not have permissions", ns.Namespace, ns.GroupID)
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

	log.Printf("allApproved=%v, status=%s, finalStatus=%s, dbNamespaces=%v", allApproved, status, finalStatus, dbNamespaces)
	if allApproved && status == "Approved" {
		var namespacesToSpec []string
		for _, ns := range dbNamespaces {
			namespacesToSpec = append(namespacesToSpec, ns.Namespace)
		}
		requestData.Namespaces = namespacesToSpec
		requestData.ID = requestID
		if err := k8s.CreateK8sObject(requestData, approverName); err != nil {
			log.Printf("Error creating k8s object for request ID %d: %v", requestID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create k8s object"})
			return
		}
	}

	// Fetch the request record
	var req models.RequestData
	if err := db.DB.First(&req, requestID).Error; err != nil {
		log.Printf("Error fetching request: %v", err)
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
		log.Printf("Error updating request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
		return
	}
}

// GetRecords returns the latest jit requests for a user with optional limit and date range
// It fetches the records from the database and returns them as JSON
func GetRecords(c *gin.Context) {
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}
	isAdmin, _ := sessionData["isAdmin"].(bool)
	userID := c.Query("userID")
	username := c.Query("username")
	limit := c.Query("limit")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt <= 0 {
		limitInt = 1
	}

	// Fetch requests as before
	var requests []models.RequestData
	query := db.DB.Order("created_at desc").Limit(limitInt)
	if isAdmin {
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if username != "" {
			query = query.Where("username = ?", username)
		}
	} else {
		// Show requests where user is requestor OR approver
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type NamespaceApprovalInfo struct {
		Namespace    string `json:"namespace"`
		GroupID      string `json:"groupID"`
		Approved     bool   `json:"approved"`
		ApproverID   string `json:"approverID"`
		ApproverName string `json:"approverName"`
	}
	type RequestWithNamespaceApprovers struct {
		models.RequestData
		NamespaceApprovals []NamespaceApprovalInfo `json:"namespaceApprovals"`
	}

	// For each request, fetch its namespace approvals
	var enriched []RequestWithNamespaceApprovers
	for _, req := range requests {
		var nsApprovals []NamespaceApprovalInfo
		if err := db.DB.
			Table("request_namespaces").
			Select("namespace, group_id, approved, approver_id, approver_name").
			Where("request_id = ?", req.ID).
			Scan(&nsApprovals).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		return // The response has already been sent by CheckLoggedIn
	}

	// Return the clusters and roles
	response := map[string]interface{}{
		"clusters": k8s.ClusterNames,
		"roles":    k8s.AllowedRoles,
	}
	c.JSON(http.StatusOK, response)
}

// GetApprovingGroups returns the list of approving groups
func GetApprovingGroups(c *gin.Context) {
	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return // The response has already been sent by CheckLoggedIn
	}

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Return the approving groups (mocked here as k8s.ApproverTeams)
	c.JSON(http.StatusOK, k8s.ApproverTeams)
}

// GetPendingApprovals returns the pending requests for the user based on their approver groups
// It uses the session to retrieve the user's approver groups
// It queries the database for requests with status "Requested" and matching approver groups
func GetPendingApprovals(c *gin.Context) {

	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return // The response has already been sent by CheckLoggedIn
	}

	// Check if the user is an admin
	isAdmin, isAdminOk := sessionData["isAdmin"].(bool)
	if isAdminOk && isAdmin {
		// If the user is an admin, return all pending requests
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
				ApprovedList:  []bool{},
			}
			req = grouped[row.ID]
		}
		req.Namespaces = append(req.Namespaces, row.Namespace)
		req.GroupIDs = append(req.GroupIDs, row.GroupID)
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
func SubmitRequest(c *gin.Context) {
	// Check if the user is logged in
	_, ok := checkLoggedIn(c)
	if !ok {
		return // The response has already been sent by CheckLoggedIn
	}

	// Process the request as before
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

	// Validate namespaces and fetch group IDs
	namespaceGroups, err := k8s.ValidateNamespaces(requestData.ClusterName.Name, requestData.Namespaces)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Namespace validation failed: %v", err)})
		return
	}

	// Create a new models.RequestData instance
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
	}

	// Insert the request data into the database
	if err := db.DB.Create(&dbRequestData).Error; err != nil {
		log.Printf("Error inserting data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit request (database error)"})
		return
	}

	// Insert namespaces into the request_namespaces table
	for namespace, groupID := range namespaceGroups {
		namespaceEntry := models.RequestNamespace{
			RequestID: dbRequestData.ID,
			Namespace: namespace,
			GroupID:   groupID,
			Approved:  false,
		}
		if err := db.DB.Create(&namespaceEntry).Error; err != nil {
			log.Printf("Error inserting namespace data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit request (namespace error)"})
			return
		}
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
	approverGroups, approverGroupsOk := sessionData["approverGroups"]
	adminGroups, adminGroupsOk := sessionData["adminGroups"]
	if isApproverOk && isAdminOk && approverGroupsOk && adminGroupsOk {
		c.JSON(http.StatusOK, gin.H{
			"isApprover":     isApprover,
			"approverGroups": approverGroups,
			"isAdmin":        isAdmin,
			"adminGroups":    adminGroups,
		})
		return
	}

	// Get token from session
	token, _ := sessionData["token"].(string)

	var userGroups []models.Team
	var err error

	switch payload.Provider {
	case "github":
		userGroups, err = GetGithubTeams(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch GitHub teams"})
			return
		}
	case "google":
		userEmail, _ := sessionData["email"].(string)
		userGroups, err = GetGoogleGroupsWithWorkloadIdentity(userEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Google groups"})
			return
		}
	case "azure":
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
	isAdmin, matchedApproverGroups, matchedAdminGroups := MatchUserGroups(
		userGroups,
		k8s.ApproverTeams,
		k8s.AdminTeams,
	)

	// Check and append if user is in any JitGroup for any cluster
	for _, clusterName := range k8s.ClusterNames {
		jitGroups, err := k8s.GetJitGroups(clusterName)
		if err != nil {
			log.Printf("Error fetching JitGroups for cluster %s: %v", clusterName, err)
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
	sessionData["adminGroups"] = matchedAdminGroups
	session.Set("data", sessionData)
	middleware.SplitSessionData(c)

	c.JSON(http.StatusOK, gin.H{
		"isApprover":     isApprover,
		"approverGroups": matchedApproverGroups,
		"isAdmin":        isAdmin,
		"adminGroups":    matchedAdminGroups,
	})
}

// MatchUserGroups checks if the user belongs to any approver or admin groups
// It returns boolean flags indicating if the user is an approver or admin
// along with the matched approver and admin groups
func MatchUserGroups(
	userGroups []models.Team,
	approverTeams []models.Team,
	adminTeams []models.Team,
) (isAdmin bool, matchedApproverGroups []string, matchedAdminGroups []string) {
	for _, group := range userGroups {
		for _, approverGroup := range approverTeams {
			if group.ID == approverGroup.ID && group.Name == approverGroup.Name {
				matchedApproverGroups = append(matchedApproverGroups, group.ID)
			}
		}
		for _, adminGroup := range adminTeams {
			if group.ID == adminGroup.ID && group.Name == adminGroup.Name {
				matchedAdminGroups = append(matchedAdminGroups, group.ID)
			}
		}
	}
	isAdmin = len(matchedAdminGroups) > 0
	return
}
