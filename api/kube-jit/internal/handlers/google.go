package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
	"kube-jit/pkg/utils"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
	adminEmail   string
)

func init() {
	// Set the admin email for Google Workspace
	// This email is used to impersonate the user to read their groups
	if oauthProvider == "google" {
		adminEmail = utils.MustGetEnv("GOOGLE_ADMIN_EMAIL")
	}
}

// Helper to fetch and decode Google user profile
func fetchGoogleUserProfile(token string) (*models.GoogleUser, error) {
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching user profile from Google: %s", string(body))
	}

	var googleUser models.GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode Google user info: %w", err)
	}
	return &googleUser, nil
}

// getGSAEmail retrieves the Google Service Account (GSA) email from the metadata server
func getGSAEmail(reqLogger *zap.Logger) (string, error) {

	gsaEmailOnce.Do(func() {
		req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email", nil)
		if err != nil {
			reqLogger.Error("Failed to create request for GSA email", zap.Error(err))
			gsaEmailErr = err
			return
		}
		req.Header.Add("Metadata-Flavor", "Google")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			reqLogger.Error("Failed to fetch GSA email from metadata server", zap.Error(err))
			gsaEmailErr = err
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			reqLogger.Error("Failed to read GSA email response body", zap.Error(err))
			gsaEmailErr = err
			return
		}

		gsaEmail = string(body)
	})

	return gsaEmail, gsaEmailErr
}

// GetGoogleGroupsWithWorkloadIdentity retrieves the Google Groups for a user using Workload Identity
func GetGoogleGroupsWithWorkloadIdentity(userEmail string, reqLogger *zap.Logger) ([]models.Team, error) {
	ctx := context.Background()

	serviceAccountEmail, err := getGSAEmail(reqLogger)
	if err != nil {
		reqLogger.Error("Failed to get GSA email", zap.Error(err))
		return nil, fmt.Errorf("failed to get GSA email")
	}

	iamService, err := iamcredentials.NewService(ctx)
	if err != nil {
		reqLogger.Error("Failed to create IAM credentials service", zap.Error(err))
		return nil, fmt.Errorf("failed to create IAM credentials service")
	}

	now := time.Now()
	claims := map[string]interface{}{
		"iss":   serviceAccountEmail,
		"sub":   adminEmail, // The email of the admin user to impersonate
		"aud":   "https://oauth2.googleapis.com/token",
		"scope": "https://www.googleapis.com/auth/admin.directory.group.readonly",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		reqLogger.Error("Failed to marshal JWT claims", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal claims")
	}

	name := fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccountEmail)
	signJwtRequest := &iamcredentials.SignJwtRequest{
		Payload: string(claimsJSON),
	}

	signJwtResponse, err := iamService.Projects.ServiceAccounts.SignJwt(name, signJwtRequest).Do()
	if err != nil {
		reqLogger.Error("Failed to sign JWT", zap.Error(err))
		return nil, fmt.Errorf("failed to sign JWT")
	}

	signedJwt := signJwtResponse.SignedJwt

	resp, err := http.Post(
		"https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(fmt.Sprintf("grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=%s", signedJwt)),
	)
	if err != nil {
		reqLogger.Error("Failed to exchange JWT for access token", zap.Error(err))
		return nil, fmt.Errorf("failed to exchange JWT for access token")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		reqLogger.Error("Failed to get token from Google", zap.String("response", string(body)))
		return nil, fmt.Errorf("failed to get token")
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		reqLogger.Error("Failed to parse token response", zap.Error(err))
		return nil, fmt.Errorf("failed to parse token response")
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: tokenResp.AccessToken,
	})
	service, err := admin.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		reqLogger.Error("Failed to create Admin SDK service", zap.Error(err))
		return nil, fmt.Errorf("failed to create Admin SDK service")
	}

	groupsCall := service.Groups.List().UserKey(userEmail)
	groupsResponse, err := groupsCall.Do()
	if err != nil {
		reqLogger.Error("Failed to list Google groups", zap.Error(err))
		return nil, fmt.Errorf("failed to list groups")
	}

	var teams []models.Team
	for _, group := range groupsResponse.Groups {
		teams = append(teams, models.Team{
			Name: group.Name,
			ID:   group.Email,
		})
	}

	return teams, nil
}

// HandleGoogleLogin godoc
// @Summary Google OAuth callback
// @Description Handles the Google OAuth callback, exchanges the code for an access token, fetches user info, sets session data, and returns normalized user data and expiration time.
// @Tags google
// @Accept  json
// @Produce  json
// @Param   code query string true "Google OAuth authorization code"
// @Success 200 {object} models.LoginResponse "Normalized user data and expiration time"
// @Failure 400 {object} models.SimpleMessageResponse "Missing or invalid code"
// @Failure 403 {object} models.SimpleMessageResponse "Unauthorized domain"
// @Failure 500 {object} models.SimpleMessageResponse "Internal server error"
// @Router /oauth/google/callback [get]
func HandleGoogleLogin(c *gin.Context) {
	code := c.Query("code")

	if code == "" {
		logger.Warn("Missing 'code' query parameter in Google login")
		c.JSON(http.StatusBadRequest, models.SimpleMessageResponse{Error: "Code query parameter is required"})
		return
	}

	token, err := googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Error("Failed to exchange Google token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to exchange token"})
		return
	}

	googleUser, err := fetchGoogleUserProfile(token.AccessToken)
	if err != nil {
		logger.Error("Failed to get Google user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	// Check if the user is allowed to log in
	if !isAllowedUser("google", googleUser.Email, nil) {
		logger.Warn("Login attempt from unauthorized Google domain", zap.String("email", googleUser.Email))
		c.JSON(http.StatusForbidden, models.SimpleMessageResponse{Error: "Unauthorized domain"})
		return
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        googleUser.ID,
		Name:      googleUser.Name,
		Email:     googleUser.Email,
		AvatarURL: googleUser.Picture,
		Provider:  "google",
	}

	sessionData := map[string]any{
		"id":    googleUser.ID,
		"name":  googleUser.Name,
		"email": googleUser.Email,
		"token": token.AccessToken,
	}

	session := sessions.Default(c)
	session.Set("data", sessionData)

	sessioncookie.SplitSessionData(c)

	logger.Debug("Session cookies set successfully for Google login", zap.String("email", googleUser.Email))

	c.JSON(http.StatusOK, models.LoginResponse{
		UserData:  normalizedUserData,
		ExpiresIn: int(time.Until(token.Expiry).Seconds()),
	})
}

// GetGoogleProfile godoc
// @Summary Get the logged in user's Google profile
// @Description Returns the normalized Google user profile for the authenticated user.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags google
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Success 200 {object} models.NormalizedUserData
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: no token in session data"
// @Failure 500 {object} models.SimpleMessageResponse "Internal server error"
// @Router /google/profile [get]
func GetGoogleProfile(c *gin.Context) {
	// Check if the user is logged
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		reqLogger.Warn("No token in session data for Google profile")
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: no token in session data"})
		return
	}

	googleUser, err := fetchGoogleUserProfile(token)
	if err != nil {
		reqLogger.Error("Failed to fetch Google user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: err.Error()})
		return
	}

	normalizedUserData := map[string]interface{}{
		"id":         googleUser.ID,
		"name":       googleUser.Name,
		"email":      googleUser.Email,
		"avatar_url": googleUser.Picture,
		"provider":   "google",
	}

	c.JSON(http.StatusOK, normalizedUserData)
}
