package handlers

import (
	"fmt"
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"kube-jit/pkg/email"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/utils"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	// Get OAuth values environment variables
	oauthProvider = utils.MustGetEnv("OAUTH_PROVIDER")
	clientID      = utils.MustGetEnv("OAUTH_CLIENT_ID")
	clientSecret  = utils.MustGetEnv("OAUTH_CLIENT_SECRET")
	redirectUri   = utils.MustGetEnv("OAUTH_REDIRECT_URI")
	allowedDomain string
	allowedOrg    string
	httpClient    = &http.Client{
		Timeout: 60 * time.Second,
	}
)

// OauthClientIdResponse represents the response for GetOauthClientId
type OauthClientIdResponse struct {
	ClientID    string `json:"client_id"`
	Provider    string `json:"provider"`
	RedirectURI string `json:"redirect_uri"`
	AuthURL     string `json:"auth_url"`
}

// BuildShaResponse represents the response for GetBuildSha
type BuildShaResponse struct {
	Sha string `json:"sha"`
}

func init() {
	// Set the admin email for Google OAuth provider
	if oauthProvider == "google" {
		allowedDomain = utils.MustGetEnv("ALLOWED_DOMAIN")
	} else if oauthProvider == "github" {
		allowedOrg = utils.MustGetEnv("ALLOWED_GITHUB_ORG")
	} else if oauthProvider == "azure" {
		allowedDomain = utils.MustGetEnv("ALLOWED_DOMAIN")
	}
}

// ClustersAndRolesResponse represents the response for clusters and roles
type ClustersAndRolesResponse struct {
	Clusters []string       `json:"clusters"`
	Roles    []models.Roles `json:"roles"`
}

// K8sCallback godoc
// @Summary Kubernetes controller callback for status update
// @Description Used by the downstream Kubernetes controller to callback for status update. Validates the signed URL and updates the request status in the database. Returns a success message.
// @Tags k8s
// @Accept  json
// @Produce  json
// @Param   request body object true "Callback payload (ticketID, status, message)"
// @Success 200 {object} models.SimpleMessageResponse "Status updated successfully"
// @Failure 400 {object} models.SimpleMessageResponse "Invalid request"
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized"
// @Failure 500 {object} models.SimpleMessageResponse "Failed to update request"
// @Router /k8s-callback [post]
func K8sCallback(c *gin.Context) {
	var callbackData struct {
		TicketID string `json:"ticketID"`
		Status   string `json:"status"`
		Message  string `json:"message"`
	}

	if err := c.ShouldBindJSON(&callbackData); err != nil {
		logger.Warn("Failed to bind JSON in K8sCallback", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.SimpleMessageResponse{Error: "Invalid request"})
		return
	}

	callbackURL := c.Request.URL
	if !utils.ValidateSignedURL(callbackURL, k8s.CallbackHostOverride) {
		logger.Warn("Invalid or expired signed URL in K8sCallback")
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized"})
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
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to update request (database error)"})
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

	c.JSON(http.StatusOK, models.SimpleMessageResponse{Error: "Success"})
}

// GetOauthClientId godoc
// @Summary Get OAuth client configuration
// @Description Returns the OAuth client_id, provider, redirect URI, and auth URL for the frontend to initiate login.
// @Tags auth
// @Accept  json
// @Produce  json
// @Success 200 {object} handlers.OauthClientIdResponse "OAuth client configuration"
// @Router /client_id [get]
func GetOauthClientId(c *gin.Context) {
	response := OauthClientIdResponse{
		ClientID:    clientID,
		Provider:    oauthProvider,
		RedirectURI: redirectUri,
		AuthURL:     azureOAuthConfig.Endpoint.AuthURL,
	}

	c.JSON(http.StatusOK, response)
}

// GetClustersAndRoles godoc
// @Summary Get available clusters and roles
// @Description Returns the list of clusters and roles available to the user.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags records
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Success 200 {object} ClustersAndRolesResponse "clusters and roles"
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: no token in session data"
// @Router /clusters-and-roles [get]
func GetClustersAndRoles(c *gin.Context) {
	response := ClustersAndRolesResponse{
		Clusters: k8s.ClusterNames,
		Roles:    k8s.AllowedRoles,
	}
	c.JSON(http.StatusOK, response)
}

// GetBuildSha godoc
// @Summary Get build SHA
// @Description Returns the current build SHA for the running API.
// @Tags health
// @Accept  json
// @Produce  json
// @Success 200 {object} handlers.BuildShaResponse "Current build SHA"
// @Router /build-sha [get]
func GetBuildSha(c *gin.Context) {
	c.JSON(http.StatusOK, BuildShaResponse{Sha: os.Getenv("BUILD_SHA")})
}

// GetApprovingGroups godoc
// @Summary Get platform approving groups
// @Description Returns the list of platform approving groups for the authenticated user.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags records
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Success 200 {array} models.Team
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: no token in session data"
// @Router /approving-groups [get]
func GetApprovingGroups(c *gin.Context) {
	// Check if the user is logged in
	sessionData := GetSessionData(c)

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: no token in session data"})
		return
	}

	// Return the platform approving groups
	c.JSON(http.StatusOK, k8s.PlatformApproverTeams)
}

// HealthCheck godoc
// @Summary Health check endpoint
// @Description Returns a simple status message to verify the API is running.
// @Tags health
// @Accept  json
// @Produce  json
// @Success 200 {object} models.SimpleMessageResponse "API is healthy"
// @Router /healthz [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.SimpleMessageResponse{Status: "healthy"})
}

// contains checks if a string is present in a slice of strings
func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

// MatchUserGroups checks if the user belongs to any approver or admin groups
// It returns boolean flags indicating if the user is an approver or admin
// along with the matched approver and admin groups
func MatchUserGroups(
	userGroups []models.Team,
	platformTeams []models.Team,
	adminTeams []models.Team,
) (isAdmin bool, isPlatformApprover bool, matchedPlatformGroups, matchedAdminGroups []models.Team) {
	for _, group := range userGroups {
		for _, approverGroup := range platformTeams {
			if group.ID == approverGroup.ID && group.Name == approverGroup.Name {
				matchedPlatformGroups = append(matchedPlatformGroups, group)
			}
		}
		for _, adminGroup := range adminTeams {
			if group.ID == adminGroup.ID && group.Name == adminGroup.Name {
				matchedAdminGroups = append(matchedAdminGroups, group)
			}
		}
	}
	isAdmin = len(matchedAdminGroups) > 0
	isPlatformApprover = len(matchedPlatformGroups) > 0
	return
}

func RequestLogger(c *gin.Context) *zap.Logger {
	sessionData := GetSessionData(c)
	userID, _ := sessionData["id"].(string)
	username, _ := sessionData["name"].(string)
	return logger.With(
		zap.String("userID", userID),
		zap.String("username", username),
	)
}
