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
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"
)

var (
	oauthProvider     = os.Getenv("OAUTH_PROVIDER")
	ghAppClientID     = os.Getenv("GH_APP_CLIENT_ID")
	ghAppClientSecret = os.Getenv("GH_APP_CLIENT_SECRET")
	httpClient        = &http.Client{
		Timeout: 60 * time.Second, // Set a global timeout for all requests
	}
	googleOAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
	gsaEmail     string
	gsaEmailErr  error
	gsaEmailOnce sync.Once
)

// getGSAEmail retrieves the Google Service Account (GSA) email from the metadata server
// It caches the email to avoid multiple requests
// It uses a sync.Once to ensure that the email is only fetched once
func getGSAEmail() (string, error) {
	gsaEmailOnce.Do(func() {
		req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email", nil)
		if err != nil {
			gsaEmailErr = err
			return
		}
		req.Header.Add("Metadata-Flavor", "Google")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			gsaEmailErr = err
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			gsaEmailErr = err
			return
		}

		gsaEmail = string(body)
	})

	return gsaEmail, gsaEmailErr
}

// GetGoogleGroupsWithWorkloadIdentity retrieves the Google Groups for a user using Workload Identity
// It uses the Google Service Account (GSA) to impersonate the user and fetch their groups
// It returns a list of teams (groups) associated with the user
// It uses the Google Admin SDK to list the groups
func GetGoogleGroupsWithWorkloadIdentity(userEmail string) ([]models.Team, error) {
	ctx := context.Background()

	// Get GSA email
	serviceAccountEmail, err := getGSAEmail()
	if err != nil {
		return nil, fmt.Errorf("failed to get GSA email: %v", err)
	}

	// Initialize IAMCredentials Service (to sign JWTs)
	iamService, err := iamcredentials.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM credentials service: %v", err)
	}

	// JWT Claims
	now := time.Now()
	claims := map[string]interface{}{
		"iss":   serviceAccountEmail,
		"sub":   userEmail, // IMPERSONATE USER HERE
		"aud":   "https://oauth2.googleapis.com/token",
		"scope": "https://www.googleapis.com/auth/admin.directory.group.readonly",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}

	// Marshal claims into JSON
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal claims: %v", err)
	}

	// Sign the JWT
	name := fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccountEmail)
	signJwtRequest := &iamcredentials.SignJwtRequest{
		Payload: string(claimsJSON),
	}

	signJwtResponse, err := iamService.Projects.ServiceAccounts.SignJwt(name, signJwtRequest).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to sign JWT: %v", err)
	}

	signedJwt := signJwtResponse.SignedJwt

	// Exchange signed JWT for OAuth2 token
	resp, err := http.Post(
		"https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(fmt.Sprintf("grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=%s", signedJwt)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange JWT for access token: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get token: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %v", err)
	}

	// Build Admin SDK Directory client
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: tokenResp.AccessToken,
	})
	service, err := admin.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create Admin SDK service: %v", err)
	}

	// List Groups
	groupsCall := service.Groups.List().UserKey(userEmail)
	groupsResponse, err := groupsCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %v", err)
	}

	// Print the groups
	var teams []models.Team
	for _, group := range groupsResponse.Groups {
		//log.Printf("Group: %s (%s)", group.Name, group.Email)
		teams = append(teams, models.Team{
			Name: group.Name,
			ID:   group.Email,
		})
	}

	return teams, nil
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
	// Retrieve the token from the session
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in request cookie"})
		return
	}

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
// It fetches the records from the database and returns them as JSON
func GetRecords(c *gin.Context) {
	// Retrieve the token from the session
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in request cookie"})
		return
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

// GetOauthClientId checks the oauthProvider and returns the appropriate client_id
func GetOauthClientId(c *gin.Context) {
	var clientId string
	var provider string
	if oauthProvider == "google" {
		clientId = googleOAuthConfig.ClientID
		provider = "google"
	} else if oauthProvider == "github" {
		clientId = ghAppClientID
		provider = "github"
	}

	response := map[string]interface{}{
		"client_id": clientId,
		"provider":  provider,
	}

	c.JSON(http.StatusOK, response)
}

// GetClustersAndRoles returns the list of clusters and roles available
func GetClustersAndRoles(c *gin.Context) {
	// Retrieve the token from the session
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in request cookie"})
		return
	}

	response := map[string]interface{}{
		"clusters": k8s.ClusterNames,
		"roles":    k8s.AllowedRoles,
	}
	c.JSON(http.StatusOK, response)
}

// GetApprovingGroups returns the list of approving groups
func GetApprovingGroups(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in request cookie"})
		return
	}
	c.JSON(http.StatusOK, k8s.ApproverTeams)
}

// IsGithubApprover uses the GitHub API to fetch the user's teams and checks if they belong to any approver teams
// It returns true if the user is an approver, false otherwise
func IsGithubApprover(c *gin.Context) {
	session := sessions.Default(c)

	// Check if isApprover and approverGroups are already in the session cookie
	isApprover, ok := session.Get("isApprover").(bool)
	_, groupsOk := session.Get("approverGroups").([]string)

	if ok && groupsOk {
		// Return cached values
		c.JSON(http.StatusOK, gin.H{"isApprover": isApprover})
		return
	}

	// Retrieve token from session
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in request cookie"})
		return
	}

	// Fetch user's teams from GitHub
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

	// Determine if the user is an approver
	var matchedGroups []string
	for _, userTeam := range userTeams {
		for _, approverTeam := range k8s.ApproverTeams {
			if userTeam.ID == approverTeam.ID && userTeam.Name == approverTeam.Name {
				matchedGroups = append(matchedGroups, userTeam.ID)
			}
		}
	}

	isApprover = len(matchedGroups) > 0

	// Cache the result in the session
	session.Set("isApprover", isApprover)
	session.Set("approverGroups", matchedGroups)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"isApprover": isApprover})
}

// IsGoogleApprover uses the Google Admin SDK to fetch the user's groups
// It returns true if the user is an approver, false otherwise
func IsGoogleApprover(c *gin.Context) {
	session := sessions.Default(c)

	// Check if isApprover and approverGroups are already in the session
	isApprover, ok := session.Get("isApprover").(bool)
	_, groupsOk := session.Get("approverGroups").([]string)

	if ok && groupsOk {
		// Return cached values
		c.JSON(http.StatusOK, gin.H{"isApprover": isApprover})
		return
	}

	// Retrieve the user's email from the session
	userEmail := session.Get("email")
	if userEmail == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no email in session"})
		return
	}

	// Fetch the user's groups using Workload Identity
	userGroups, err := GetGoogleGroupsWithWorkloadIdentity(userEmail.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Google groups"})
		return
	}

	// Check if the user belongs to any approver groups
	var matchedGroups []string
	for _, userGroup := range userGroups {
		for _, approverGroup := range k8s.ApproverTeams {
			if userGroup.ID == approverGroup.ID && userGroup.Name == approverGroup.Name {
				matchedGroups = append(matchedGroups, userGroup.ID)
			}
		}
	}

	isApprover = len(matchedGroups) > 0

	// Cache the result in the session
	session.Set("isApprover", isApprover)
	session.Set("approverGroups", matchedGroups)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"isApprover": isApprover})
}

// GetPendingApprovals returns the pending requests for the user based on their approver groups
// It uses the session to retrieve the user's approver groups
// It queries the database for requests with status "Requested" and matching approver groups
func GetPendingApprovals(c *gin.Context) {
	session := sessions.Default(c)

	// Retrieve approverGroups from the session
	approverGroups, ok := session.Get("approverGroups").([]string)
	if !ok || len(approverGroups) == 0 {
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

// HandleGitHubLogin handles the GitHub OAuth callback
func HandleGitHubLogin(c *gin.Context) {
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

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching access token from GitHub"})
		return
	}

	var tokenData models.GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
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

	var githubUser models.GitHubUser
	if err := json.NewDecoder(userResp.Body).Decode(&githubUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        strconv.Itoa(githubUser.ID),
		Name:      githubUser.Login,
		Email:     "", // GitHub API may not return email by default
		AvatarURL: githubUser.AvatarURL,
		Provider:  "github",
	}

	session := sessions.Default(c)
	session.Options(sessions.Options{
		MaxAge:   tokenData.ExpiresIn,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	})
	session.Set("token", tokenData.TokenType+" "+tokenData.AccessToken)
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"userData":  normalizedUserData,
		"expiresIn": tokenData.ExpiresIn,
	})
}

func HandleGoogleLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code query parameter is required"})
		return
	}

	// Exchange the authorization code for a token
	token, err := googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	// Use the token to fetch user info
	client := googleOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Google"})
		return
	}

	// Decode the user info
	var googleUser models.GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user info"})
		return
	}

	// Normalize the user data
	normalizedUserData := models.NormalizedUserData{
		ID:        googleUser.ID,
		Name:      googleUser.Name,
		Email:     googleUser.Email,
		AvatarURL: googleUser.Picture,
		Provider:  "google",
	}

	// Save the email and token in the session
	session := sessions.Default(c)
	session.Options(sessions.Options{
		MaxAge:   int(time.Until(token.Expiry).Seconds()),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	})
	session.Set("email", googleUser.Email) // Store the email in the session
	session.Set("token", token.AccessToken)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	// Respond with the normalized user data
	c.JSON(http.StatusOK, gin.H{
		"userData":  normalizedUserData,
		"expiresIn": int(time.Until(token.Expiry).Seconds()),
	})
}

// HealthCheck used for status checking the api
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// GetGithubProfile gets the logged in users profile info
func GetGithubProfile(c *gin.Context) {
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

// GetGoogleProfile gets the logged in user's profile info from Google
func GetGoogleProfile(c *gin.Context) {
	// Retrieve the token from the session
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in request cookie"})
		return
	}

	// Use the token to fetch the user's profile from Google's API
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token.(string),
	}))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Google"})
		return
	}

	// Decode the response into a struct
	var googleUser struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Email         string `json:"email"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user profile"})
		return
	}

	// Normalize the response to match the GitHub profile structure
	normalizedUserData := map[string]interface{}{
		"id":         googleUser.ID,
		"name":       googleUser.Name,
		"email":      googleUser.Email,
		"avatar_url": googleUser.Picture,
		"provider":   "google",
	}

	// Return the normalized user data
	c.JSON(http.StatusOK, normalizedUserData)
}

// SubmitRequest creates the new jit record in postgress
func SubmitRequest(c *gin.Context) {

	// Retrieve the token from the session
	session := sessions.Default(c)
	token := session.Get("token")
	if token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in request cookie"})
		return
	}

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
