package handlers

import (
	"fmt"
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"kube-jit/pkg/email"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/utils"
	"net/http"
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
	if !utils.ValidateSignedURL(callbackURL, k8s.CallbackHostOverride) {
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
	// Return the clusters and roles
	response := map[string]interface{}{
		"clusters": k8s.ClusterNames,
		"roles":    k8s.AllowedRoles,
	}
	c.JSON(http.StatusOK, response)
}

// GetApprovingGroups returns the list of platform approving groups
func GetApprovingGroups(c *gin.Context) {
	// Check if the user is logged in and get logger
	sessionData, _ := GetSessionData(c)

	// Retrieve the token from the session data
	token, ok := sessionData["token"].(string)
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no token in session data"})
		return
	}

	// Return the platform approving groups
	c.JSON(http.StatusOK, k8s.PlatformApproverTeams)
}

// HealthCheck used for status checking the api
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
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
