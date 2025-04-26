package routes

import (
	"kube-jit/internal/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	r.GET("/kube-jit-api/approving-groups", handlers.GetApprovingGroups)
	r.GET("/kube-jit-api/roles-and-clusters", handlers.GetClustersAndRoles)
	r.GET("/kube-jit-api/github/teams", handlers.GetUsersGithubTeams)
	r.GET("/kube-jit-api/github/org/teams", handlers.GetGithubTeams)
	r.GET("/kube-jit-api/oauth/redirect", handlers.OauthRedirect)
	r.GET("/kube-jit-api/profile", handlers.GetProfile)
	r.POST("/kube-jit-api/submit-request", handlers.SubmitRequest)
	r.GET("/kube-jit-api/history", handlers.GetRecords)
	r.GET("/kube-jit-api/approvals", handlers.GetPendingApprovals)
	r.POST("/kube-jit-api/approve-reject", handlers.ApproveOrRejectRequests)
	r.GET("/kube-jit-api/is-approver", handlers.IsApprover)
	r.POST("/kube-jit-api/k8s-callback", handlers.K8sCallback)
	r.GET("/kube-jit-api/healthz", handlers.HealthCheck)
	r.GET("/kube-jit-api/github/client_id", handlers.GetGithubClientId)
}
