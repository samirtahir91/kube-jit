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
	_, ok := checkLoggedIn(c)
	if !ok {
		return // The response has already been sent by CheckLoggedIn
	}

	type ApproveRequest struct {
		Requests     []models.RequestData `json:"requests"`
		ApproverID   string               `json:"approverID"`
		ApproverName string               `json:"approverName"`
		Status       string               `json:"status"`
	}

	var approveReq ApproveRequest

	if err := c.ShouldBindJSON(&approveReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// If status is "Approved", create the k8s object for each request
	if approveReq.Status == "Approved" {
		for _, req := range approveReq.Requests {
			if err := k8s.CreateK8sObject(req, approveReq.ApproverName); err != nil {
				log.Printf("Error creating k8s object for request ID %d: %v", req.ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create k8s object"})
				return
			}
		}
	}

	// Extract request IDs for bulk update
	requestIDs := make([]uint, len(approveReq.Requests))
	for i, req := range approveReq.Requests {
		requestIDs[i] = req.ID
	}
	// Update the status of the requests to "Approved or Rejected"
	if err := db.DB.Model(&models.RequestData{}).Where("id IN ?", requestIDs).Updates(map[string]interface{}{
		"status":        approveReq.Status,
		"approver_id":   approveReq.ApproverID,
		"approver_name": approveReq.ApproverName,
	}).Error; err != nil {
		log.Printf("Error updating requests: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve requests (database error)"})
		return
	}

	log.Printf("Approved requests: %v", requestIDs)
	c.JSON(http.StatusOK, gin.H{"message": "Requests approved successfully"})
}

// GetRecords returns the latest jit requests for a user with optional limit and date range
// It fetches the records from the database and returns them as JSON
func GetRecords(c *gin.Context) {

	// Check if the user is logged in
	_, ok := checkLoggedIn(c)
	if !ok {
		return // The response has already been sent by CheckLoggedIn
	}

	userID := c.Query("userID")       // Get userID from query parameters
	limit := c.Query("limit")         // Get limit from query parameters
	startDate := c.Query("startDate") // Get startDate from query parameters
	endDate := c.Query("endDate")     // Get endDate from query parameters

	var requests []models.RequestData // Define requests as a slice of models.RequestData

	// Convert limit to an integer
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt <= 0 {
		limitInt = 1 // Default to 1 if limit is not provided or invalid
	}

	// Build the query with optional date range filter
	query := db.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(limitInt)
	if startDate != "" && endDate != "" {
		query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
	}

	// Fetch the latest records based on the limit and date range
	if err := query.Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, requests)
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

	// Check if isAdmin is already in the session cookie
	isAdmin, isAdminOk := sessionData["isAdmin"].(bool)
	if isAdminOk && isAdmin {
		// If the user is an admin, return all pending requests
		var pendingRequests []models.RequestData
		if err := db.DB.Where("status = ?", "Requested").Find(&pendingRequests).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"pendingRequests": pendingRequests})
		return
	}

	// Retrieve approverGroups from the session
	rawApproverGroups, ok := sessionData["approverGroups"]
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no approver groups in session"})
		return
	}

	// Convert to []string if necessary
	approverGroups := []string{}
	if rawGroups, ok := rawApproverGroups.([]interface{}); ok {
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

	// Query the database for pending requests
	var pendingRequests []models.RequestData
	if err := db.DB.Where("approving_team_id IN (?) AND status = ?", approverGroups, "Requested").Find(&pendingRequests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
		ApprovingTeam models.Team    `json:"approvingTeam"`
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

	// Create a new models.RequestData instance with extracted labels
	dbRequestData := models.RequestData{
		ApprovingTeamID:   requestData.ApprovingTeam.ID,
		ApprovingTeamName: requestData.ApprovingTeam.Name,
		ClusterName:       requestData.ClusterName.Name,
		RoleName:          requestData.Role.Name,
		Status:            "Requested",
		UserID:            requestData.UserID,
		Username:          requestData.Username,
		Users:             requestData.Users,
		Namespaces:        requestData.Namespaces,
		Justification:     requestData.Justification,
		StartDate:         requestData.StartDate,
		EndDate:           requestData.EndDate,
	}

	// Insert the request data into the database
	if err := db.DB.Create(&dbRequestData).Error; err != nil {
		log.Printf("Error inserting data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit request (database err)"})
		return
	}

	// Process the request data (e.g., save to database, perform actions)
	log.Printf("Received request: Approving Team: %s, Cluster: %s, Role: %s, Namespaces: %s, Users: %s", requestData.ApprovingTeam.Name, requestData.ClusterName, requestData.Role.Name, requestData.Namespaces, requestData.Users)

	// Respond with success message
	c.JSON(http.StatusOK, gin.H{"message": "Request submitted successfully"})
}
