package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Helper to fetch and decode GitHub user profile
func fetchGitHubUserProfile(token string) (*models.GitHubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for GitHub user: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching user data from GitHub: %s", string(body))
	}

	var githubUser models.GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub user: %w", err)
	}
	return &githubUser, nil
}

// Helper to fetch primary/verified email if not present in user profile
func fetchGitHubPrimaryEmail(token string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for GitHub emails: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch GitHub emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error fetching emails from GitHub: %s", string(body))
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("failed to decode GitHub emails: %w", err)
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified email found")
}

// Fetch GitHub teams for a user using their OAuth token
// This function is used to get the teams associated with the authenticated user
// It sends a GET request to the GitHub API endpoint for user teams
// and returns a slice of models.Team
// Each team is represented by its ID and name
// It returns an error if the request fails or if the response is not as expected
func GetGithubTeams(token string, reqLogger *zap.Logger) ([]models.Team, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/teams", nil)
	if err != nil {
		reqLogger.Error("Failed to create request for GitHub teams", zap.Error(err))
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		reqLogger.Error("Failed to fetch GitHub teams", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		reqLogger.Warn("Error fetching teams from GitHub", zap.String("response", string(body)))
		return nil, fmt.Errorf("error fetching teams from GitHub: %s", string(body))
	}

	var githubTeams []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&githubTeams); err != nil {
		reqLogger.Error("Failed to decode GitHub teams", zap.Error(err))
		return nil, err
	}

	var teams []models.Team
	for _, t := range githubTeams {
		teams = append(teams, models.Team{
			ID:   strconv.Itoa(t.ID),
			Name: t.Name,
		})
	}
	return teams, nil
}

// HandleGitHubLogin godoc
// @Summary GitHub OAuth callback
// @Description Handles the GitHub OAuth callback, exchanges the code for an access token, fetches user info, sets session data, and returns normalized user data and expiration time.
// @Tags github
// @Accept  json
// @Produce  json
// @Param   code query string true "GitHub OAuth authorization code"
// @Success 200 {object} models.LoginResponse "Normalized user data and expiration time"
// @Failure 400 {object} models.SimpleMessageResponse "Missing or invalid code"
// @Failure 403 {object} models.SimpleMessageResponse "Unauthorized org"
// @Failure 500 {object} models.SimpleMessageResponse "Internal server error"
// @Router /oauth/github/callback [get]
func HandleGitHubLogin(c *gin.Context) {
	// Check for the presence of the 'code' query parameter
	code := c.Query("code")
	if code == "" {
		logger.Warn("Missing 'code' query parameter in GitHub login")
		c.JSON(http.StatusBadRequest, models.SimpleMessageResponse{Error: "Code query parameter is required"})
		return
	}

	// Get the client ID and secret from global variables
	ctx := context.Background()
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}
	// Send a POST request to GitHub to exchange the code for an access token
	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		logger.Error("Failed to create request for GitHub access token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to create request for access token"})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to fetch GitHub access token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch access token"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Error fetching access token from GitHub", zap.Int("status", resp.StatusCode))
		c.JSON(http.StatusBadRequest, models.SimpleMessageResponse{Error: "Error fetching access token from GitHub"})
		return
	}

	// Decode the response body to get the access token
	var tokenData models.GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		logger.Error("Failed to decode GitHub token response", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to decode access token"})
		return
	}

	// Fetch user profile
	githubUser, err := fetchGitHubUserProfile(tokenData.AccessToken)
	if err != nil {
		logger.Error("Failed to get GitHub user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	// Fetch email if not present in profile
	email := githubUser.Email
	if email == "" {
		email, _ = fetchGitHubPrimaryEmail(tokenData.AccessToken)
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        strconv.Itoa(githubUser.ID),
		Name:      githubUser.Login,
		Email:     email,
		AvatarURL: githubUser.AvatarURL,
		Provider:  "github",
	}

	sessionData := map[string]interface{}{
		"token": tokenData.AccessToken,
		"email": email,
		"id":    normalizedUserData.ID,
		"name":  normalizedUserData.Name,
	}

	// Save the session data in the session
	session := sessions.Default(c)
	session.Set("data", sessionData)
	sessioncookie.SplitSessionData(c)

	logger.Debug("Session cookies set successfully for GitHub login", zap.String("user", githubUser.Login))

	// Fetch orgs for the user
	orgReq, err := http.NewRequest("GET", "https://api.github.com/user/orgs", nil)
	if err != nil {
		logger.Error("Failed to create request for GitHub orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to create request for orgs"})
		return
	}
	orgReq.Header.Set("Authorization", tokenData.TokenType+" "+tokenData.AccessToken)
	orgResp, err := httpClient.Do(orgReq)
	if err != nil {
		logger.Error("Failed to fetch GitHub orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch orgs"})
		return
	}
	defer orgResp.Body.Close()

	var orgs []struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(orgResp.Body).Decode(&orgs); err != nil {
		logger.Error("Failed to decode GitHub orgs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to decode orgs"})
		return
	}
	orgNames := []string{}
	for _, org := range orgs {
		orgNames = append(orgNames, org.Login)
	}
	extraInfo := map[string]any{"orgs": orgNames}

	if !isAllowedUser("github", email, extraInfo) {
		logger.Warn("Login attempt from unauthorized GitHub org", zap.String("email", email))
		c.JSON(http.StatusForbidden, models.SimpleMessageResponse{Error: "Unauthorized org"})
		return
	}

	// return the normalized user data and expiration time
	c.JSON(http.StatusOK, models.LoginResponse{
		UserData:  normalizedUserData,
		ExpiresIn: tokenData.ExpiresIn,
	})
}

// GetGithubProfile godoc
// @Summary Get the logged in user's GitHub profile
// @Description Returns the normalized GitHub user profile for the authenticated user.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags github
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Success 200 {object} models.NormalizedUserData
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: no token in session data"
// @Failure 500 {object} models.SimpleMessageResponse "Internal server error"
// @Router /github/profile [get]
func GetGithubProfile(c *gin.Context) {
	// Check if the user is logged in
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		reqLogger.Warn("No token in session data for GitHub profile")
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: no token in session data"})
		return
	}

	githubUser, err := fetchGitHubUserProfile(token)
	if err != nil {
		reqLogger.Error("Failed to fetch GitHub user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	email := githubUser.Email
	if email == "" {
		email, _ = fetchGitHubPrimaryEmail(token)
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        strconv.Itoa(githubUser.ID),
		Name:      githubUser.Login,
		Email:     email,
		AvatarURL: githubUser.AvatarURL,
		Provider:  "github",
	}

	c.JSON(http.StatusOK, normalizedUserData)
}
