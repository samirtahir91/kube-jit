package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/middleware"
	"kube-jit/internal/models"
	"kube-jit/pkg/k8s"
	"log"
	"net/http"
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
	googleOAuthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUri,
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

// IsGoogleApprover uses the Google Admin SDK to fetch the user's groups
// It returns true if the user is an approver, false otherwise
func IsGoogleApprover(c *gin.Context) {
	session := sessions.Default(c)

	// Check if the user is logged in
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return // The response has already been sent by CheckLoggedIn
	}

	// Check if isApprover and approverGroups are already in the session cookie
	isApprover, isApproverOk := sessionData["isApprover"].(bool)
	approverGroups, groupsOk := sessionData["approverGroups"]
	if isApproverOk && groupsOk {
		// Return cached values
		c.JSON(http.StatusOK, gin.H{"isApprover": isApprover, "approverGroups": approverGroups})
		return
	}

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Retrieve the user's email from the session
	userEmail := sessionData["email"]
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

	// Update the session data with isApprover and approverGroups
	sessionData["isApprover"] = isApprover
	sessionData["approverGroups"] = matchedGroups
	session.Set("data", sessionData)

	// Split the session data into cookies
	middleware.SplitSessionData(c)

	c.JSON(http.StatusOK, gin.H{"isApprover": isApprover})
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

	// Prepare session data
	sessionData := map[string]interface{}{
		"email": googleUser.Email,
		"token": token.AccessToken,
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData) // Store as a map, not a JSON string

	// Split the session data into cookies
	middleware.SplitSessionData(c)

	log.Println("Session cookies set successfully")

	// Respond with the normalized user data
	c.JSON(http.StatusOK, gin.H{
		"userData":  normalizedUserData,
		"expiresIn": int(time.Until(token.Expiry).Seconds()),
	})
}

// GetGoogleProfile gets the logged in user's profile info from Google
func GetGoogleProfile(c *gin.Context) {
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

	// Fetch the user's profile from Google's API
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
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
