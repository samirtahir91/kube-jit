package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/utils"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

var (
	ghAppClientID     = os.Getenv("GH_APP_CLIENT_ID")
	ghAppClientSecret = os.Getenv("GH_APP_CLIENT_SECRET")
	ghAppInstallId, _ = strconv.Atoi(os.Getenv("GH_APP_INSTALL_ID"))
	ghAppId, _        = strconv.Atoi(os.Getenv("GH_APP_ID"))
	ghAppPrivateKey   = os.Getenv("GH_APP_PK")
	ghOrg             = os.Getenv("GH_ORG")
	httpClient        = &http.Client{
		Timeout: 60 * time.Second, // Set a global timeout for all requests
	}
)

// K8sCallback is used by downstream controller to callback for status update
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
func ApproveOrRejectRequests(c *gin.Context) {
	type ApproveRequest struct {
		Requests     []models.RequestData `json:"requests"`
		ApproverID   int                  `json:"approverID"`
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
func GetRecords(c *gin.Context) {
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

	c.JSON(http.StatusOK, requests) // Return the list of requests
}

// GetGithubClientId returns the github app client_id
func GetGithubClientId(c *gin.Context) {
	response := map[string]interface{}{
		"client_id": ghAppClientID,
	}
	c.JSON(http.StatusOK, response)
}

// GetClustersAndRoles gets clusters and roles
func GetClustersAndRoles(c *gin.Context) {
	response := map[string]interface{}{
		"clusters": k8s.ClusterNames,
		"roles":    k8s.AllowedRoles,
	}
	c.JSON(http.StatusOK, response)
}

// GetApprovingGroups returns the list of approving groups
func GetApprovingGroups(c *gin.Context) {
	c.JSON(http.StatusOK, k8s.ApproverTeams)
}

// GenerateJWT creates a JWT for the github app
func GenerateJWT() (string, error) {
	privateKeyData, err := os.ReadFile(ghAppPrivateKey)
	if err != nil {
		return "", err
	}

	// Parse private key
	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
	if err != nil {
		return "", fmt.Errorf("failed to decode PEM block containing private key")
	}

	// Generate JWT
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    fmt.Sprintf("%d", ghAppId),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)), // Expiry time is 10 minutes from now
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(parsedKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GenerateInstallationAccessToken generates the installation access token
func GenerateInstallationAccessToken(jwtToken string, installationID int) (string, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to generate access token: %s", resp.Status)
	}

	var tokenResponse struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}

	return tokenResponse.Token, nil
}

// GetGithubTeams gets teams for a github org
func GetGithubTeams(c *gin.Context) {
	jwtToken, err := GenerateJWT()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	accessToken, err := GenerateInstallationAccessToken(jwtToken, ghAppInstallId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	req, err := http.NewRequest("GET", "https://api.github.com/orgs/"+ghOrg+"/teams", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Error fetching teams from GitHub"})
		return
	}

	var teams interface{}
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// GetUsersGithubTeams gets teams for a user
func GetUsersGithubTeams(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized no token in request cookie"})
		return
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user/teams", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", token.(string))

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Error fetching teams from GitHub"})
		return
	}

	var teams []models.Team
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// IsApprover checks is user belongs to an approver team and return bool
func IsApprover(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized no token in request cookie"})
		return
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user/teams", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", token.(string))

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Error fetching teams from GitHub"})
		return
	}

	var userTeams []models.Team
	if err := json.NewDecoder(resp.Body).Decode(&userTeams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, userTeam := range userTeams {
		for _, approverTeam := range k8s.ApproverTeams {
			if userTeam.ID == approverTeam.ID && userTeam.Name == approverTeam.Name {
				c.JSON(http.StatusOK, gin.H{"isApprover": true})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"isApprover": false})
}

// GetPendingApprovals checks matched teams for a user compared to approver teams and gets pending requests
func GetPendingApprovals(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized no token in request cookie"})
		return
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user/teams", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", token.(string))

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Error fetching teams from GitHub"})
		return
	}

	var userTeams []models.Team
	if err := json.NewDecoder(resp.Body).Decode(&userTeams); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var matchedTeams []int
	for _, userTeam := range userTeams {
		for _, approverTeam := range k8s.ApproverTeams {
			if userTeam.ID == approverTeam.ID && userTeam.Name == approverTeam.Name {
				matchedTeams = append(matchedTeams, userTeam.ID)
			}
		}
	}

	var pendingRequests []models.RequestData
	if err := db.DB.Where("approving_team_id IN (?) AND status = ?", matchedTeams, "Requested").Find(&pendingRequests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pendingRequests": pendingRequests})
}

// OauthRedirect used for github app oauhth flow
func OauthRedirect(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code query parameter is required"})
		return
	}

	ctx := context.Background()
	data := url.Values{
		"client_id":     {ghAppClientID},
		"client_secret": {ghAppClientSecret},
		"code":          {code},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("Response body: %s", body)

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching access token from GitHub"})
		return
	}

	var tokenData models.GitHubTokenResponse
	if err := json.Unmarshal(body, &tokenData); err != nil {
		log.Printf("Error decoding response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	req, err = http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", tokenData.TokenType+" "+tokenData.AccessToken)

	userResp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user data from GitHub"})
		return
	}

	var userData interface{}
	if err := json.NewDecoder(userResp.Body).Decode(&userData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	session := sessions.Default(c)
	session.Options(sessions.Options{
		MaxAge:   tokenData.ExpiresIn,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		// SameSite:
	})
	session.Set("token", tokenData.TokenType+" "+tokenData.AccessToken)
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"userData":  userData,
		"expiresIn": tokenData.ExpiresIn,
	})
}

// HealthCheck used for status checking the api
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// GetProfile gets the logged in users profile info
func GetProfile(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized no token in request cookie"})
		return
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", token.(string))

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user data from GitHub"})
		return
	}

	var userData interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, userData)
}

// SubmitRequest creates the new jit record in postgress
func SubmitRequest(c *gin.Context) {

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
