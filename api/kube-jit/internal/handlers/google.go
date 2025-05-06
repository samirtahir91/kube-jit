package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kube-jit/internal/models"
	"kube-jit/pkg/sessioncookie"
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
)

// getGSAEmail retrieves the Google Service Account (GSA) email from the metadata server
// It caches the email to avoid multiple requests
// It uses a sync.Once to ensure that the email is only fetched once
func getGSAEmail() (string, error) {
	gsaEmailOnce.Do(func() {
		req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email", nil)
		if err != nil {
			logger.Error("Failed to create request for GSA email", zap.Error(err))
			gsaEmailErr = err
			return
		}
		req.Header.Add("Metadata-Flavor", "Google")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Error("Failed to fetch GSA email from metadata server", zap.Error(err))
			gsaEmailErr = err
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Failed to read GSA email response body", zap.Error(err))
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

	serviceAccountEmail, err := getGSAEmail()
	if err != nil {
		logger.Error("Failed to get GSA email", zap.Error(err))
		return nil, fmt.Errorf("failed to get GSA email")
	}

	iamService, err := iamcredentials.NewService(ctx)
	if err != nil {
		logger.Error("Failed to create IAM credentials service", zap.Error(err))
		return nil, fmt.Errorf("failed to create IAM credentials service")
	}

	now := time.Now()
	claims := map[string]interface{}{
		"iss":   serviceAccountEmail,
		"sub":   adminEmail, // The email of the admin to impersonate to read a user's groups
		"aud":   "https://oauth2.googleapis.com/token",
		"scope": "https://www.googleapis.com/auth/admin.directory.group.readonly",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		logger.Error("Failed to marshal JWT claims", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal claims")
	}

	name := fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccountEmail)
	signJwtRequest := &iamcredentials.SignJwtRequest{
		Payload: string(claimsJSON),
	}

	signJwtResponse, err := iamService.Projects.ServiceAccounts.SignJwt(name, signJwtRequest).Do()
	if err != nil {
		logger.Error("Failed to sign JWT", zap.Error(err))
		return nil, fmt.Errorf("failed to sign JWT")
	}

	signedJwt := signJwtResponse.SignedJwt

	resp, err := http.Post(
		"https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(fmt.Sprintf("grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=%s", signedJwt)),
	)
	if err != nil {
		logger.Error("Failed to exchange JWT for access token", zap.Error(err))
		return nil, fmt.Errorf("failed to exchange JWT for access token")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		logger.Error("Failed to get token from Google", zap.String("response", string(body)))
		return nil, fmt.Errorf("failed to get token")
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		logger.Error("Failed to parse token response", zap.Error(err))
		return nil, fmt.Errorf("failed to parse token response")
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: tokenResp.AccessToken,
	})
	service, err := admin.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		logger.Error("Failed to create Admin SDK service", zap.Error(err))
		return nil, fmt.Errorf("failed to create Admin SDK service")
	}

	groupsCall := service.Groups.List().UserKey(userEmail)
	groupsResponse, err := groupsCall.Do()
	if err != nil {
		logger.Error("Failed to list Google groups", zap.Error(err))
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

func HandleGoogleLogin(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		logger.Warn("Missing 'code' query parameter in Google login")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code query parameter is required"})
		return
	}

	token, err := googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Error("Failed to exchange Google token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	client := googleOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		logger.Error("Failed to get Google user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Error fetching user profile from Google", zap.Int("status", resp.StatusCode))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Google"})
		return
	}

	var googleUser models.GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		logger.Error("Failed to decode Google user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user info"})
		return
	}

	normalizedUserData := models.NormalizedUserData{
		ID:        googleUser.ID,
		Name:      googleUser.Name,
		Email:     googleUser.Email,
		AvatarURL: googleUser.Picture,
		Provider:  "google",
	}

	sessionData := map[string]interface{}{
		"email": googleUser.Email,
		"token": token.AccessToken,
	}

	session := sessions.Default(c)
	session.Set("data", sessionData)

	sessioncookie.SplitSessionData(c)

	logger.Info("Session cookies set successfully for Google login", zap.String("email", googleUser.Email))

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
		return
	}

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		logger.Warn("No token in session data for Google profile")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Fetch the user's profile from Google's API
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	}))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		logger.Error("Failed to fetch Google user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Error fetching user profile from Google", zap.String("response", string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fetching user profile from Google"})
		return
	}

	var googleUser struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Email         string `json:"email"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		logger.Error("Failed to decode Google user profile", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user profile"})
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
