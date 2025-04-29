package routes

import (
	"kube-jit/internal/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	r.GET("/kube-jit-api/approving-groups", handlers.GetApprovingGroups)
	r.GET("/kube-jit-api/roles-and-clusters", handlers.GetClustersAndRoles)
	r.GET("/kube-jit-api/oauth/github/callback", handlers.HandleGitHubLogin)
	r.GET("/kube-jit-api/oauth/google/callback", handlers.HandleGoogleLogin)
	r.GET("/kube-jit-api/oauth/azure/callback", handlers.HandleAzureLogin)
	r.GET("/kube-jit-api/github/profile", handlers.GetGithubProfile)
	r.GET("/kube-jit-api/google/profile", handlers.GetGoogleProfile)
	r.POST("/kube-jit-api/submit-request", handlers.SubmitRequest)
	r.GET("/kube-jit-api/history", handlers.GetRecords)
	r.GET("/kube-jit-api/approvals", handlers.GetPendingApprovals)
	r.POST("/kube-jit-api/approve-reject", handlers.ApproveOrRejectRequests)
	r.GET("/kube-jit-api/github/is-approver", handlers.IsGithubApprover)
	r.GET("/kube-jit-api/google/is-approver", handlers.IsGoogleApprover)
	r.POST("/kube-jit-api/k8s-callback", handlers.K8sCallback)
	r.GET("/kube-jit-api/healthz", handlers.HealthCheck)
	r.GET("/kube-jit-api/client_id", handlers.GetOauthClientId)
}
